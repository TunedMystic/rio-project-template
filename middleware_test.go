package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/report"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type capReporter struct{ events []report.Event }

func (c *capReporter) Report(_ context.Context, e report.Event) { c.events = append(c.events, e) }

func TestRequestID_SetsHeaderAndContext(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = report.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	hdr := rec.Header().Get("X-Request-ID")
	if hdr == "" {
		t.Fatal("X-Request-ID header not set")
	}
	if seen != hdr {
		t.Errorf("context id %q != header id %q", seen, hdr)
	}
}

func TestRequestID_HonorsInbound(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "inbound-42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "inbound-42" {
		t.Errorf("X-Request-ID = %q, want inbound-42", got)
	}
}

func TestRecoverAndReport_Returns500AndReports(t *testing.T) {
	rep := &capReporter{}
	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	// Compose with RequestID so the event carries an id.
	h := RequestID(RecoverAndReport(discardLogger(), rep)(panicky))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if len(rep.events) != 1 {
		t.Fatalf("reporter got %d events, want 1", len(rep.events))
	}
	e := rep.events[0]
	if e.Stack == "" {
		t.Error("event missing stack trace")
	}
	if e.RequestID == "" || e.RequestID != rec.Header().Get("X-Request-ID") {
		t.Errorf("event RequestID %q != header %q", e.RequestID, rec.Header().Get("X-Request-ID"))
	}
	if e.Method != http.MethodGet || e.URL != "/x" {
		t.Errorf("event method/url = %s %s, want GET /x", e.Method, e.URL)
	}
}

func TestLogRequests_PassesThrough(t *testing.T) {
	h := LogRequests(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rec.Code)
	}
}
