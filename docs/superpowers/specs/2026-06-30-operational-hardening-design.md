# Operational Hardening — Design

**Date:** 2026-06-30
**Status:** Approved (pending spec review)

## Goal

Close the four highest-value operational gaps that stand between this template
and running a real SaaS in production, without breaking its "single binary,
scratch image, optional env-gated features" philosophy:

1. **Background scheduler** — an in-process job runner.
2. **SQLite snapshots + Litestream docs** — a data durability story.
3. **Pluggable error reporter + request IDs** — production error visibility.
4. **`.env.example`** — a complete, committed config template.

Chosen approaches (from brainstorming):
- Backups: **built-in local snapshot** (always-on, zero deps) **plus documented
  Litestream** sidecar for offsite/point-in-time replication.
- Errors: **pluggable reporter interface** (Nop default + optional Webhook impl),
  env-gated like Google/Stripe, wired via app-level middleware; **plus per-request
  IDs**.

## Scope

In scope: the four items above. **Out of scope (Tier 2, deferred):** Content
Security Policy header, broader/global rate limiting, admin back-office,
transactional email breadth, teams/orgs, public API. Do not add these here.

## Global constraints

- **Zero new Go module dependencies.** Snapshots use SQL (`VACUUM INTO`), request
  IDs use `crypto/rand`, the webhook reporter uses `net/http`. Litestream is
  external (docs/compose only).
- Preserve the scratch single-binary deploy. Nothing may require CGO or a shell
  in the runtime image.
- New features are **optional and env-gated**, matching the existing Google/Stripe
  pattern: absent/zero config → feature disabled, app still runs.
- Follow existing idioms: `config.New` env parsing helpers (`cmpOr`, `isTruthy`),
  `database.Store` methods, `rio` middleware signatures, table-driven tests with
  the existing helpers.
- All new background work stops cleanly on the shutdown context already created in
  `main.run()` (`signal.NotifyContext`).

---

## Component 1: Background scheduler (`scheduler` package)

New package `scheduler` (new dir `scheduler/`).

```go
type Job struct {
    Name     string
    Interval time.Duration
    Run      func(ctx context.Context) error
}

type Scheduler struct { /* jobs []Job; logger; reporter */ }

func New(logger *slog.Logger, reporter report.Reporter) *Scheduler
func (s *Scheduler) Add(job Job)               // no-op guard: skip if Interval <= 0
func (s *Scheduler) Start(ctx context.Context) // launches one goroutine per job
```

- `Start` launches one goroutine per registered job. Each goroutine runs on a
  `time.NewTicker(job.Interval)`; on each tick it calls `job.Run(ctx)` inside a
  `recover()` guard so a panicking or erroring job never crashes the process.
- On error or recovered panic: log at error level (`job` name attribute) and
  `reporter.Report(ctx, ...)`.
- Each goroutine returns when `ctx.Done()` fires (shutdown), stopping its ticker.
- Jobs do **not** run immediately on `Start` (first run is after one interval),
  keeping startup fast and avoiding a snapshot on every deploy.
- `Add` with `Interval <= 0` is a silent no-op, so a job is disabled purely by
  configuring its interval to `0`.

**Wiring in `main.run()`:** after the store is built and before `serve(...)`,
construct the scheduler and register jobs, then `sched.Start(ctx)`:

```go
sched := scheduler.New(logger, reporter)
sched.Add(scheduler.Job{Name: "sessions-cleanup", Interval: Conf.SessionCleanupInterval,
    Run: func(ctx context.Context) error { return store.DeleteExpiredSessions(ctx) }})
sched.Add(scheduler.Job{Name: "tokens-cleanup", Interval: Conf.TokenCleanupInterval,
    Run: func(ctx context.Context) error { return store.DeleteExpiredTokens(ctx) }})
sched.Add(scheduler.Job{Name: "db-snapshot", Interval: Conf.BackupInterval,
    Run: func(ctx context.Context) error {
        return database.Snapshot(ctx, db, Conf.BackupDir, Conf.BackupRetain)
    }})
sched.Start(ctx)
```

**Config additions** (`config.Config` fields + `config.New` parsing), all with
env overrides and defaults:

