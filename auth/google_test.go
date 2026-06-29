// auth/google_test.go
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestStateCookie_RoundTripAndTamper(t *testing.T) {
	rec := httptest.NewRecorder()
	st := OAuthState{State: "abc", Next: "/account", Mode: "login", Verifier: "ver"}
	SetStateCookie(rec, "secret", st, true)

	c := rec.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("insecure state cookie flags: %+v", c)
	}

	req := httptest.NewRequest(http.MethodGet, "/cb", nil)
	req.AddCookie(c)
	got, ok := ReadStateCookie(req, "secret")
	if !ok || got != st {
		t.Fatalf("ReadStateCookie = %+v ok=%v, want %+v", got, ok, st)
	}

	// A wrong secret (forged signature) is rejected.
	req2 := httptest.NewRequest(http.MethodGet, "/cb", nil)
	req2.AddCookie(c)
	if _, ok := ReadStateCookie(req2, "different-secret"); ok {
		t.Error("ReadStateCookie accepted a cookie signed with another secret")
	}
}

func TestAuthCodeURL_HasStateAndPKCE(t *testing.T) {
	g := NewGoogleOAuth("cid", "csec", "https://app/cb")
	u := g.AuthCodeURL("state-xyz", NewVerifier())
	parsed, _ := url.Parse(u)
	if !strings.HasPrefix(u, "https://accounts.google.com/o/oauth2/auth") {
		t.Errorf("auth url = %q", u)
	}
	q := parsed.Query()
	if q.Get("state") != "state-xyz" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Errorf("missing PKCE challenge: %v", q)
	}
	if q.Get("client_id") != "cid" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
}

func TestFetchUser_DecodesUserinfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":            "sub-1",
			"email":          "a@example.com",
			"email_verified": true,
			"name":           "Ada",
		})
	}))
	defer srv.Close()

	g := NewGoogleOAuth("cid", "csec", "https://app/cb")
	g.UserinfoURL = srv.URL
	gu, err := g.FetchUser(context.Background(), &oauth2.Token{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("FetchUser: %v", err)
	}
	if gu.Sub != "sub-1" || gu.Email != "a@example.com" || !gu.EmailVerified || gu.Name != "Ada" {
		t.Errorf("decoded user = %+v", gu)
	}
}
