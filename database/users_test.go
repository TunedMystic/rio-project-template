package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return NewStore(db)
}

func TestUsers_CreateLookupUpdateDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "Person@Example.com", "Person")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == 0 || u.CreatedAt.IsZero() {
		t.Fatalf("user not populated: %+v", u)
	}

	// Email lookup is case-insensitive.
	got, err := s.UserByEmail(ctx, "person@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("UserByEmail id = %d, want %d", got.ID, u.ID)
	}

	// Duplicate email (different case) is rejected by the unique index.
	if _, err := s.CreateUser(ctx, "PERSON@example.com", ""); err == nil {
		t.Error("expected duplicate email error")
	}

	if err := s.UpdateUserName(ctx, u.ID, "Renamed"); err != nil {
		t.Fatalf("UpdateUserName: %v", err)
	}
	got, _ = s.UserByID(ctx, u.ID)
	if got.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", got.Name)
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.UserByID(ctx, u.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after delete err = %v, want sql.ErrNoRows", err)
	}
}
