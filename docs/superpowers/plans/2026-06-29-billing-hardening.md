# Billing Hardening + Test Backfill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Stripe webhook idempotent against replayed/duplicate events via a processed-events table, and backfill the three coverage gaps the #4 review flagged.

**Architecture:** A new `processed_webhook_events` table records each handled Stripe event id. The webhook does **check → apply → record**: skip if the id is already recorded, apply the event, then record the id (so a failed apply is retried, not lost). Plus a trivial `ListEntitlements` zero-value tweak and three test additions for already-correct paths.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio` v0.26.0, `modernc.org/sqlite`, `github.com/stripe/stripe-go/v82`. **No new dependency, no new config/env, no UI change.**

## Global Constraints

- Module path `app`; internal imports `app/config`, `app/database`, `app/auth`, `app/billing`, `app/views`.
- Raw SQL over `database/sql`; methods hang off the existing `*database.Store`. Migrations forward-only, embedded; add `database/migrations/0005_webhook_events.sql` (numbers cleanly after `0004`).
- **Webhook ordering is check → apply → record.** Check `IsEventProcessed` before the event-type switch (skip with 200 if already processed); call `RecordEvent` only AFTER the apply switch succeeds. Never record before applying — a failed apply must be retryable by Stripe, not skipped.
- On any store error in the webhook (`IsEventProcessed`, an apply step, or `RecordEvent`) → `rio.LogError(err)` and `return err` (yields 500 so Stripe retries). Signature-verify failure stays 400.
- Dedup is keyed on the Stripe event id (`billing.Event.ID`, from `ev.ID`); it drops true replays. Out-of-order distinct events are NOT reordered (declined monotonic guard, out of scope).
- `ListEntitlements` returns `[]string{}` (non-nil) for an empty result.
- Only `billing/` imports stripe-go.
- Run all tests with `go test ./...` from the repo root. TDD: failing test first (the Task 4 backfill tests assert already-correct behavior, so they pass on first run — see that task's note). Commit after each task.

---

### Task 1: Processed-events store + migration + ListEntitlements tweak

**Files:**
- Create: `database/migrations/0005_webhook_events.sql`, `database/webhook_events.go`
- Modify: `database/entitlements.go`
- Test: `database/webhook_events_test.go` (new), `database/entitlements_test.go` (append)

**Interfaces:**
- Consumes: existing `Store`, `newTestStore`, `CreateUser`.
- Produces:
  - `func (s *Store) IsEventProcessed(ctx context.Context, eventID string) (bool, error)`
  - `func (s *Store) RecordEvent(ctx context.Context, eventID string) error` (idempotent)
  - `ListEntitlements` now returns a non-nil empty slice when there are no entitlements.

- [ ] **Step 1: Write the migration**

```sql
-- database/migrations/0005_webhook_events.sql
CREATE TABLE processed_webhook_events (
    event_id   TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 2: Write the failing tests**

```go
// database/webhook_events_test.go
package database

import (
	"context"
	"testing"
)

func TestProcessedWebhookEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if done, err := s.IsEventProcessed(ctx, "evt_1"); err != nil || done {
		t.Fatalf("before record: done=%v err=%v, want false/nil", done, err)
	}
	if err := s.RecordEvent(ctx, "evt_1"); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	if done, _ := s.IsEventProcessed(ctx, "evt_1"); !done {
		t.Error("event not marked processed after RecordEvent")
	}
	// RecordEvent is idempotent: re-recording the same id is a no-op, not an error.
	if err := s.RecordEvent(ctx, "evt_1"); err != nil {
		t.Fatalf("re-record: %v", err)
	}
	// Distinct ids are independent.
	if done, _ := s.IsEventProcessed(ctx, "evt_2"); done {
		t.Error("distinct id should be unprocessed")
	}
}
```

```go
// database/entitlements_test.go  (append)

func TestListEntitlements_EmptyIsNonNil(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "z@example.com", "Z")

	list, err := s.ListEntitlements(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListEntitlements: %v", err)
	}
	if list == nil {
		t.Error("ListEntitlements returned nil, want non-nil empty slice")
	}
	if len(list) != 0 {
		t.Errorf("len = %d, want 0", len(list))
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./database/ -run 'TestProcessedWebhookEvents|TestListEntitlements_EmptyIsNonNil' -v`
Expected: FAIL — `undefined: (*Store).IsEventProcessed`; the empty-list test fails because `ListEntitlements` returns `nil`.

- [ ] **Step 4: Write the implementation**

```go
// database/webhook_events.go
package database

import "context"

// IsEventProcessed reports whether a Stripe webhook event id has already been
// recorded as processed.
func (s *Store) IsEventProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM processed_webhook_events WHERE event_id = ?)",
		eventID).Scan(&exists)
	return exists, err
}

// RecordEvent marks a Stripe webhook event id as processed. Idempotent: a
// repeated record is a no-op (the primary key).
func (s *Store) RecordEvent(ctx context.Context, eventID string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO processed_webhook_events (event_id) VALUES (?) ON CONFLICT(event_id) DO NOTHING",
		eventID)
	return err
}
```

In `database/entitlements.go`, change `ListEntitlements` to initialize a non-nil slice. Replace the line `var out []string` with:

```go
	out := []string{}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./database/ -v`
Expected: PASS (all database tests).

- [ ] **Step 6: Commit**

```bash
git add database/migrations/0005_webhook_events.sql database/webhook_events.go database/webhook_events_test.go database/entitlements.go database/entitlements_test.go
git commit -m "feat(db): processed-events store for webhook dedup; ListEntitlements non-nil empty"
```

---

### Task 2: `billing.Event.ID` from the Stripe event id

**Files:**
- Modify: `billing/billing.go`
- Test: `billing/billing_test.go` (modify)

**Interfaces:**
- Consumes: stripe-go `webhook.ConstructEventWithOptions` (already used).
- Produces: `billing.Event` gains `ID string`, populated from the Stripe event's id in `VerifyWebhook`.

- [ ] **Step 1: Add a failing assertion to the existing webhook test**

In `billing/billing_test.go`, inside `TestVerifyWebhook_GoodAndBadSignature`, after the good-signature event is parsed (the block that already asserts `ev.Type`/`ev.CustomerID`/`ev.Status`), add an id assertion (the test payload's event id is `evt_1`):

```go
	if ev.ID != "evt_1" {
		t.Errorf("event ID = %q, want evt_1", ev.ID)
	}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./billing/ -run TestVerifyWebhook_GoodAndBadSignature -v`
Expected: FAIL to compile — `ev.ID undefined (type Event has no field or method ID)`.

- [ ] **Step 3: Write the implementation**

In `billing/billing.go`, add the field to `Event`:

```go
type Event struct {
	ID               string // Stripe event id (for idempotency/dedup)
	Type             string
	CustomerID       string
	UserID           string // from checkout session metadata.user_id
	Status           string // subscription status
	CurrentPeriodEnd time.Time
	ProductKey       string // from checkout session metadata.product_key
}
```

In `VerifyWebhook`, set the id when building the normalized event. Change:

```go
	out := Event{Type: string(ev.Type)}
```
to:

```go
	out := Event{ID: ev.ID, Type: string(ev.Type)}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./billing/ -v`
Expected: PASS (all billing tests).

- [ ] **Step 5: Commit**

```bash
git add billing/billing.go billing/billing_test.go
git commit -m "feat(billing): expose Stripe event id on the normalized Event"
```

---

### Task 3: Webhook dedup (check → apply → record) + tests

**Files:**
- Modify: `handlers_billing.go`
- Test: `handlers_billing_test.go` (append)

**Interfaces:**
- Consumes: `store.IsEventProcessed`/`RecordEvent` (Task 1), `billing.Event.ID` (Task 2), existing `billing.FakeClient` (`NextEvent`), `Conf.ProductByKey`, `config.OneTime`.
- Produces: `HandleStripeWebhook` skips an already-processed event id (200) and records the id only after a successful apply.

- [ ] **Step 1: Write the failing tests**

```go
// handlers_billing_test.go  (append)
// Add imports "path/filepath" and "app/database" to this file's import block.

// A duplicate (already-recorded) event id is skipped without re-applying.
// Proof: the event targets a non-existent user, so if the handler attempted the
// grant it would hit a foreign-key error and return 500. A 200 proves it skipped.
func TestHandleStripeWebhook_SkipsAlreadyProcessed(t *testing.T) {
	store := authTestStore(t)
	if err := store.RecordEvent(context.Background(), "evt_dup"); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	fake := &billing.FakeClient{NextEvent: billing.Event{
		ID: "evt_dup", Type: "checkout.session.completed",
		ProductKey: "ebook", UserID: "999999", // user does not exist
	}}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	HandleStripeWebhook(store, fake).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (skipped, no apply attempted)", rec.Code)
	}
}

// A fresh event is applied, then its id is recorded.
func TestHandleStripeWebhook_RecordsAfterApply(t *testing.T) {
	store := authTestStore(t)
	u, _ := store.CreateUser(context.Background(), "wid@example.com", "W")
	fake := &billing.FakeClient{NextEvent: billing.Event{
		ID: "evt_new", Type: "checkout.session.completed",
		ProductKey: "ebook", UserID: itoaTest(u.ID),
	}}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	HandleStripeWebhook(store, fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if has, _ := store.HasEntitlement(context.Background(), u.ID, "ebook"); !has {
		t.Error("entitlement not granted on first delivery")
	}
	if done, _ := store.IsEventProcessed(context.Background(), "evt_new"); !done {
		t.Error("event id not recorded after a successful apply")
	}
}

// A store failure (closed DB) makes the handler return 500 so Stripe retries.
func TestHandleStripeWebhook_StoreError500(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "we.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MigrateUp(db); err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(db)
	db.Close() // force every store call to fail

	fake := &billing.FakeClient{NextEvent: billing.Event{
		ID: "evt_err", Type: "checkout.session.completed", ProductKey: "ebook", UserID: "1",
	}}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	HandleStripeWebhook(store, fake).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run 'TestHandleStripeWebhook_SkipsAlreadyProcessed|TestHandleStripeWebhook_RecordsAfterApply|TestHandleStripeWebhook_StoreError500' -v`
Expected: FAIL — without the dedup logic, `SkipsAlreadyProcessed` returns 500 (the grant for the non-existent user hits the FK error) instead of 200, and `RecordsAfterApply` fails the `IsEventProcessed` assertion (nothing records the id). (`StoreError500` already passes — it characterizes the existing 500-on-store-error behavior, which is unchanged; it's grouped here with the other webhook tests.)

- [ ] **Step 3: Add the dedup logic to `HandleStripeWebhook`**

In `handlers_billing.go`, in `HandleStripeWebhook`, insert the check immediately after the `VerifyWebhook` error block (before the `switch event.Type`):

```go
		// Idempotency: skip an event we've already fully processed. Stripe may
		// re-deliver an event even after a 200. Check before applying; record
		// only after a successful apply (so a failed apply is retried, not lost).
		if event.ID != "" {
			done, err := store.IsEventProcessed(r.Context(), event.ID)
			if err != nil {
				rio.LogError(err)
				return err
			}
			if done {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		}
```

Then, after the `switch event.Type { ... }` block and before the final `w.WriteHeader(http.StatusOK)`, add the record step:

```go
		if event.ID != "" {
			if err := store.RecordEvent(r.Context(), event.ID); err != nil {
				rio.LogError(err)
				return err
			}
		}
```

(The existing apply branches already `rio.LogError(err); return err` on store failure, so a failed apply returns 500 before the record step is reached.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test . -run TestHandleStripeWebhook -v`
Expected: PASS (the three new tests plus the existing webhook tests — the existing ones use events with no `ID`, so they bypass the dedup block and behave exactly as before).

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_billing.go handlers_billing_test.go
git commit -m "feat(billing): dedup Stripe webhooks via processed-events (check, apply, record)"
```

---

### Task 4: Backfill the remaining coverage gaps

**Files:**
- Test: `auth/gating_test.go` (append), `views/account_test.go` (append)

**Interfaces:**
- Consumes: `auth.RequireSubscription`, `views.Billing`, `views.BillingView`, `views.AccountView`, `testPageData`.
- Produces: tests only — no production code changes.

> **Note for the implementer:** these two tests cover behavior that is already
> correct, so they PASS on first run (no red phase). That is expected — the
> point is regression coverage. Each test must be *meaningful*: it would fail if
> the guarded behavior regressed (a non-subscriber being let through; a disabled
> billing tab emitting purchase forms). Do not add production code in this task.

- [ ] **Step 1: Write the test for `RequireSubscription` with no user**

```go
// auth/gating_test.go  (append)

func TestRequireSubscription_NoUser(t *testing.T) {
	guard := RequireSubscription(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not run when no user is in context")
	}))
	rec := httptest.NewRecorder()
	// A bare request with no user in context (LoadUser never ran).
	guard.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/premium", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account/billing" {
		t.Errorf("no-user: got %d %q, want 303 /account/billing", rec.Code, rec.Header().Get("Location"))
	}
}
```

- [ ] **Step 2: Write the test for the disabled-billing tab**

```go
// views/account_test.go  (append)

func TestBilling_NotConfigured(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "billing", CSRF: "c"}
	var b bytes.Buffer
	_ = Billing(pd, config.Meta{Title: "Billing"}, av, BillingView{StripeEnabled: false}).Render(&b)
	html := b.String()
	if !strings.Contains(html, "Billing is not configured") {
		t.Error("disabled billing should show the not-configured message")
	}
	if strings.Contains(html, `/account/billing/checkout`) || strings.Contains(html, `/account/billing/portal`) {
		t.Error("disabled billing must not render checkout/portal forms")
	}
}
```

- [ ] **Step 3: Run the tests**

Run: `go test ./auth/ -run TestRequireSubscription_NoUser -v` and `go test ./views/ -run TestBilling_NotConfigured -v`
Expected: PASS (both exercise existing, correct behavior). `auth/gating_test.go` already imports `net/http`, `net/http/httptest`, `testing`; `views/account_test.go` already imports `bytes`, `strings`, `testing`, `app/config`.

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: all packages PASS.

- [ ] **Step 5: Commit**

```bash
git add auth/gating_test.go views/account_test.go
git commit -m "test: backfill RequireSubscription no-user and disabled-billing tab cases"
```

---

## Self-Review

**Spec coverage:**
- Processed-events table + `IsEventProcessed`/`RecordEvent` → Task 1 ✓
- `billing.Event.ID` from the Stripe event id → Task 2 ✓
- Webhook check → apply → record (skip duplicates, record after apply, 500 on store error) → Task 3 ✓
- `ListEntitlements` non-nil empty → Task 1 ✓
- Test backfill: webhook store-error 500 + replay skip + record-after-apply → Task 3; `RequireSubscription` no-user + `!StripeEnabled` Billing fallback → Task 4 ✓
- No new dependency / config / UI → entire plan stays within existing packages ✓

**Placeholder scan:** every code step has complete code; every test has real assertions. The Task 4 note explains the intentional no-red-phase (coverage backfill of existing behavior).

**Type consistency:** `IsEventProcessed(ctx, string) (bool, error)`, `RecordEvent(ctx, string) error`, `billing.Event.ID`, `HandleStripeWebhook` dedup using both — consistent across Tasks 1→2→3. The dedup-skip test relies on the entitlements FK (a grant for a non-existent user errors) to prove the apply was skipped; the FK is enforced (`foreign_keys = ON`, set in `database.Open`).

**Notes for the implementer:**
- The migration is `0005_webhook_events.sql`; `main` already has `0001`–`0004`.
- The dedup block is guarded by `event.ID != ""`, so the existing webhook tests (which use events without an ID) are unaffected.
- Record happens only after the apply switch succeeds — never before — so a failed apply returns 500 and Stripe retries it.
