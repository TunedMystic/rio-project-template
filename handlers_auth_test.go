// handlers_auth_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/auth"
	"app/database"
	"app/email"
)

type fakeSender struct{ lastBody string }

func (f *fakeSender) Send(ctx context.Context, to, subject, textBody string) error {
	f.lastBody = textBody
	return nil
}

func authTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatal(err)
	}
	return database.NewStore(db)
}

func TestHandleLogin_POST_IssuesAndSends(t *testing.T) {
	store := authTestStore(t)
	sender := &fakeSender{}
	h := HandleLogin(store, sender, auth.NewLimiter(5, time.Minute))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=new@example.com&next=/account"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login/sent?email=new%40example.com" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(sender.lastBody, "/auth/verify?token=") {
		t.Errorf("sent body missing verify link: %q", sender.lastBody)
	}
}

func TestHandleVerify_CreatesUserAndSession(t *testing.T) {
	store := authTestStore(t)

	// Seed a token the way the login handler would.
	token, hash, _ := auth.GenerateToken()
	_ = store.CreateToken(context.Background(), hash, "new@example.com", time.Now().Add(time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/auth/verify?token="+token, nil)
	rec := httptest.NewRecorder()
	HandleVerify(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("no session cookie set")
	}
	if _, err := store.UserByEmail(context.Background(), "new@example.com"); err != nil {
		t.Errorf("user not created: %v", err)
	}
}

func TestClientIP_XFFTrust(t *testing.T) {
	// Standard addr with port: header ignored when trustProxy=false.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.7:5555"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	if got := clientIP(req, false); got != "203.0.113.7" {
		t.Errorf("trustProxy=false: got %q, want 203.0.113.7", got)
	}
	if got := clientIP(req, true); got != "1.2.3.4" {
		t.Errorf("trustProxy=true: got %q, want 1.2.3.4", got)
	}

	// RemoteAddr without port (no colon): falls back to raw value.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "203.0.113.9"
	if got := clientIP(req2, false); got != "203.0.113.9" {
		t.Errorf("no-port fallback: got %q, want 203.0.113.9", got)
	}
}

// ensure the package compiles with the email.Sender interface used
var _ email.Sender = (*fakeSender)(nil)
