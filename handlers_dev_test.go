package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDevEmails_Index(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails", nil)
	rec := httptest.NewRecorder()
	HandleDevEmails().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/dev/emails/login") || !strings.Contains(body, "/dev/emails/notification") {
		t.Error("index missing preview links")
	}
}

func TestHandleDevEmailPreview_HTML(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails/login", nil)
	r.SetPathValue("name", "login")
	rec := httptest.NewRecorder()
	HandleDevEmailPreview().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "SAMPLE_TOKEN") {
		t.Error("preview missing sample link")
	}
}

func TestHandleDevEmailPreview_Text(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails/login?format=text", nil)
	r.SetPathValue("name", "login")
	rec := httptest.NewRecorder()
	HandleDevEmailPreview().ServeHTTP(rec, r)
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q, want text/plain", ct)
	}
	if !strings.Contains(rec.Body.String(), "SAMPLE_TOKEN") {
		t.Error("text preview missing sample link")
	}
}

func TestHandleDevEmailPreview_Unknown(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails/nope", nil)
	r.SetPathValue("name", "nope")
	rec := httptest.NewRecorder()
	HandleDevEmailPreview().ServeHTTP(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
