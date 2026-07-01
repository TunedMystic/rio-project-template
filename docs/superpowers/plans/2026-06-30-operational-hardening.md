# Operational Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an in-process job scheduler (wiring the existing session/token cleanup), a pluggable error reporter with per-request IDs, a Litestream backup doc set, and a complete `.env.example` — turning the template's operational envelope production-ready.

**Architecture:** Two new leaf packages (`report`, `scheduler`) with no app dependencies beyond each other; app-level middleware in `package main` for request IDs and panic reporting; a small wiring change in `main.run()` to build the server with an explicit middleware list and start the scheduler under the existing shutdown context. Backups are documentation + example config only (Litestream sidecar), no Go code.

**Tech Stack:** Go 1.26, `log/slog`, `net/http`, `crypto/rand`, `runtime/debug`, `github.com/tunedmystic/rio`. No new Go module dependencies.

## Global Constraints

- **Zero new Go module dependencies.** Request IDs use `crypto/rand`; the webhook reporter uses `net/http`; Litestream is external (docs/compose only).
- **Optional and env-gated:** absent/zero config disables a feature; the app still runs (matches the Google/Stripe pattern). A scheduler job with `Interval <= 0` is skipped; an empty `ERROR_WEBHOOK_URL` selects the no-op reporter.
- Preserve the scratch single-binary deploy (no CGO, no shell in the runtime image).
- All background work stops on the shutdown `ctx` created in `main.run()` via `signal.NotifyContext`.
- Follow existing idioms: `config.New` env helpers, `database.Store` methods, `rio` middleware signatures, table-driven tests writing to `io.Discard` loggers.
- Run tests with `go test ./...`. The module path is `app` (so packages import as `app/report`, `app/scheduler`).

---

### Task 1: `report` package — reporter interface, Nop, Webhook, request-id context, Capture

**Files:**
- Create: `report/report.go`
- Test: `report/report_test.go`

**Interfaces:**
- Consumes: nothing (leaf package).
- Produces:
  - `type Event struct { Message, Err, Stack, RequestID, Method, URL string }` (all JSON-tagged)
  - `type Reporter interface { Report(ctx context.Context, e Event) }`
  - `type Nop struct{}` with `Report`
  - `type Webhook struct { URL string; Client *http.Client }`, `func NewWebhook(url string) Webhook`, `Report`
  - `func ContextWithRequestID(ctx context.Context, id string) context.Context`
  - `func RequestIDFromContext(ctx context.Context) string`
  - `func Capture(ctx context.Context, r Reporter, err error)`

- [ ] **Step 1: Write the failing tests**

Create `report/report_test.go`:

```go
package report

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

// reporterFunc adapts a func to the Reporter interface for tests.
type reporterFunc func(context.Context, Event)

func (f reporterFunc) Report(ctx context.Context, e Event) { f(ctx, e) }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./report/ -v`
Expected: FAIL — build error, `undefined: ContextWithRequestID` etc.

- [ ] **Step 3: Write the implementation**

Create `report/report.go`:

```go
// Package report provides a pluggable error-reporting seam: a Reporter
// interface with a no-op default and an optional JSON webhook implementation,
// plus request-id context helpers shared by the app middleware.
package report

import (
	"bytes"
	"context"
	"encoding/json"
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
	Err       string `json:"error,omitempty"`
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
	_ = resp.Body.Close()
}

// Capture reports err as an event, enriching it with the request id from ctx.
// It is a no-op when err or r is nil.
func Capture(ctx context.Context, r Reporter, err error) {
	if err == nil || r == nil {
		return
	}
	r.Report(ctx, Event{
		Message:   err.Error(),
		Err:       err.Error(),
		RequestID: RequestIDFromContext(ctx),
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./report/ -v`
Expected: PASS (all 6 tests).

- [ ] **Step 5: Commit**

```bash
git add report/report.go report/report_test.go
git commit -m "feat: add report package (pluggable error reporter + request-id context)"
```

---

### Task 2: `scheduler` package — interval job runner

**Files:**
- Create: `scheduler/scheduler.go`
- Test: `scheduler/scheduler_test.go`

**Interfaces:**
- Consumes: `report.Reporter`, `report.Event` from Task 1.
- Produces:
  - `type Job struct { Name string; Interval time.Duration; Run func(ctx context.Context) error }`
  - `type Scheduler struct{ ... }`
  - `func New(logger *slog.Logger, reporter report.Reporter) *Scheduler`
  - `func (s *Scheduler) Add(job Job)` (skips `Interval <= 0`)
  - `func (s *Scheduler) Start(ctx context.Context)`

