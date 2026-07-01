package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlePages_ListsRoutes(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pages", nil)
	rec := httptest.NewRecorder()
	HandlePages().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="/about"`, `href="/kit"`, `href="/privacy-policy"`, `href="/terms"`,
		`href="/account"`, `href="/admin"`, `href="/dev/emails"`, `href="/pages"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pages index missing %q", want)
		}
	}
}
