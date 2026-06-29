package database

import (
	"context"
	"testing"
)

func TestProcessedWebhookEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if done, err := s.IsEventProcessed(ctx, "evt_1"); err != nil || done {
		t.Fatalf("before record: done=%v err=%v, want false/nil", done, err)
	}
	if err := s.RecordEvent(ctx, "evt_1"); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	if done, _ := s.IsEventProcessed(ctx, "evt_1"); !done {
		t.Error("event not marked processed after RecordEvent")
	}
	// RecordEvent is idempotent: re-recording the same id is a no-op, not an error.
	if err := s.RecordEvent(ctx, "evt_1"); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	// Distinct ids are independent.
	if done, _ := s.IsEventProcessed(ctx, "evt_2"); done {
		t.Error("distinct id should be unprocessed")
	}
}
