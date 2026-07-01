package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"app/report"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// captureReporter records events for assertions.
type captureReporter struct {
	mu     sync.Mutex
	events []report.Event
}

func (c *captureReporter) Report(_ context.Context, e report.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *captureReporter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func TestScheduler_RunsJobThenStopsOnCancel(t *testing.T) {
	s := New(testLogger(), report.Nop{})
	ran := make(chan struct{}, 1)
	s.Add(Job{Name: "tick", Interval: 5 * time.Millisecond, Run: func(context.Context) error {
		select {
		case ran <- struct{}{}:
		default:
		}
		return nil
	}})
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("job did not run within 1s")
	}
	cancel() // must return cleanly; no assertion beyond not hanging
}

func TestScheduler_SkipsDisabledJob(t *testing.T) {
	s := New(testLogger(), report.Nop{})
	s.Add(Job{Name: "off", Interval: 0, Run: func(context.Context) error { return nil }})
	if len(s.jobs) != 0 {
		t.Errorf("job with Interval<=0 should be skipped, got %d jobs", len(s.jobs))
	}
}

func TestScheduler_RunOnceReportsError(t *testing.T) {
	rep := &captureReporter{}
	s := New(testLogger(), rep)
	s.runOnce(context.Background(), Job{Name: "boom", Run: func(context.Context) error {
		return errors.New("nope")
	}})
	if rep.count() != 1 {
		t.Errorf("errored job should report 1 event, got %d", rep.count())
	}
}

func TestScheduler_RunOnceRecoversPanic(t *testing.T) {
	rep := &captureReporter{}
	s := New(testLogger(), rep)
	s.runOnce(context.Background(), Job{Name: "panics", Run: func(context.Context) error {
		panic("kaboom")
	}})
	if rep.count() != 1 {
		t.Errorf("panicking job should report 1 event, got %d", rep.count())
	}
	if rep.events[0].Stack == "" {
		t.Error("panic event should carry a stack trace")
	}
}
