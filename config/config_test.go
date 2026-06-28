package config

import (
	"path/filepath"
	"testing"
)

func TestDBPath_DerivesFromProjectName(t *testing.T) {
	t.Setenv("DB_DIR", "/data")
	got := DBPath("RioProg", false)
	want := filepath.Join("/data", "RioProg.db")
	if got != want {
		t.Errorf("DBPath = %q, want %q", got, want)
	}
}

func TestDBPath_DevDefaultsToCurrentDir(t *testing.T) {
	t.Setenv("DB_DIR", "") // unset -> dev default when debug
	got := DBPath("RioProg", true)
	want := filepath.Join(".", "RioProg.db")
	if got != want {
		t.Errorf("DBPath = %q, want %q", got, want)
	}
}

func TestNew_PopulatesDBPathAndTokens(t *testing.T) {
	t.Setenv("DB_DIR", "/data")
	c := New("production", "abc123")
	if c.ProjectName == "" {
		t.Fatal("ProjectName is empty")
	}
	if c.DBPath != filepath.Join("/data", c.ProjectName+".db") {
		t.Errorf("DBPath = %q", c.DBPath)
	}
	if c.Tokens.ColorPrimary == "" {
		t.Error("Tokens.ColorPrimary is empty")
	}
}

func TestNew_CarriesAssetVersion(t *testing.T) {
	c := New("production", "abc123")
	if c.AssetVersion != "abc123" {
		t.Errorf("AssetVersion = %q, want abc123", c.AssetVersion)
	}
	if c.PageData().AssetVersion != "abc123" {
		t.Errorf("PageData.AssetVersion = %q, want abc123", c.PageData().AssetVersion)
	}
}

func TestAddrFromEnv(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("ADDR", "")
		t.Setenv("PORT", "")
		if got := addrFromEnv(); got != ":3000" {
			t.Errorf("got %q, want :3000", got)
		}
	})
	t.Run("PORT", func(t *testing.T) {
		t.Setenv("ADDR", "")
		t.Setenv("PORT", "8080")
		if got := addrFromEnv(); got != ":8080" {
			t.Errorf("got %q, want :8080", got)
		}
	})
	t.Run("ADDR overrides PORT", func(t *testing.T) {
		t.Setenv("ADDR", "127.0.0.1:9000")
		t.Setenv("PORT", "8080")
		if got := addrFromEnv(); got != "127.0.0.1:9000" {
			t.Errorf("got %q, want 127.0.0.1:9000", got)
		}
	})
}
