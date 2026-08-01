package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benscobie/lidarr-utils/internal/config"
)

func TestRunnerSerializesJobsAndCoalescesDuplicates(t *testing.T) {
	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(runDone)
	}()

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	executed := make(chan string, 2)
	var active atomic.Int32
	var maxActive atomic.Int32
	job := func(name string, block bool) Job {
		return Job{
			Name:     name,
			Schedule: config.ScheduleConfig{Enabled: true},
			Run: func() error {
				current := active.Add(1)
				for {
					previous := maxActive.Load()
					if current <= previous || maxActive.CompareAndSwap(previous, current) {
						break
					}
				}
				executed <- name
				if block {
					close(firstStarted)
					<-releaseFirst
				}
				active.Add(-1)
				return nil
			},
		}
	}

	if !runner.Enqueue(job("first", true)) {
		t.Fatal("expected first job to be accepted")
	}
	awaitSignal(t, firstStarted)
	if runner.Enqueue(job("first", false)) {
		t.Fatal("expected duplicate job to be coalesced")
	}
	if !runner.Enqueue(job("second", false)) {
		t.Fatal("expected different job to be accepted")
	}
	close(releaseFirst)

	if first, second := awaitValue(t, executed), awaitValue(t, executed); first != "first" ||
		second != "second" {
		t.Fatalf("jobs ran out of FIFO order: %q, %q", first, second)
	}
	if maxActive.Load() != 1 {
		t.Fatalf("expected serialized execution, max active was %d", maxActive.Load())
	}
	cancel()
	awaitSignal(t, runDone)
}

func TestRunnerContinuesAfterJobFailure(t *testing.T) {
	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(runDone)
	}()

	secondRan := make(chan struct{})
	if !runner.Enqueue(Job{Name: "first", Run: func() error {
		return errors.New("failed")
	}}) {
		t.Fatal("expected first job to be accepted")
	}
	if !runner.Enqueue(Job{Name: "second", Run: func() error {
		close(secondRan)
		return nil
	}}) {
		t.Fatal("expected second job to be accepted")
	}
	awaitSignal(t, secondRan)
	cancel()
	awaitSignal(t, runDone)
}

func TestRunnerShutdownDropsQueuedJobsAndWaitsForActive(t *testing.T) {
	runner := NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		runner.Run(ctx)
		close(runDone)
	}()

	activeStarted := make(chan struct{})
	releaseActive := make(chan struct{})
	queuedRan := make(chan struct{})
	if !runner.Enqueue(Job{Name: "active", Run: func() error {
		close(activeStarted)
		<-releaseActive
		return nil
	}}) {
		t.Fatal("expected active job to be accepted")
	}
	awaitSignal(t, activeStarted)
	if !runner.Enqueue(Job{Name: "queued", Run: func() error {
		close(queuedRan)
		return nil
	}}) {
		t.Fatal("expected queued job to be accepted")
	}

	cancel()
	if runner.Enqueue(Job{Name: "new", Run: func() error { return nil }}) {
		t.Fatal("expected enqueue after cancellation to be rejected")
	}
	select {
	case <-runDone:
		t.Fatal("runner exited before active job completed")
	default:
	}
	close(releaseActive)
	awaitSignal(t, runDone)
	select {
	case <-queuedRan:
		t.Fatal("queued job ran during shutdown")
	default:
	}
}

func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for signal")
	}
}

func awaitValue(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for value")
		return ""
	}
}
