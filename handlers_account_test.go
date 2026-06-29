// handlers_account_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/auth"
	"app/database"
)

// loggedInRequestSession creates a user + real session row in the store and
// returns them along with the raw (unhashed) token needed to build the cookie.
func loggedInRequestSession(t *testing.T, store *database.Store) (database.Session, database.User) {
	t.Helper()
	u, err := store.CreateUser(context.Background(), "me@example.com", "Me")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	token, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if err := store.CreateSession(context.Background(), hash, u.ID, time.Now().Add(time.Hour), "ua", "ip"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sess, err := store.SessionByID(context.Background(), hash)
	if err != nil {
		t.Fatalf("SessionByID: %v", err)
	}
	// Stash the raw token in the session's UserAgent field so loggedInWith can
	// retrieve it — a bit of a hack but keeps the signature clean.
	// Better: embed the raw token via a small wrapper type. We'll use a closure
	// approach: return the sess but store the raw token in a test-local map.
	// Simplest: embed it in a package-level map keyed by session ID.
	rawTokenBySessionID[sess.ID] = token
	return sess, u
}

// rawTokenBySessionID maps session.ID → raw token so loggedInWith can look it up.
var rawTokenBySessionID = map[string]string{}

// loggedInWith builds an HTTP request carrying the given session's cookie and
// runs it through auth.LoadUser so the context carries user+session exactly as
// in production. The returned *http.Request has the user and session in its
// context.
func loggedInWith(t *testing.T, store *database.Store, u database.User, sess database.Session, method, target, body string) (*http.Request, database.User) {
	t.Helper()
	rawToken := rawTokenBySessionID[sess.ID]
	if rawToken == "" {
		t.Fatalf("loggedInWith: no raw token found for session %s — call loggedInRequestSession first", sess.ID)
	}

	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: rawToken})

	// Run through LoadUser so the context is populated exactly like prod.
	var out *http.Request
	auth.LoadUser(store)(http.HandlerFunc(func(w http.ResponseWriter, rr *http.Request) {
		out = rr
	})).ServeHTTP(httptest.NewRecorder(), r)
	return out, u
}

func TestHandleAccount_POSTUpdatesName(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	csrf := auth.CSRFToken(Conf.AppSecret, sess.ID)

	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account", "name=Renamed&_csrf="+csrf)
	rec := httptest.NewRecorder()
	HandleAccount(store).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	got, _ := store.UserByID(context.Background(), u.ID)
	if got.Name != "Renamed" {
		t.Errorf("name=%q, want Renamed", got.Name)
	}
}

func TestHandleAccount_POSTBadCSRF(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account", "name=X&_csrf=wrong")
	rec := httptest.NewRecorder()
	HandleAccount(store).ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestHandleDisconnectGoogle_ClearsLink(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	_ = store.SetUserGoogleID(context.Background(), u.ID, "sub-xyz")
	csrf := auth.CSRFToken(Conf.AppSecret, sess.ID)

	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/google/disconnect", "_csrf="+csrf)
	rec := httptest.NewRecorder()
	HandleDisconnectGoogle(store).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	got, _ := store.UserByID(context.Background(), u.ID)
	if got.GoogleID != "" {
		t.Errorf("GoogleID = %q, want empty after disconnect", got.GoogleID)
	}
}

func TestHandleDisconnectGoogle_BadCSRF(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/google/disconnect", "_csrf=wrong")
	rec := httptest.NewRecorder()
	HandleDisconnectGoogle(store).ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