- [ ] **Step 1: Write the failing tests**

Create `scheduler/scheduler_test.go`:

```go
package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"app/report"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// captureReporter records events for assertions.
type captureReporter struct {
	mu     sync.Mutex
	events []report.Event
}

func (c *captureReporter) Report(_ context.Context, e report.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
}

func (c *captureReporter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

func TestScheduler_RunsJobThenStopsOnCancel(t *testing.T) {
	s := New(testLogger(), report.Nop{})
	ran := make(chan struct{}, 1)
	s.Add(Job{Name: "tick", Interval: 5 * time.Millisecond, Run: func(context.Context) error {
		select {
		case ran <- struct{}{}:
		default:
		}
		return nil
	}})
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	select {
	case <-ran:
	case <-time.After(time.Second):
		t.Fatal("job did not run within 1s")
	}
	cancel() // must return cleanly; no assertion beyond not hanging
}

func TestScheduler_SkipsDisabledJob(t *testing.T) {
	s := New(testLogger(), report.Nop{})
	s.Add(Job{Name: "off", Interval: 0, Run: func(context.Context) error { return nil }})
	if len(s.jobs) != 0 {
		t.Errorf("job with Interval<=0 should be skipped, got %d jobs", len(s.jobs))
	}
}

func TestScheduler_RunOnceReportsError(t *testing.T) {
	rep := &captureReporter{}
	s := New(testLogger(), rep)
	s.runOnce(context.Background(), Job{Name: "boom", Run: func(context.Context) error {
		return errors.New("nope")
	}})
	if rep.count() != 1 {
		t.Errorf("errored job should report 1 event, got %d", rep.count())
	}
}

func TestScheduler_RunOnceRecoversPanic(t *testing.T) {
	rep := &captureReporter{}
	s := New(testLogger(), rep)
	s.runOnce(context.Background(), Job{Name: "panics", Run: func(context.Context) error {
		panic("kaboom")
	}})
	if rep.count() != 1 {
		t.Errorf("panicking job should report 1 event, got %d", rep.count())
	}
	if rep.events[0].Stack == "" {
		t.Error("panic event should carry a stack trace")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./scheduler/ -v`
Expected: FAIL — `undefined: New` / `Job`.

- [ ] **Step 3: Write the implementation**

Create `scheduler/scheduler.go`:

```go
// Package scheduler runs registered jobs on fixed intervals in the background,
// each isolated by panic recovery and error reporting, until a context is
// cancelled.
package scheduler

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"app/report"
)

// Job is a unit of periodic work. A Job with Interval <= 0 is never scheduled.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Scheduler owns a set of jobs and runs each on its own goroutine.
type Scheduler struct {
	logger   *slog.Logger
	reporter report.Reporter
	jobs     []Job
}

// New returns a Scheduler that logs via logger and reports failures via reporter.
func New(logger *slog.Logger, reporter report.Reporter) *Scheduler {
	return &Scheduler{logger: logger, reporter: reporter}
}

// Add registers job. Jobs with Interval <= 0 are skipped (disabled by config).
func (s *Scheduler) Add(job Job) {
	if job.Interval <= 0 {
		s.logger.Info("scheduler: job disabled", slog.String("job", job.Name))
		return
	}
	s.jobs = append(s.jobs, job)
}

// Start launches one goroutine per job. Each runs on a ticker until ctx is
// cancelled. Jobs first run after one interval (not immediately).
func (s *Scheduler) Start(ctx context.Context) {
	for _, job := range s.jobs {
		go s.runLoop(ctx, job)
	}
}

func (s *Scheduler) runLoop(ctx context.Context, job Job) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx, job)
		}
	}
}

// runOnce executes job once, recovering panics and reporting any failure.
func (s *Scheduler) runOnce(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("scheduler: job panic", slog.String("job", job.Name))
			s.reporter.Report(ctx, report.Event{
				Message: "scheduler job panic: " + job.Name,
				Stack:   string(debug.Stack()),
			})
		}
	}()
	if err := job.Run(ctx); err != nil {
		s.logger.Error("scheduler: job failed",
			slog.String("job", job.Name), slog.String("err", err.Error()))
		s.reporter.Report(ctx, report.Event{
			Message: "scheduler job failed: " + job.Name,
			Err:     err.Error(),
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./scheduler/ -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add scheduler/scheduler.go scheduler/scheduler_test.go
git commit -m "feat: add background scheduler for interval jobs"
```

