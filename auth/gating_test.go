// auth/gating_test.go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"app/database"
)

func TestHasActiveSubscription(t *testing.T) {
	for status, want := range map[string]bool{"active": true, "trialing": true, "past_due": false, "canceled": false, "": false} {
		if got := HasActiveSubscription(database.User{SubscriptionStatus: status}); got != want {
			t.Errorf("HasActiveSubscription(%q) = %v, want %v", status, got, want)
		}
	}
}

func reqWithUser(u database.User) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/premium", nil)
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
}

func TestRequireSubscription(t *testing.T) {
	ok := RequireSubscription(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	rec := httptest.NewRecorder()
	ok.ServeHTTP(rec, reqWithUser(database.User{SubscriptionStatus: "active"}))
	if rec.Code != 200 {
		t.Errorf("active subscriber blocked: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	ok.ServeHTTP(rec, reqWithUser(database.User{SubscriptionStatus: ""}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account/billing" {
		t.Errorf("non-subscriber not redirected: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRequireEntitlement(t *testing.T) {
	db, _ := database.Open(filepath.Join(t.TempDir(), "g.db"))
	t.Cleanup(func() { db.Close() })
	_ = database.MigrateUp(db)
	store := database.NewStore(db)
	u, _ := store.CreateUser(context.Background(), "g@example.com", "G")
	_ = store.GrantEntitlement(context.Background(), u.ID, "ebook")

	guard := RequireEntitlement(store, "ebook")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, reqWithUser(u))
	if rec.Code != 200 {
		t.Errorf("owner blocked: %d", rec.Code)
	}

	other, _ := store.CreateUser(context.Background(), "h@example.com", "H")
	rec = httptest.NewRecorder()
	guard.ServeHTTP(rec, reqWithUser(other))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("non-owner not redirected: %d", rec.Code)
	}
}

func TestRequireSubscription_NoUser(t *testing.T) {
	guard := RequireSubscription(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not run when no user is in context")
	}))
	rec := httptest.NewRecorder()
	// A bare request with no user in context (LoadUser never ran).
	guard.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/premium", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account/billing" {
		t.Errorf("no-user: got %d %q, want 303 /account/billing", rec.Code, rec.Header().Get("Location"))
	}
}
