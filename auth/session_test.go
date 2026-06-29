package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionCookieRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "tok123", true)

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies", len(cookies))
	}
	c := cookies[0]
	if c.Name != CookieName || c.Value != "tok123" {
		t.Errorf("cookie = %+v", c)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("insecure cookie flags: %+v", c)
	}

	// Reading it back.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	if got := SessionToken(req); got != "tok123" {
		t.Errorf("SessionToken = %q", got)
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, false)
	c := rec.Result().Cookies()[0]
	if c.MaxAge >= 0 {
		t.Errorf("clear cookie MaxAge = %d, want < 0", c.MaxAge)
	}
}
