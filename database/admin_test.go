package database

import (
	"context"
	"path/filepath"
	"testing"
)

func newAdminTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return NewStore(db)
}

func TestListAndCountUsers(t *testing.T) {
	s := newAdminTestStore(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, "alice@example.com", "Alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "bob@other.com", "Bob"); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListUsers(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d users, want 2", len(all))
	}
	// Newest first: bob created after alice.
	if all[0].Email != "bob@other.com" {
		t.Errorf("all[0] = %q, want bob@other.com", all[0].Email)
	}

	filtered, err := s.ListUsers(ctx, "example", 10, 0)
	if err != nil {
		t.Fatalf("ListUsers(query): %v", err)
	}
	if len(filtered) != 1 || filtered[0].Email != "alice@example.com" {
		t.Errorf("search 'example' = %v, want [alice@example.com]", filtered)
	}

	n, err := s.CountUsers(ctx, "example")
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if n != 1 {
		t.Errorf("CountUsers('example') = %d, want 1", n)
	}

	// Paging: limit 1 offset 1 returns the second-newest (alice).
	page2, err := s.ListUsers(ctx, "", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].Email != "alice@example.com" {
		t.Errorf("page2 = %v, want [alice@example.com]", page2)
	}
}

func TestRevokeEntitlement(t *testing.T) {
	s := newAdminTestStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "c@example.com", "C")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GrantEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatalf("RevokeEntitlement: %v", err)
	}
	has, err := s.HasEntitlement(ctx, u.ID, "ebook")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("entitlement should be revoked")
	}
	// Revoking an absent entitlement is a no-op, not an error.
	if err := s.RevokeEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Errorf("revoking absent entitlement should be no-op, got %v", err)
	}
}