| Field | Env | Default |
|-------|-----|---------|
| `SessionCleanupInterval` | `SESSION_CLEANUP_INTERVAL` | `1h` |
| `TokenCleanupInterval` | `TOKEN_CLEANUP_INTERVAL` | `1h` |
| `BackupInterval` | `BACKUP_INTERVAL` | `24h` |
| `BackupDir` | `BACKUP_DIR` | `<DB_DIR>/backups` |
| `BackupRetain` | `BACKUP_RETAIN` | `7` |

Durations are parsed with `time.ParseDuration`; on parse failure, log a warning
and fall back to the default. `BackupRetain` parsed with `strconv.Atoi`, same
fallback. A new helper `durationFromEnv(key string, def time.Duration)
time.Duration` and `intFromEnv(key string, def int) int` live in `config`
alongside the existing env helpers.

---

## Component 2: SQLite snapshots + Litestream docs

### `database.Snapshot`

Add to the `database` package (new file `database/backup.go`):

```go
// Snapshot writes a consistent copy of the database to destDir via
// VACUUM INTO, then prunes older snapshots so at most retain remain.
func Snapshot(ctx context.Context, db *sql.DB, destDir string, retain int) error
```

- `os.MkdirAll(destDir, 0o755)`.
- Build a filename `snapshot-<RFC3339-with-safe-chars>.db` (colons replaced so
  the name is filesystem-safe on all platforms).
- Execute `VACUUM INTO ?` bound to the full path. `VACUUM INTO` produces a
  transactionally consistent snapshot even with WAL active, and works on the
  cgo-free `modernc.org/sqlite` driver.
- Prune: list files matching `snapshot-*.db` in `destDir`, sort by name
  (RFC3339 sorts lexicographically = chronologically), delete all but the newest
  `retain`. If `retain <= 0`, keep all.
- Return wrapped errors (`fmt.Errorf("...: %w", err)`); never panic.

### Litestream docs (no code)

- `docs/deploy/litestream.md` — when/why to use Litestream on top of the built-in
  snapshots (continuous replication + point-in-time restore to S3-compatible
  storage), and restore instructions.
- `litestream.yml` (repo root, example) — a minimal config replicating
  `/data/<ProjectName>.db` to an S3 bucket, with placeholder credentials.
- `docker-compose.yml` (repo root, example) — two services sharing the `/data`
  volume: the app (scratch image) and a `litestream/litestream` sidecar running
  `litestream replicate`. Documents that Litestream cannot run inside the scratch
  image and therefore runs as a separate container.

---

## Component 3: Pluggable error reporter (`report` package) + request IDs

### `report` package (new dir `report/`)

```go
type Event struct {
    Message   string
    Err       error
    Stack     string
    RequestID string
    Method    string
    URL       string
}

type Reporter interface {
    Report(ctx context.Context, e Event)
}

// Nop is the default reporter; it does nothing (errors are still logged).
type Nop struct{}
func (Nop) Report(context.Context, Event) {}

// Webhook posts the event as JSON to a collector URL.
type Webhook struct { URL string; Client *http.Client }
func NewWebhook(url string) Webhook
func (w Webhook) Report(ctx context.Context, e Event) // POST JSON; best-effort

// Capture is a helper for deliberately reporting a non-panic error.
func Capture(ctx context.Context, r Reporter, err error)
```

- `Webhook.Report` marshals `Event` to JSON and POSTs it with a short timeout
  (e.g. 5s context). It is **best-effort**: on transport/marshal error it logs a
  warning and returns; reporting failures must never break request handling.
- `Capture` builds an `Event` from `err` + request context (via the request-ID
  helper below) and calls `r.Report`.

**Selection in `config`/`main`:** `ErrorWebhookURL` field, env `ERROR_WEBHOOK_URL`.
`main.run()` picks the reporter: `report.NewWebhook(url)` when set, else
`report.Nop{}`.

### Request IDs + recovery middleware (app package)

New file `middleware.go` in the `main` package (server middleware belongs with
the other `main`-package wiring, not in `views`). Contains:

```go
type ctxKey int
const requestIDKey ctxKey = 0

func RequestID(next http.Handler) http.Handler          // sets X-Request-ID + ctx
func RequestIDFromContext(ctx context.Context) string   // "" if absent

func LogRequests(logger *slog.Logger) func(http.Handler) http.Handler
func RecoverAndReport(logger *slog.Logger, reporter report.Reporter) func(http.Handler) http.Handler
```

