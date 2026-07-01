package main

import (
	"os"
	"strings"
	"testing"
)

func TestEnvExample_ListsAllKeys(t *testing.T) {
	data, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	content := string(data)
	keys := []string{
		"APP_SECRET", "ADDR", "PORT", "BASE_URL", "DB_DIR", "TRUST_PROXY",
		"POSTMARK_TOKEN", "EMAIL_FROM",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_PRO", "STRIPE_PRICE_EBOOK",
		"SESSION_CLEANUP_INTERVAL", "TOKEN_CLEANUP_INTERVAL", "ERROR_WEBHOOK_URL",
		"ADMIN_EMAILS",
	}
	for _, k := range keys {
		if !strings.Contains(content, k) {
			t.Errorf(".env.example missing %s", k)
		}
	}
}