---

### Task 3: config — cleanup intervals, webhook URL, `durationFromEnv`

**Files:**
- Modify: `config/config.go` (add `time` import, three `Config` fields, parsing in `New`, one helper)
- Test: `config/env_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces (new `Config` fields): `SessionCleanupInterval time.Duration`, `TokenCleanupInterval time.Duration`, `ErrorWebhookURL string`; helper `durationFromEnv(key string, def time.Duration) time.Duration`.

- [ ] **Step 1: Write the failing test**

Create `config/env_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestDurationFromEnv(t *testing.T) {
	t.Run("unset uses default", func(t *testing.T) {
		if got := durationFromEnv("RIO_TEST_DUR", 2*time.Hour); got != 2*time.Hour {
			t.Errorf("got %s, want 2h", got)
		}
	})
	t.Run("valid parses", func(t *testing.T) {
		t.Setenv("RIO_TEST_DUR", "30m")
		if got := durationFromEnv("RIO_TEST_DUR", time.Hour); got != 30*time.Minute {
			t.Errorf("got %s, want 30m", got)
		}
	})
	t.Run("invalid uses default", func(t *testing.T) {
		t.Setenv("RIO_TEST_DUR", "not-a-duration")
		if got := durationFromEnv("RIO_TEST_DUR", time.Hour); got != time.Hour {
			t.Errorf("got %s, want 1h", got)
		}
	})
}

