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
)

func TestHandleHealth(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "hz.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	HandleHealth(db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthy status = %d, want 200", rec.Code)
	}

	// A closed DB is unhealthy.
	db.Close()
	rec = httptest.NewRecorder()
	HandleHealth(db).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unhealthy status = %d, want 503", rec.Code)
	}
}

func newTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return database.NewStore(db)
}

func TestHandleMessages_GET(t *testing.T) {
	store := newTestStore(t)
	_ = store.CreateMessage(context.Background(), "seeded-message")

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rec := httptest.NewRecorder()
	HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "seeded-message") {
		t.Error("response missing seeded message")
	}
}

func TestHandleMessages_POSTCreatesAndRedirects(t *testing.T) {
	store := newTestStore(t)

	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body=created-here"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 1 || msgs[0].Body != "created-here" {
		t.Errorf("message not persisted: %+v", msgs)
	}
}

func TestHandleRobots(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rec := httptest.NewRecorder()
	HandleRobots().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if !strings.Contains(rec.Body.String(), "User-agent") {
		t.Error("robots.txt missing User-agent directive")
	}
}

func TestHandleMessages_POSTBlankShowsError(t *testing.T) {
	store := newTestStore(t)

	// "+++" decodes to "   " (spaces); after trimming it is blank.
	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body=+++"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "cannot be blank") {
		t.Error("response missing validation error")
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 0 {
		t.Errorf("blank message should not persist, got %d", len(msgs))
	}
}

func TestHandleMessages_HoneypotDropped(t *testing.T) {
	store := newTestStore(t)

	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body=spam&website=filled"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 0 {
		t.Errorf("honeypot submission should not persist, got %d", len(msgs))
	}
}

func TestHandleMessages_RateLimited(t *testing.T) {
	store := newTestStore(t)
	h := HandleMessages(store, auth.NewLimiter(1, time.Minute)) // allow 1, block the 2nd

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body="+body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := post("first"); rec.Code != http.StatusSeeOther {
		t.Fatalf("first status=%d, want 303", rec.Code)
	}
	rec := post("second")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Too many submissions") {
		t.Error("rate-limited response missing notice")
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 1 {
		t.Errorf("only the first message should persist, got %d", len(msgs))
	}
}
