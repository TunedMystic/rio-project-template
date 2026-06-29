package database

import (
	"context"
	"testing"
)

func TestEntitlements_GrantIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "e@example.com", "E")

	has, _ := s.HasEntitlement(ctx, u.ID, "ebook")
	if has {
		t.Fatal("unexpected entitlement before grant")
	}

	if err := s.GrantEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatalf("GrantEntitlement: %v", err)
	}
	// Granting again is a no-op (unique index), not an error.
	if err := s.GrantEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatalf("second GrantEntitlement: %v", err)
	}

	has, _ = s.HasEntitlement(ctx, u.ID, "ebook")
	if !has {
		t.Error("entitlement missing after grant")
	}
	if has, _ := s.HasEntitlement(ctx, u.ID, "other"); has {
		t.Error("unrelated entitlement reported present")
	}

	list, _ := s.ListEntitlements(ctx, u.ID)
	if len(list) != 1 || list[0] != "ebook" {
		t.Errorf("ListEntitlements = %v, want [ebook]", list)
	}

	// Deleting the user cascades to entitlements.
	_ = s.DeleteUser(ctx, u.ID)
	if has, _ := s.HasEntitlement(ctx, u.ID, "ebook"); has {
		t.Error("entitlement not cascaded on user delete")
	}
}

func TestListEntitlements_EmptyIsNonNil(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "z@example.com", "Z")

	list, err := s.ListEntitlements(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListEntitlements: %v", err)
	}
	if list == nil {
		t.Error("ListEntitlements returned nil, want non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}
