// handlers_auth_test.go
package main

import (
	"context"
	"encoding/json"
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

type fakeSender struct{ lastMsg email.Message }

func (f *fakeSender) Send(ctx context.Context, msg email.Message) error {
	f.lastMsg = msg
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
	if !strings.Contains(sender.lastMsg.Text, "/auth/verify?token=") {
		t.Errorf("sent message missing verify link: %q", sender.lastMsg.Text)
	}
	if !strings.Contains(sender.lastMsg.HTML, "/auth/verify?token=") {
		t.Errorf("sent message HTML missing verify link: %q", sender.lastMsg.HTML)
	}
}

func TestHandleLogin_HoneypotDropped(t *testing.T) {
	store := authTestStore(t)
	sender := &fakeSender{}
	h := HandleLogin(store, sender, auth.NewLimiter(5, time.Minute))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=bot@example.com&website=filled"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if sender.lastMsg.Text != "" {
		t.Errorf("honeypot submission should not send an email, got %q", sender.lastMsg.Text)
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

func TestGoogleSignIn_CreateLinkAndVerify(t *testing.T) {
	store := authTestStore(t)
	ctx := context.Background()

	// New user is created with google_id and name backfilled.
	u, err := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s1", Email: "new@example.com", EmailVerified: true, Name: "Neo"})
	if err != nil {
		t.Fatalf("googleSignIn(new): %v", err)
	}
	if u.GoogleID != "s1" || u.Name != "Neo" {
		t.Errorf("created user = %+v", u)
	}

	// Same google_id returns the same user.
	again, _ := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s1", Email: "new@example.com", EmailVerified: true})
	if again.ID != u.ID {
		t.Errorf("second sign-in id = %d, want %d", again.ID, u.ID)
	}

	// Existing email (magic-link user) gets linked.
	existing, _ := store.CreateUser(ctx, "old@example.com", "Old")
	linked, err := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s2", Email: "old@example.com", EmailVerified: true})
	if err != nil || linked.ID != existing.ID || linked.GoogleID != "s2" {
		t.Errorf("link-by-email = %+v, err %v", linked, err)
	}

	// Unverified email is rejected.
	if _, err := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s3", Email: "x@example.com", EmailVerified: false}); err == nil {
		t.Error("expected rejection for unverified email")
	}
}

func TestGoogleLink_RejectsAlreadyLinked(t *testing.T) {
	store := authTestStore(t)
	ctx := context.Background()
	a, _ := store.CreateUser(ctx, "a@example.com", "A")
	_ = store.SetUserGoogleID(ctx, a.ID, "shared")
	b, _ := store.CreateUser(ctx, "b@example.com", "B")

	if err := googleLink(ctx, store, b, auth.GoogleUser{Sub: "shared", Email: "b@example.com", EmailVerified: true}); err == nil {
		t.Error("expected error linking a google_id already used by another user")
	}
	if err := googleLink(ctx, store, b, auth.GoogleUser{Sub: "fresh", Email: "b@example.com", EmailVerified: true}); err != nil {
		t.Errorf("googleLink(fresh): %v", err)
	}
	got, _ := store.UserByID(ctx, b.ID)
	if got.GoogleID != "fresh" {
		t.Errorf("b.GoogleID = %q, want fresh", got.GoogleID)
	}
}

func TestGoogleCallback_RejectsBadState(t *testing.T) {
	store := authTestStore(t)
	oauth := auth.NewGoogleOAuth("cid", "csec", "http://localhost/cb")
	// No state cookie at all → redirect to /login, no network call.
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=x&state=y", nil)
	rec := httptest.NewRecorder()
	HandleGoogleCallback(store, oauth).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("bad-state callback = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestGoogleLogin_RedirectsToGoogle(t *testing.T) {
	oauth := auth.NewGoogleOAuth("cid", "csec", "http://localhost/cb")
	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	rec := httptest.NewRecorder()
	HandleGoogleLogin(oauth).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "https://accounts.google.com/") {
		t.Errorf("Location = %q", loc)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Error("expected a state cookie to be set")
	}
}

// fakeGoogleServer builds a minimal httptest server that responds to
// /token and /userinfo.  tokenStatus is the HTTP status for /token;
// userinfoStatus is the HTTP status for /userinfo.
func fakeGoogleServer(t *testing.T, tokenStatus, userinfoStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if tokenStatus != http.StatusOK {
				http.Error(w, "bad_request", tokenStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "x",
				"token_type":   "Bearer",
			})
		case "/userinfo":
			if userinfoStatus != http.StatusOK {
				http.Error(w, "server_error", userinfoStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"sub":            "uid1",
				"email":          "u@example.com",
				"email_verified": true,
				"name":           "User",
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

// buildCallbackRequest creates a GET request to the Google callback URL with a
// valid signed state cookie already attached.
func buildCallbackRequest(t *testing.T) *http.Request {
	t.Helper()
	// Mint a signed state cookie using a helper recorder.
	rec0 := httptest.NewRecorder()
	auth.SetStateCookie(rec0, Conf.AppSecret,
		auth.OAuthState{State: "st1", Next: "/", Mode: "login", Verifier: auth.NewVerifier()},
		false)
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=st1&code=abc", nil)
	for _, c := range rec0.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

// TestGoogleCallback_FetchUserFails verifies that a 500 from Google's userinfo
// endpoint renders the friendly VerifyError page (200) instead of a bare 500.
func TestGoogleCallback_FetchUserFails(t *testing.T) {
	srv := fakeGoogleServer(t, http.StatusOK, http.StatusInternalServerError)
	defer srv.Close()

	store := authTestStore(t)
	gOAuth := auth.NewGoogleOAuth("cid", "csec", "http://localhost/cb")
	gOAuth.SetEndpoint(srv.URL+"/auth", srv.URL+"/token")
	gOAuth.UserinfoURL = srv.URL + "/userinfo"

	req := buildCallbackRequest(t)
	rec := httptest.NewRecorder()
	HandleGoogleCallback(store, gOAuth).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("FetchUser-fail: got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Link expired") {
		t.Errorf("FetchUser-fail: response body missing 'Link expired'; got: %q", body[:min(len(body), 500)])
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			t.Errorf("FetchUser-fail: unexpected session cookie set: %v", c)
		}
	}
}

// TestGoogleCallback_ExchangeFails verifies that a 400 from Google's token
// endpoint renders the friendly VerifyError page (200) instead of a bare 500.
func TestGoogleCallback_ExchangeFails(t *testing.T) {
	srv := fakeGoogleServer(t, http.StatusBadRequest, http.StatusOK)
	defer srv.Close()

	store := authTestStore(t)
	gOAuth := auth.NewGoogleOAuth("cid", "csec", "http://localhost/cb")
	gOAuth.SetEndpoint(srv.URL+"/auth", srv.URL+"/token")
	gOAuth.UserinfoURL = srv.URL + "/userinfo"

	req := buildCallbackRequest(t)
	rec := httptest.NewRecorder()
	HandleGoogleCallback(store, gOAuth).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Exchange-fail: got status %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Link expired") {
		t.Errorf("Exchange-fail: response body missing 'Link expired'; got: %q", body[:min(len(body), 500)])
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			t.Errorf("Exchange-fail: unexpected session cookie set: %v", c)
		}
	}
}
