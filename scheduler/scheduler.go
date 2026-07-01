// Package scheduler runs registered jobs on fixed intervals in the background,
// each isolated by panic recovery and error reporting, until a context is
// cancelled.
package scheduler

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"app/report"
)

// Job is a unit of periodic work. A Job with Interval <= 0 is never scheduled.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Scheduler owns a set of jobs and runs each on its own goroutine.
type Scheduler struct {
	logger   *slog.Logger
	reporter report.Reporter
	jobs     []Job
}

// New returns a Scheduler that logs via logger and reports failures via reporter.
func New(logger *slog.Logger, reporter report.Reporter) *Scheduler {
	return &Scheduler{logger: logger, reporter: reporter}
}

// Add registers job. Jobs with Interval <= 0 are skipped (disabled by config).
func (s *Scheduler) Add(job Job) {
	if job.Interval <= 0 {
		s.logger.Info("scheduler: job disabled", slog.String("job", job.Name))
		return
	}
	s.jobs = append(s.jobs, job)
}

// Start launches one goroutine per job. Each runs on a ticker until ctx is
// cancelled. Jobs first run after one interval (not immediately).
func (s *Scheduler) Start(ctx context.Context) {
	for _, job := range s.jobs {
		go s.runLoop(ctx, job)
	}
}

func (s *Scheduler) runLoop(ctx context.Context, job Job) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx, job)
		}
	}
}

// runOnce executes job once, recovering panics and reporting any failure.
func (s *Scheduler) runOnce(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("scheduler: job panic", slog.String("job", job.Name))
			s.reporter.Report(ctx, report.Event{
				Message: "scheduler job panic: " + job.Name,
				Stack:   string(debug.Stack()),
			})
		}
	}()
	if err := job.Run(ctx); err != nil {
		s.logger.Error("scheduler: job failed",
			slog.String("job", job.Name), slog.String("err", err.Error()))
		s.reporter.Report(ctx, report.Event{
			Message: "scheduler job failed: " + job.Name + ": " + err.Error(),
		})
	}
}
