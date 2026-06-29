package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestSessions_LifecycleAndCascade(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "a@example.com", "")
	exp := time.Now().Add(24 * time.Hour)

	if err := s.CreateSession(ctx, "hash1", u.ID, exp, "agent", "1.2.3.4"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.CreateSession(ctx, "hash2", u.ID, exp, "agent2", "5.6.7.8"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.SessionByID(ctx, "hash1")
	if err != nil || got.UserID != u.ID || got.IP != "1.2.3.4" {
		t.Fatalf("SessionByID = %+v, err %v", got, err)
	}

	list, _ := s.ListUserSessions(ctx, u.ID)
	if len(list) != 2 {
		t.Fatalf("ListUserSessions = %d, want 2", len(list))
	}

	// Sign out everywhere except hash1.
	if err := s.DeleteUserSessions(ctx, u.ID, "hash1"); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}
	list, _ = s.ListUserSessions(ctx, u.ID)
	if len(list) != 1 || list[0].ID != "hash1" {
		t.Fatalf("after delete-except = %+v", list)
	}

	// Deleting the user cascades to sessions.
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.SessionByID(ctx, "hash1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("session not cascaded: %v", err)
	}
}