- `RequestID`: generate 16 random bytes via `crypto/rand`, hex-encode, store in
  context, set the `X-Request-ID` response header. If the inbound request already
  has an `X-Request-ID`, honor it (trust boundary is the proxy; acceptable for a
  template — note this in a comment).
- `LogRequests`: same shape as rio's `LogRequest` but reads the request ID from
  context and includes it as a `request_id` attribute, and uses `r.Context()`.
  This replaces rio's default logger so request logs carry the ID.
- `RecoverAndReport`: replaces rio's default `RecoverPanic`. On a recovered
  panic: capture `debug.Stack()`, build a `report.Event` (message, stack, request
  ID, method, URL), call `reporter.Report`, log at error level, set
  `Connection: close`, and write `rio` `Http500`-equivalent (`http.Error(w, ...,
  500)`).

**Server construction change (`main.run()`):** build the server with an explicit
middleware list instead of the auto-registered defaults:

```go
logger := rio.NewLogger(os.Stdout)
rio.Logger(logger) // so MakeHandler's LogError uses the same logger
s := rio.NewServer(
    RequestID,
    LogRequests(logger),
    RecoverAndReport(logger, reporter),
    rio.SecureHeaders,
)
s.Use(auth.LoadUser(store))
```

Non-panic errors returned by handlers still flow through `rio.MakeHandler` →
`rio.LogError` (now our logger), so they are logged with context; deliberate
reporting of such errors uses `report.Capture`. (A future enhancement could tee
error-level slog records to the reporter, but that is out of scope.)

---

## Component 4: `.env.example`

Committed file at repo root. One commented line per env var the app reads, grouped
by concern, with dev-safe blank/placeholder values:

- Core: `APP_SECRET` (required in prod), `ADDR`/`PORT`, `BASE_URL`, `DB_DIR`,
  `TRUST_PROXY`.
- Email: `POSTMARK_TOKEN`, `EMAIL_FROM`.
- Google OAuth: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`.
- Stripe: `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_PRO`,
  `STRIPE_PRICE_EBOOK`.
- Ops (new): `SESSION_CLEANUP_INTERVAL`, `TOKEN_CLEANUP_INTERVAL`,
  `BACKUP_INTERVAL`, `BACKUP_DIR`, `BACKUP_RETAIN`, `ERROR_WEBHOOK_URL`.

`.env.example` is committed; ensure `.env` (real secrets) is git-ignored (add to
`.gitignore` if not already present).

---

## Testing

Follow the existing table-driven style and test helpers.

- **scheduler:** a registered job with a short interval runs at least once and
  stops when the context is cancelled (signal via a channel/counter); a job with
  `Interval <= 0` is never scheduled; a panicking job is recovered and does not
  stop the scheduler.
- **database.Snapshot:** against a temp DB in a temp dir — creates a snapshot
  file; a second call with `retain = 1` leaves exactly one file (older pruned);
  `retain <= 0` keeps all. Verify the snapshot file is a valid, openable SQLite
  database.
- **report.Webhook:** posts correct JSON to an `httptest.Server` (assert body
  fields); a transport error is swallowed (no panic). **report.Nop:** no-op.
  **report.Capture:** builds an event and calls the reporter.
- **RequestID middleware:** sets a non-empty `X-Request-ID` header and populates
  the context (`RequestIDFromContext` returns it); honors an inbound header.
- **RecoverAndReport middleware:** a handler that panics yields HTTP 500 and the
  reporter receives one event whose `Stack` is non-empty and `RequestID` matches
  the response header (compose with `RequestID` in the test).
- **LogRequests:** smoke test that it wraps and serves without error (status
  captured).

## Non-goals / YAGNI

- No immediate-on-start job execution (first run after one interval).
- No retry/backoff in the scheduler or webhook reporter (best-effort; log and
  move on).
- No Litestream Go code, embedding, or shelling out.
- No stack traces for non-panic handler errors (only panics carry stacks).
- No CSP, global rate limiting, admin panel, or email breadth (Tier 2).
