package database

import (
	"context"
	"testing"
	"time"
)

func TestTokens_ConsumeIsSingleUse(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	exp := time.Now().Add(15 * time.Minute)

	if err := s.CreateToken(ctx, "h1", "a@example.com", exp); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	tok, ok, err := s.ConsumeToken(ctx, "h1")
	if err != nil || !ok {
		t.Fatalf("ConsumeToken ok=%v err=%v", ok, err)
	}
	if tok.Email != "a@example.com" {
		t.Errorf("email = %q", tok.Email)
	}

	// Second consume finds nothing (single-use).
	_, ok, err = s.ConsumeToken(ctx, "h1")
	if err != nil {
		t.Fatalf("2nd ConsumeToken err: %v", err)
	}
	if ok {
		t.Error("token was consumable twice")
	}
}
