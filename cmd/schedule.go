package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/scheduler"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Run enabled artist, label, and dedupe schedules",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := getConfig(cmd)
		if err != nil {
			return err
		}
		return runWithLoadedConfigLogging(cfg, func(cfg *config.Config) error {
			return runSchedule(cmd.Context(), cfg)
		})
	},
}

func init() {
	rootCmd.AddCommand(scheduleCmd)
}

type parsedJob struct {
	job      scheduler.Job
	schedule cron.Schedule
}

func runSchedule(parent context.Context, cfg *config.Config) error {
	jobs, err := configuredJobs(cfg)
	if err != nil {
		return err
	}
	parsed := make([]parsedJob, 0, len(jobs))
	for _, job := range jobs {
		schedule, err := cron.ParseStandard(job.Schedule.Cron)
		if err != nil {
			return fmt.Errorf("invalid cron expression for %s: %w", job.Name, err)
		}
		parsed = append(parsed, parsedJob{job: job, schedule: schedule})
	}

	ctx, stopSignals := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	runner := scheduler.NewRunner(log.Default())
	runnerDone := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(runnerDone)
	}()

	cronRunner := cron.New()
	for _, entry := range parsed {
		entry := entry
		cronRunner.Schedule(entry.schedule, cron.FuncJob(func() {
			if !runner.Enqueue(entry.job) {
				log.Printf("Scheduled job %s is already pending; coalescing trigger", entry.job.Name)
			}
		}))
	}
	cronRunner.Start()

	for _, entry := range parsed {
		if entry.job.Schedule.RunOnStart && !runner.Enqueue(entry.job) {
			log.Printf("Scheduled job %s was already pending at startup", entry.job.Name)
		}
	}

	log.Printf("Scheduler started with %d enabled job(s)", len(parsed))
	<-ctx.Done()
	log.Println("Scheduler stopping")
	cronStopped := cronRunner.Stop()
	<-cronStopped.Done()
	<-runnerDone
	return nil
}

func configuredJobs(cfg *config.Config) ([]scheduler.Job, error) {
	var jobs []scheduler.Job
	if cfg.Monitor.Artists.Schedule.Enabled {
		jobs = append(jobs, scheduler.Job{
			Name:     "monitor-artists",
			Schedule: cfg.Monitor.Artists.Schedule,
			Run:      func() error { return runArtistJob(cfg, nil) },
		})
	}
	if cfg.Monitor.Labels.Schedule.Enabled {
		if _, err := labelIDsForRun(nil, cfg.Monitor.Labels.IDs); err != nil {
			return nil, fmt.Errorf("scheduled label monitoring: %w", err)
		}
		jobs = append(jobs, scheduler.Job{
			Name:     "monitor-labels",
			Schedule: cfg.Monitor.Labels.Schedule,
			Run:      func() error { return runLabelJob(cfg, nil) },
		})
	}
	if cfg.Dedupe.Schedule.Enabled {
		jobs = append(jobs, scheduler.Job{
			Name:     "dedupe",
			Schedule: cfg.Dedupe.Schedule,
			Run:      func() error { return runDedupeJob(cfg) },
		})
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("no scheduled jobs are enabled")
	}
	return jobs, nil
}