func TestNew_SetsOpsDefaults(t *testing.T) {
	c := New("debug", "hash")
	if c.SessionCleanupInterval != time.Hour {
		t.Errorf("SessionCleanupInterval = %s, want 1h", c.SessionCleanupInterval)
	}
	if c.TokenCleanupInterval != time.Hour {
		t.Errorf("TokenCleanupInterval = %s, want 1h", c.TokenCleanupInterval)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run 'TestDurationFromEnv|TestNew_SetsOpsDefaults' -v`
Expected: FAIL — `undefined: durationFromEnv` and unknown field `SessionCleanupInterval`.

- [ ] **Step 3: Add the `time` import**

In `config/config.go`, add `"time"` to the import block (alongside `log`, `os`, `path/filepath`, `strings`).

- [ ] **Step 4: Add the three Config fields**

In `config/config.go`, inside the `Config` struct, after the `Products []Product` field, add:

```go
	// Operational
	SessionCleanupInterval time.Duration
	TokenCleanupInterval   time.Duration
	ErrorWebhookURL        string
```

- [ ] **Step 5: Parse them in `New` and add the helper**

In `config/config.go`, in `New`, immediately before `return c`, add:

```go
	c.SessionCleanupInterval = durationFromEnv("SESSION_CLEANUP_INTERVAL", time.Hour)
	c.TokenCleanupInterval = durationFromEnv("TOKEN_CLEANUP_INTERVAL", time.Hour)
	c.ErrorWebhookURL = os.Getenv("ERROR_WEBHOOK_URL")
```

Then add this helper near the other `*FromEnv` helpers:

```go
// durationFromEnv parses key as a Go duration (e.g. "1h", "30m"), falling back
// to def when unset or invalid (logging a warning in the invalid case).
func durationFromEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("WARNING: invalid %s=%q; using %s", key, v, def)
		return def
	}
	return d
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./config/ -v`
Expected: PASS (new tests plus existing config tests).

- [ ] **Step 7: Commit**

```bash
git add config/config.go config/env_test.go
git commit -m "feat: config for cleanup intervals and error webhook"
```

---

### Task 4: app middleware — RequestID, LogRequests, RecoverAndReport

**Files:**
- Create: `middleware.go` (package `main`)
- Test: `middleware_test.go` (package `main`)

**Interfaces:**
- Consumes: `report.ContextWithRequestID`, `report.RequestIDFromContext`, `report.Reporter`, `report.Event` (Task 1); `rio.Http500`.
- Produces:
  - `func RequestID(next http.Handler) http.Handler`
  - `func LogRequests(logger *slog.Logger) func(http.Handler) http.Handler`
  - `func RecoverAndReport(logger *slog.Logger, reporter report.Reporter) func(http.Handler) http.Handler`

- [ ] **Step 1: Write the failing tests**

Create `middleware_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run 'TestRequestID|TestRecoverAndReport|TestLogRequests' -v`
Expected: FAIL — `undefined: RequestID` etc.

- [ ] **Step 3: Write the implementation**

Create `middleware.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test . -run 'TestRequestID|TestRecoverAndReport|TestLogRequests' -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add middleware.go middleware_test.go
git commit -m "feat: request-id, logging, and panic-reporting middleware"
```

---

### Task 5: wire the reporter, middleware, and scheduler into `main.run()`

**Files:**
- Modify: `main.go` (imports; build logger + reporter; explicit server middleware list; construct + start scheduler)

**Interfaces:**
- Consumes: `report.Nop`, `report.NewWebhook`, `report.Reporter` (Task 1); `scheduler.New`, `scheduler.Job` (Task 2); `RequestID`, `LogRequests`, `RecoverAndReport` (Task 4); `Conf.SessionCleanupInterval`, `Conf.TokenCleanupInterval`, `Conf.ErrorWebhookURL` (Task 3); existing `store.DeleteExpiredSessions`, `store.DeleteExpiredTokens` (both `func(context.Context) error`).
- Produces: no new exported symbols.

- [ ] **Step 1: Add imports**

In `main.go`, add to the import block: `"app/report"` and `"app/scheduler"` (with the other `app/*` imports).

- [ ] **Step 2: Build the logger and reporter**

In `main.go` `run()`, after the `APP_SECRET` production check and before `store := database.NewStore(db)`, add:

```go
	logger := rio.NewLogger(os.Stdout)
	rio.Logger(logger) // rio.MakeHandler's LogError uses this logger

	var reporter report.Reporter = report.Nop{}
	if Conf.ErrorWebhookURL != "" {
		reporter = report.NewWebhook(Conf.ErrorWebhookURL)
	}
```

- [ ] **Step 3: Replace the server construction with an explicit middleware list**

In `main.go` `run()`, change:

```go
	s := rio.NewServer()
	s.Use(auth.LoadUser(store)) // server-wide: populate the current user
```

to:

```go
	s := rio.NewServer(
		RequestID,
		LogRequests(logger),
		RecoverAndReport(logger, reporter),
		rio.SecureHeaders,
	)
	s.Use(auth.LoadUser(store)) // server-wide: populate the current user
```

- [ ] **Step 4: Construct and start the scheduler**

In `main.go` `run()`, after the line `defer stop()` (which follows `ctx, stop := signal.NotifyContext(...)`) and before `ln, err := net.Listen(...)`, add:

```go
	sched := scheduler.New(logger, reporter)
	sched.Add(scheduler.Job{Name: "sessions-cleanup", Interval: Conf.SessionCleanupInterval, Run: store.DeleteExpiredSessions})
	sched.Add(scheduler.Job{Name: "tokens-cleanup", Interval: Conf.TokenCleanupInterval, Run: store.DeleteExpiredTokens})
	sched.Start(ctx)
```

- [ ] **Step 5: Build and run the full suite**

Run: `go vet ./... && go build ./... && go test ./...`
Expected: all pass, no vet complaints.

- [ ] **Step 6: Smoke-test request IDs on a live server**

Run:
```bash
go build -o /tmp/rio-ophard . && DB_DIR=/tmp PORT=3013 APP_SECRET=dev /tmp/rio-ophard &
sleep 2
curl -sI http://localhost:3013/healthz | grep -i "x-request-id"
kill %1; rm -f /tmp/rio-ophard /tmp/riostarter.db*
```
Expected: one `X-Request-Id:` header line printed (proves the middleware chain is wired).

- [ ] **Step 7: Commit**

```bash
git add main.go
git commit -m "feat: wire error reporter, request-id/logging/recover middleware, and cleanup scheduler"
```

---

### Task 6: `.env.example`, `.gitignore`, and Litestream backup docs

**Files:**
- Create: `.env.example`
- Modify: `.gitignore` (add `.env`)
- Create: `docs/deploy/litestream.md`
- Create: `litestream.yml`
- Create: `docker-compose.yml`
- Modify: `README.md` (add Backups + Ops env rows)
- Test: `env_example_test.go` (package `main`)

**Interfaces:**
- Consumes: nothing (docs/config).
- Produces: no Go symbols; a test guarding `.env.example` completeness.

- [ ] **Step 1: Write the failing test**

Create `env_example_test.go`:

```go
package main

import (
	"os"
	"strings"
	"testing"
)

func TestEnvExample_ListsAllKeys(t *testing.T) {
	data, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	content := string(data)
	keys := []string{
		"APP_SECRET", "ADDR", "PORT", "BASE_URL", "DB_DIR", "TRUST_PROXY",
		"POSTMARK_TOKEN", "EMAIL_FROM",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_PRO", "STRIPE_PRICE_EBOOK",
		"SESSION_CLEANUP_INTERVAL", "TOKEN_CLEANUP_INTERVAL", "ERROR_WEBHOOK_URL",
	}
	for _, k := range keys {
		if !strings.Contains(content, k) {
			t.Errorf(".env.example missing %s", k)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestEnvExample -v`
Expected: FAIL — `read .env.example: open .env.example: no such file or directory`.

- [ ] **Step 3: Create `.env.example`**

Create `.env.example`:

```bash
# Copy to .env and fill in as needed. All values are optional in dev except
# where noted; unset optional features stay disabled.

# ----- Core -----
# APP_SECRET is REQUIRED in production (app refuses to start without it).
APP_SECRET=
# Listen address. ADDR (host:port) wins; else PORT (":<port>"); else :3000.
ADDR=
PORT=3000
# Absolute base URL for building magic links (defaults to http://localhost:<port>).
BASE_URL=
# SQLite directory. Defaults to /data in prod, the working dir in dev.
DB_DIR=
# Honor X-Forwarded-For (set only behind a trusted proxy): 1/true/yes.
TRUST_PROXY=

# ----- Email (magic-link login) -----
# Unset POSTMARK_TOKEN logs magic links to the console instead of sending.
POSTMARK_TOKEN=
EMAIL_FROM=noreply@localhost

# ----- Google OAuth (optional; both required to enable) -----
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=

# ----- Stripe billing (optional) -----
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_PRICE_PRO=
STRIPE_PRICE_EBOOK=

# ----- Operations -----
# Background cleanup intervals (Go durations, e.g. 1h, 30m). Set 0 to disable.
SESSION_CLEANUP_INTERVAL=1h
TOKEN_CLEANUP_INTERVAL=1h
# Optional error-reporting webhook (Sentry/Slack/any JSON collector). Unset = disabled.
ERROR_WEBHOOK_URL=
```

- [ ] **Step 4: Ignore real `.env`**

In `.gitignore`, under the `# Directories` section (or a new `# Local env` section), add a line:

```
# Local env (keep .env.example tracked)
.env
```

- [ ] **Step 5: Create the Litestream example config**

Create `litestream.yml`:

```yaml
# Example Litestream config. Replicates the app's SQLite database to S3-compatible
# storage for continuous backup + point-in-time restore. Litestream runs as a
# SEPARATE process/container (it cannot run inside the app's scratch image).
#
# Docs: https://litestream.io
dbs:
  - path: /data/riostarter.db   # match <DB_DIR>/<ProjectName>.db
    replicas:
      - type: s3
        bucket: your-backup-bucket
        path: riostarter
        region: us-east-1
        # Credentials via env: LITESTREAM_ACCESS_KEY_ID / LITESTREAM_SECRET_ACCESS_KEY
```

- [ ] **Step 6: Create the compose example**

Create `docker-compose.yml`:

```yaml
# Example deployment: the app plus a Litestream sidecar sharing the /data volume.
# Litestream cannot run inside the scratch app image, so it runs as its own
# container replicating the same SQLite file.
services:
  app:
    build:
      context: .
      args:
        BUILD_HASH: dev
    environment:
      APP_SECRET: change-me-in-prod
      DB_DIR: /data
    volumes:
      - data:/data
    ports:
      - "3000:3000"

  litestream:
    image: litestream/litestream:latest
    depends_on:
      - app
    command: ["replicate"]
    environment:
      LITESTREAM_ACCESS_KEY_ID: your-access-key
      LITESTREAM_SECRET_ACCESS_KEY: your-secret-key
    volumes:
      - data:/data
      - ./litestream.yml:/etc/litestream.yml:ro

volumes:
  data:
```

- [ ] **Step 7: Create the deploy doc**

Create `docs/deploy/litestream.md`:

```markdown
# Backups with Litestream

This template stores data in a single SQLite file (`<DB_DIR>/<ProjectName>.db`).
The backup story is [Litestream](https://litestream.io): it continuously
replicates the database's WAL to S3-compatible storage, giving offsite backups
with point-in-time restore and seconds-level RPO.

Litestream runs as a **separate process/container** — it cannot live inside the
app's `scratch` image. See `litestream.yml` and `docker-compose.yml` at the repo
root for a working sidecar example that shares the `/data` volume with the app.

## Setup

1. Create an S3 (or compatible: R2, MinIO, B2) bucket.
2. Edit `litestream.yml`: set `path` to `<DB_DIR>/<ProjectName>.db`, and fill in
   the bucket, path, and region.
3. Provide credentials via `LITESTREAM_ACCESS_KEY_ID` and
   `LITESTREAM_SECRET_ACCESS_KEY` (see `docker-compose.yml`).
4. Start both services: `docker compose up -d`.

## Restore

On a fresh volume, restore the database **before** the app starts (the app runs
migrations on an existing file, so restore first):

    litestream restore -config litestream.yml /data/riostarter.db

Then start the app. The compose file documents this restore-on-boot pattern in a
comment.

## Why not a built-in snapshot job?

A scheduled in-process `VACUUM INTO` would write to the same volume as the
database, so it would not survive the volume/host loss that is the real disaster,
and it would duplicate what Litestream already does better. Litestream is the
single backup story.
```

- [ ] **Step 8: Update the README**

In `README.md`, add a `## Backups` section (after the `## Build & deploy` section) with:

```markdown
## Backups

Data lives in a single SQLite file. Use [Litestream](https://litestream.io) for
continuous, offsite backups with point-in-time restore — see
[docs/deploy/litestream.md](docs/deploy/litestream.md) plus the root
`litestream.yml` and `docker-compose.yml` examples.

Expired sessions and login tokens are pruned automatically by a background
scheduler (intervals via `SESSION_CLEANUP_INTERVAL` / `TOKEN_CLEANUP_INTERVAL`;
set `0` to disable).
```

Also add these rows to the environment tables (in the auth section table, append the ops vars):

```markdown
| `SESSION_CLEANUP_INTERVAL` | Interval to prune expired sessions (`0` disables) | `1h` |
| `TOKEN_CLEANUP_INTERVAL` | Interval to prune expired login tokens (`0` disables) | `1h` |
| `ERROR_WEBHOOK_URL` | POST JSON error events here (Sentry/Slack/any); unset disables | unset |
```

Also mention `.env.example` in the Quick start: add a bullet "Copy `.env.example`
to `.env` and fill in values as needed."

- [ ] **Step 9: Run the test and full suite**

Run: `go test . -run TestEnvExample -v && go test ./...`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add .env.example .gitignore litestream.yml docker-compose.yml docs/deploy/litestream.md README.md env_example_test.go
git commit -m "docs: env template, .env ignore, and Litestream backup docs"
```

---

## Self-Review

**Spec coverage:**
- Component 1 scheduler (Job, New, Add skip-disabled, Start, panic recovery, error reporting) → Task 2; config intervals → Task 3; wired in `main.run()` with sessions/tokens jobs → Task 5. ✓
- Component 2 Litestream docs (litestream.md, litestream.yml, docker-compose.yml, restore instructions, "cannot run in scratch") → Task 6; no `database` backup code → confirmed (no such task). ✓
- Component 3 reporter (Event, Reporter, Nop, Webhook, Capture) → Task 1; request IDs + RecoverAndReport + LogRequests + explicit server middleware list + reporter selection → Tasks 4 & 5. ✓
- Component 4 `.env.example` (all keys) + `.env` gitignore → Task 6. ✓
- Global constraints: zero new deps (crypto/rand, net/http only), env-gated (Nop default, Interval<=0 skip), scratch preserved (no code changes to Dockerfile runtime), shutdown ctx stops scheduler → Task 5 Step 4. ✓
- Testing per spec: scheduler run/skip/panic → Task 2; webhook JSON / nop / capture → Task 1; RequestID header+context / RecoverAndReport 500+event+stack / LogRequests → Task 4. ✓

**Placeholder scan:** No TBD/TODO/"similar to"/"add error handling" — every code step is complete. ✓

**Type consistency:** `report.Reporter`, `report.Event` (fields Message/Err/Stack/RequestID/Method/URL), `report.NewWebhook`, `report.Nop`, `report.ContextWithRequestID`, `report.RequestIDFromContext`, `report.Capture`, `scheduler.New(logger, reporter)`, `scheduler.Job{Name,Interval,Run}`, `RequestID`/`LogRequests(logger)`/`RecoverAndReport(logger, reporter)`, and `Conf.SessionCleanupInterval`/`TokenCleanupInterval`/`ErrorWebhookURL` are referenced identically across Tasks 1–6. `store.DeleteExpiredSessions`/`DeleteExpiredTokens` match the real `func(context.Context) error` signatures. ✓
