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

func TestNew_LoadsAuthEnv(t *testing.T) {
	t.Setenv("BASE_URL", "https://app.example.com")
	t.Setenv("APP_SECRET", "supersecret")
	t.Setenv("POSTMARK_TOKEN", "pm-tok")
	t.Setenv("EMAIL_FROM", "noreply@example.com")

	c := New("production", "h")
	if c.BaseURL != "https://app.example.com" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.AppSecret != "supersecret" || c.PostmarkToken != "pm-tok" || c.EmailFrom != "noreply@example.com" {
		t.Errorf("auth env not loaded: %+v", c)
	}
}

func TestPageDataFor_CarriesAccount(t *testing.T) {
	c := New("debug", "h")
	pd := c.PageDataFor(Account{LoggedIn: true, Name: "Sam", Email: "sam@example.com"})
	if !pd.Account.LoggedIn || pd.Account.Email != "sam@example.com" {
		t.Errorf("account not carried: %+v", pd.Account)
	}
}

func TestAppSecret_DevDefaults(t *testing.T) {
	t.Setenv("APP_SECRET", "")
	if got := appSecretFromEnv(true); got == "" {
		t.Error("dev APP_SECRET should fall back to a default")
	}
	t.Setenv("APP_SECRET", "")
	if got := appSecretFromEnv(false); got != "" {
		t.Error("prod APP_SECRET should stay empty when unset (caller fails fast)")
	}
}
