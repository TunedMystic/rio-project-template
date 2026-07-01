package config

import (
	"reflect"
	"testing"
)

func TestCsvEnv(t *testing.T) {
	t.Run("unset is empty", func(t *testing.T) {
		if got := csvEnv("RIO_TEST_CSV"); len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
	t.Run("splits, trims, lowercases, drops empties", func(t *testing.T) {
		t.Setenv("RIO_TEST_CSV", " Admin@Example.com , ,bob@x.io ")
		got := csvEnv("RIO_TEST_CSV")
		want := []string{"admin@example.com", "bob@x.io"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestNew_ParsesAdminEmails(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "root@example.com")
	c := New("debug", "hash")
	if len(c.AdminEmails) != 1 || c.AdminEmails[0] != "root@example.com" {
		t.Errorf("AdminEmails = %v, want [root@example.com]", c.AdminEmails)
	}
}
