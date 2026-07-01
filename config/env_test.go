package config

import (
	"testing"
	"time"
)

func TestDurationFromEnv(t *testing.T) {
	t.Run("unset uses default", func(t *testing.T) {
		if got := durationFromEnv("RIO_TEST_DUR", 2*time.Hour); got != 2*time.Hour {
			t.Errorf("got %s, want 2h", got)
		}
	})
	t.Run("valid parses", func(t *testing.T) {
		t.Setenv("RIO_TEST_DUR", "30m")
		if got := durationFromEnv("RIO_TEST_DUR", time.Hour); got != 30*time.Minute {
			t.Errorf("got %s, want 30m", got)
		}
	})
	t.Run("invalid uses default", func(t *testing.T) {
		t.Setenv("RIO_TEST_DUR", "not-a-duration")
		if got := durationFromEnv("RIO_TEST_DUR", time.Hour); got != time.Hour {
			t.Errorf("got %s, want 1h", got)
		}
	})
}

func TestNew_SetsOpsDefaults(t *testing.T) {
	c := New("debug", "hash")
	if c.SessionCleanupInterval != time.Hour {
		t.Errorf("SessionCleanupInterval = %s, want 1h", c.SessionCleanupInterval)
	}
	if c.TokenCleanupInterval != time.Hour {
		t.Errorf("TokenCleanupInterval = %s, want 1h", c.TokenCleanupInterval)
	}
}
