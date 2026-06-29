package auth

import (
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("first two should be allowed")
	}
	if l.Allow("k") {
		t.Error("third should be denied")
	}
	if !l.Allow("other") {
		t.Error("different key should be allowed")
	}
}
