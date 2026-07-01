package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"app/report"

	"github.com/tunedmystic/rio"
)

// RequestID attaches a request id to the request context and the X-Request-ID
// response header. An inbound X-Request-ID (e.g. from a trusted proxy) is
// honored; otherwise a random id is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(report.ContextWithRequestID(r.Context(), id)))
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// statusRecorder captures the response status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// LogRequests logs each request with method, url, status, duration and request
// id. It replaces rio's default LogRequest so logs carry the request id.
func LogRequests(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)
			logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.Int("status", rec.status),
				slog.String("method", r.Method),
				slog.String("url", r.URL.RequestURI()),
				slog.String("request_id", report.RequestIDFromContext(r.Context())),
				slog.Duration("time", time.Since(start)),
			)
		})
	}
}

// RecoverAndReport recovers from panics in downstream handlers, reports them
// (with a stack trace and request context), logs, and writes a 500. It replaces
// rio's default RecoverPanic to add stack capture and external reporting.
func RecoverAndReport(logger *slog.Logger, reporter report.Reporter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := report.RequestIDFromContext(r.Context())
					msg := fmt.Sprintf("%v", rec)
					logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
						slog.String("panic", msg),
						slog.String("request_id", reqID),
					)
					reporter.Report(r.Context(), report.Event{
						Message:   "panic: " + msg,
						Stack:     string(debug.Stack()),
						RequestID: reqID,
						Method:    r.Method,
						URL:       r.URL.RequestURI(),
					})
					w.Header().Set("Connection", "close")
					rio.Http500(w)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
