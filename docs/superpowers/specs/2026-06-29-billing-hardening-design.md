# Accounts Platform — Sub-project #5: Billing Hardening + Test Backfill — Design

**Date:** 2026-06-29
**Status:** Approved, ready for implementation planning
**Branch:** `billing-hardening` (off `main`)
**Target repo:** `github.com/TunedMystic/rio-project-template`
**Depends on:** Sub-projects #1 (identity) and #4 (Stripe billing), both merged to `main`.

## Context

Sub-projects #1, #3, and #4 are merged. The final review of #4 surfaced two
worthwhile follow-ups that were deferred as non-blocking:

- **M2 — webhook replay guard.** The webhook's store operations are naturally
  idempotent, but there is no explicit dedup, so a replayed *stale*
  `customer.subscription.updated` could overwrite newer subscription state.
- **Test-coverage gaps.** Three already-correct paths lack tests: the
  `RequireSubscription` no-user case, the `!StripeEnabled` Billing-tab
  fallback, and the webhook store-error → 500 path.

This sub-project hardens the webhook against duplicate/replayed delivery and
backfills those tests. It is a small, cohesive bundle — one spec, one plan. No
new dependency.

## Decisions (locked during brainstorming)

| Decision | Choice | Rationale |
|---|---|---|
| Replay guard | **Processed-events table** | Record each Stripe event id after handling; INSERT-or-ignore and skip if already seen. Stripe's recommended idempotency pattern — general dedup for ALL event types, not just subscriptions. |
| Ordering | **Covered by dedup, no separate monotonic guard** | Dedup-by-id drops the realistic replay case. A strict out-of-order guard (never move `current_period_end` backward) is additional complexity a starter template does not need (YAGNI). |
| Row pruning | **Out of scope** | Processed-event rows are tiny; a deployment can prune later. Consistent with the existing `DeleteExpired*` helpers that exist but aren't cron-wired. |
| `ListEntitlements` empty value | **Return `[]string{}` (not `nil`)** | Friendlier zero value for callers; cheap to fix while we're here. |

## Out of scope

- Processed-event row pruning / a cron job.
- A monotonic ordering guard on subscription updates.
- Any new product, UI, or billing behavior.

## Architecture & changes

Dependency direction unchanged. The webhook gains a dedup step backed by a new
table; everything else is test additions and one trivial store tweak.

- **`database/`**
  - `migrations/0005_webhook_events.sql` (new) — the processed-events table.
  - `webhook_events.go` (new) — `MarkEventProcessed`.
  - `entitlements.go` (modify) — `ListEntitlements` returns `[]string{}` for an
    empty result.
- **`billing/`**
  - `billing.go` (modify) — `Event` gains `ID string`, set from the Stripe
    event id in `VerifyWebhook`.
- **Root handlers**
  - `handlers_billing.go` (modify) — `HandleStripeWebhook` dedups via
    `MarkEventProcessed` before applying the event.
- **Tests** — `database/webhook_events_test.go` (new); appends to
  `auth/gating_test.go`, `views/account_test.go`, `handlers_billing_test.go`,
  and the existing `database/entitlements_test.go` for the empty-list value.

## Data model — `database/migrations/0005_webhook_events.sql`

```sql
CREATE TABLE processed_webhook_events (
    event_id   TEXT PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Numbered `0005` to slot after #4's `0004_billing.sql` (the runner applies any
unapplied migration in filename order).

New store methods (`database/webhook_events.go`):

- `IsEventProcessed(ctx, eventID string) (bool, error)` — `SELECT EXISTS(...)`
  on `processed_webhook_events`. True when the id has already been recorded.
- `RecordEvent(ctx, eventID string) error` —
  `INSERT INTO processed_webhook_events (event_id) VALUES (?) ON CONFLICT(event_id) DO NOTHING`
  (idempotent: re-recording the same id is a no-op, not an error).

## Webhook dedup flow

`billing.Event` gains `ID` (from `ev.ID`). `HandleStripeWebhook` checks **before**
applying and records **after** a successful apply — never the other way around,
so a failed apply is correctly retried rather than skipped:

```text
// after VerifyWebhook succeeds, before the event-type switch:
if event.ID != "" {
    done, err := store.IsEventProcessed(ctx, event.ID)
    if err != nil { rio.LogError(err); return err }   // 500 → Stripe retries
    if done { return 200 }                             // already handled; skip
}

... existing switch (grant entitlement / link customer / update subscription);
    any apply step that errors → rio.LogError + return err (500) ...

// only after the apply succeeded:
if event.ID != "" {
    if err := store.RecordEvent(ctx, event.ID); err != nil { rio.LogError(err); return err } // 500
}
return 200
```

Properties:
- **Check-then-apply-then-record.** A duplicate delivery of the same event id
  is dropped at the top before any state change.
- A genuine first delivery is applied exactly as today, then recorded.
- **A failed apply is retried, not lost.** If an apply step errors, the handler
  returns 500 *before* recording the id, so Stripe's retry re-runs the apply
  (idempotently). The id is recorded only once the apply succeeded.
- If `RecordEvent` itself fails after a successful apply, the handler returns
  500; Stripe retries, the apply re-runs idempotently, and the record is
  retried. No event is silently lost.
- **Scope note:** dedup is keyed on the event id, so it drops true *replays*
  (Stripe re-delivering the same event). Two *distinct* events arriving out of
  order (different ids) are not reordered — that is the declined monotonic
  guard and is out of scope.

## Test plan

Automated (Go tests, no network):

- `database/webhook_events_test.go` — `IsEventProcessed` is false before and
  true after `RecordEvent`; `RecordEvent` is idempotent (re-recording the same
  id returns no error and `IsEventProcessed` stays true); distinct ids are
  independent.
- `database/entitlements_test.go` (append) — `ListEntitlements` for a user with
  no entitlements returns a non-nil empty slice (`len == 0`, `!= nil`).
- `auth/gating_test.go` (append) — `RequireSubscription` with a request that has
  **no user in context** redirects (303) to `/account/billing`.
- `views/account_test.go` (append) — `Billing` with `BillingView{StripeEnabled:
  false}` renders "Billing is not configured" and contains **no**
  `action="/account/billing/checkout"` or `/portal` form.
- `handlers_billing_test.go` (append):
  - Replay: two webhook calls with the same `billing.Event{ID: "evt_x", Type:
    "checkout.session.completed", ProductKey: "ebook", UserID: <uid>}` (via the
    `FakeClient`) — the entitlement is granted once; the second call returns 200
    and does not error; the dedup is observable (e.g. only one
    `processed_webhook_events` row / grant remains idempotent).
  - Store-error → 500: open a DB, build the store, **close the DB**, then call
    `HandleStripeWebhook` with a `FakeClient` returning an event with an `ID`;
    the first store call (`IsEventProcessed`) fails on the closed DB → assert
    500. (This forces the error path without a production seam — the gap the #4
    fix could not test.)

Manual: none beyond #4's existing manual Stripe round-trip.

## Documentation

No README/env changes (no new config). A one-line comment in
`handlers_billing.go` explains the dedup step.
