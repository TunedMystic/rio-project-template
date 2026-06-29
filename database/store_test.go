package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_CreateAndList(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}

	store := NewStore(db)
	ctx := context.Background()

	if err := store.CreateMessage(ctx, "hello"); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if err := store.CreateMessage(ctx, "world"); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}

	msgs, err := store.ListMessages(ctx)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	// Newest first.
	if msgs[0].Body != "world" {
		t.Errorf("msgs[0].Body = %q, want %q", msgs[0].Body, "world")
	}
}
