// Package report provides a pluggable error-reporting seam: a Reporter
// interface with a no-op default and an optional JSON webhook implementation,
// plus request-id context helpers shared by the app middleware.
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type ctxKey int

const requestIDKey ctxKey = 0

// ContextWithRequestID returns a copy of ctx carrying the request id.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request id in ctx, or "" if absent.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// Event is a single reportable error occurrence.
type Event struct {
	Message   string `json:"message"`
	Stack     string `json:"stack,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Method    string `json:"method,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Reporter reports error events to an external sink.
type Reporter interface {
	Report(ctx context.Context, e Event)
}

// Nop is the default reporter; it does nothing (errors are still logged).
type Nop struct{}

// Report satisfies Reporter and does nothing.
func (Nop) Report(context.Context, Event) {}

// Webhook posts events as JSON to a collector URL.
type Webhook struct {
	URL    string
	Client *http.Client
}

// NewWebhook returns a Webhook reporter posting to url with a short timeout.
func NewWebhook(url string) Webhook {
	return Webhook{URL: url, Client: &http.Client{Timeout: 5 * time.Second}}
}

// Report posts e as JSON. It is best-effort: any failure is logged and
// swallowed so reporting never breaks request handling. A fresh timeout
// context is used so a cancelled request context does not abort the report.
func (w Webhook) Report(_ context.Context, e Event) {
	body, err := json.Marshal(e)
	if err != nil {
		slog.Error("report: marshal event", slog.String("err", err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		slog.Error("report: build request", slog.String("err", err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := w.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("report: post event", slog.String("err", err.Error()))
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
