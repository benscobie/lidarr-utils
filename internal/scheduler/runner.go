package scheduler

import (
	"context"
	"log"
	"sync"

	"github.com/benscobie/lidarr-utils/internal/config"
)

type Job struct {
	Name     string
	Schedule config.ScheduleConfig
	Run      func() error
}

type Runner struct {
	logger *log.Logger

	mu          sync.Mutex
	accepting   bool
	contextDone <-chan struct{}
	pending     map[string]struct{}
	commands    chan Job
}

func NewRunner(logger *log.Logger) *Runner {
	if logger == nil {
		logger = log.Default()
	}
	return &Runner{
		logger:    logger,
		accepting: true,
		pending:   make(map[string]struct{}),
		commands:  make(chan Job, 64),
	}
}

func (r *Runner) Enqueue(job Job) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.contextDone != nil {
		select {
		case <-r.contextDone:
			r.accepting = false
		default:
		}
	}
	if !r.accepting || job.Name == "" || job.Run == nil {
		return false
	}
	if _, duplicate := r.pending[job.Name]; duplicate {
		return false
	}

	select {
	case r.commands <- job:
		r.pending[job.Name] = struct{}{}
		return true
	default:
		return false
	}
}

type jobResult struct {
	job Job
	err error
}

func (r *Runner) Run(ctx context.Context) {
	r.mu.Lock()
	r.contextDone = ctx.Done()
	select {
	case <-ctx.Done():
		r.accepting = false
	default:
	}
	r.mu.Unlock()

	var (
		queue     []Job
		active    bool
		stopping  bool
		cancelled = ctx.Done()
		completed = make(chan jobResult, 1)
	)
	beginShutdown := func() {
		stopping = true
		cancelled = nil
		r.rejectNewJobs()
		for _, job := range queue {
			r.clearPending(job.Name)
		}
		queue = nil
		r.dropCommands()
	}

	for {
		if !stopping {
			select {
			case <-ctx.Done():
				beginShutdown()
			default:
			}
		}
		if !stopping && !active && len(queue) > 0 {
			job := queue[0]
			queue = queue[1:]
			active = true
			r.logger.Printf("Starting scheduled job %s", job.Name)
			go func() {
				completed <- jobResult{job: job, err: job.Run()}
			}()
		}
		if stopping && !active {
			r.rejectNewJobs()
			r.dropCommands()
			return
		}

		select {
		case job := <-r.commands:
			if stopping {
				r.clearPending(job.Name)
				continue
			}
			queue = append(queue, job)

		case result := <-completed:
			active = false
			r.clearPending(result.job.Name)
			if result.err != nil {
				r.logger.Printf("Scheduled job %s failed: %v", result.job.Name, result.err)
			} else {
				r.logger.Printf("Scheduled job %s completed", result.job.Name)
			}

		case <-cancelled:
			beginShutdown()
		}
	}
}

func (r *Runner) rejectNewJobs() {
	r.mu.Lock()
	r.accepting = false
	r.mu.Unlock()
}

func (r *Runner) clearPending(name string) {
	r.mu.Lock()
	delete(r.pending, name)
	r.mu.Unlock()
}

func (r *Runner) dropCommands() {
	for {
		select {
		case job := <-r.commands:
			r.clearPending(job.Name)
		default:
			return
		}
	}
}
