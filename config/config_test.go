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
	c := New("production")
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
