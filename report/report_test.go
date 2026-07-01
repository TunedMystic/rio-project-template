package report

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func TestRequestIDContext_RoundTrips(t *testing.T) {
	ctx := ContextWithRequestID(context.Background(), "abc123")
	if got := RequestIDFromContext(ctx); got != "abc123" {
		t.Errorf("RequestIDFromContext = %q, want abc123", got)
	}
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("empty context should yield \"\", got %q", got)
	}
}

func TestNop_DoesNothing(t *testing.T) {
	Nop{}.Report(context.Background(), Event{Message: "x"}) // must not panic
}

func TestWebhook_PostsJSON(t *testing.T) {
	var got Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	NewWebhook(srv.URL).Report(context.Background(), Event{Message: "boom", RequestID: "rid-1"})

	if got.Message != "boom" || got.RequestID != "rid-1" {
		t.Errorf("server received %+v, want Message=boom RequestID=rid-1", got)
	}
}

func TestWebhook_SwallowsTransportError(t *testing.T) {
	// Nothing is listening on this URL; Report must not panic or block long.
	NewWebhook("http://127.0.0.1:0/nope").Report(context.Background(), Event{Message: "x"})
}

func TestCapture_ReportsWithRequestID(t *testing.T) {
	var got Event
	fake := reporterFunc(func(_ context.Context, e Event) { got = e })
	ctx := ContextWithRequestID(context.Background(), "rid-9")
	Capture(ctx, fake, errors.New("kaboom"))
	if got.Message != "kaboom" || got.RequestID != "rid-9" {
		t.Errorf("Capture produced %+v, want Message=kaboom RequestID=rid-9", got)
	}
}

func TestCapture_NilErrorNoOp(t *testing.T) {
	called := false
	fake := reporterFunc(func(_ context.Context, _ Event) { called = true })
	Capture(context.Background(), fake, nil)
	if called {
		t.Error("Capture with nil error should not report")
	}
}

func TestCapture_NilReporterNoOp(t *testing.T) {
	Capture(context.Background(), nil, errors.New("x")) // must not panic
}

// reporterFunc adapts a func to the Reporter interface for tests.
type reporterFunc func(context.Context, Event)

func (f reporterFunc) Report(ctx context.Context, e Event) { f(ctx, e) }
