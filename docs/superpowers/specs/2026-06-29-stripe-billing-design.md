# Accounts Platform — Sub-project #4: Stripe Billing (Subscriptions + One-Time Purchases) — Design

**Date:** 2026-06-29
**Status:** Approved, ready for implementation planning
**Branch:** `stripe-billing` (off `accounts`)
**Target repo:** `github.com/TunedMystic/rio-project-template`
**Depends on:** Sub-project #1 (Identity Foundation). Independent of #3 (Google OAuth).

## Context

Sub-project #1 left the Billing tab a stub and reserved `stripe_customer_id` +
subscription fields on `users` for "sub-project #4." This sub-project fills
that in with a real billing foundation that two kinds of product can clone:

- **Subscription projects** — recurring access gated by subscription status.
- **One-time / directory-style projects** — permanent access to a digital
  product (e-book, business-plan upsell, etc.) gated by ownership.

Both are served by one `billing/` foundation over Stripe-hosted Checkout (so
card data never touches the app) and the Stripe Billing Portal, kept in sync by
signed webhooks. This is the largest of the four sub-projects but a single
cohesive subsystem.

## Decisions (locked during brainstorming)

| Decision | Choice | Rationale |
|---|---|---|
| Integration | **Hosted Checkout + Billing Portal** | Server-created Checkout Sessions to subscribe/buy; Stripe's hosted Portal to manage subscriptions. Minimal PCI, almost no front-end JS. |
| Scope | **Unified billing: subscriptions AND one-time purchases** | One foundation fits both project types. Subscriptions are status-gated; one-time purchases are ownership-gated via an entitlements table. |
| Product catalog | **Per-clone `Products` literal in `config.go`** | Each product is `{Key, Name, Kind, PriceID}` with the Stripe Price ID from env. `config.go` is already the per-clone seam; the cloner edits the catalog. |
| Source of truth | **Webhooks** | Subscription status and entitlement grants are written from signed webhook events into the local DB. Access derives from local state only — never from the success redirect. |
| Account model | **One Stripe Customer per user, shared** | The same `stripe_customer_id` backs both subscriptions and one-time purchases (B2C, consistent with #1). |
| Entitlements | **Permanent in v1** | A one-time purchase grants ownership forever; no expiry or refund-revocation in v1 (refunds handled in the Stripe dashboard). |
| Gating | **Fail closed** | No active subscription / no entitlement → access denied, including when Stripe is unconfigured. Safer than fail-open if prod creds are forgotten. |
| Trial | **None by default** | Keep the template simple; a `trial_period_days` is a one-line future toggle. |
| Webhook security | **Signature verification mandatory** | `STRIPE_WEBHOOK_SECRET` via stripe-go's `webhook.ConstructEvent`; unsigned/forged events rejected. |
| Library | **`github.com/stripe/stripe-go`** | Official, the one new dependency (approved in #1's dependency philosophy). Wrapped in a `billing/` package so the rest of the app never imports it. |

## Out of scope (this sub-project)

- Multiple subscription tiers / plan-change UI (one subscription product in v1).
- Entitlement expiry, refund-driven revocation, or "rental"/time-limited access.
- Invoices/receipts UI, taxes, coupons/promo codes, usage-based billing.
- Organizations/teams/seats.

## Architecture & package layout

Dependency direction unchanged: `handlers → billing → (stripe-go)`,
`handlers → auth → database`. The `billing/` package fully contains stripe-go.

- **`billing/`** (new; wraps stripe-go, like `email/` wraps Postmark)
  - `billing.go` — a `Client` interface with `EnsureCustomer(ctx, user) (customerID string, error)`, `CreateCheckoutSession(ctx, in CheckoutInput) (url string, error)`, `CreatePortalSession(ctx, customerID, returnURL string) (url string, error)`; a `StripeClient` impl; and `VerifyWebhook(payload []byte, sigHeader, secret string) (Event, error)` returning a **normalized** `Event{Type, CustomerID, UserID, Status, CurrentPeriodEnd, ProductKey}` (`UserID`/`ProductKey` come from the checkout session metadata; `Status`/`CurrentPeriodEnd` from subscription events). A `FakeClient` for tests lives in the test files.
  - `CheckoutInput` carries `CustomerID`, `Product` (Key/Kind/PriceID), `SuccessURL`, `CancelURL`; `Kind` selects `mode=subscription` vs `mode=payment` and the session metadata (`user_id`, `product_key`).
- **`database/`**
  - `users.go` (extend) — `User` gains `StripeCustomerID, SubscriptionStatus string` and `CurrentPeriodEnd time.Time`; methods `SetStripeCustomerID`, `UserByStripeCustomerID`, `UpdateSubscription`.
  - `entitlements.go` (new) — `GrantEntitlement`, `HasEntitlement`, `ListEntitlements`.
  - `migrations/0004_billing.sql`.
- **`auth/`**
  - `middleware.go` (extend) — `RequireSubscription` and `RequireEntitlement(store, productKey)`, mirroring `RequireUser` (redirect to `/account/billing` when denied). A small `HasActiveSubscription(user)` predicate.
- **Root handlers**
  - `handlers_billing.go` (new) — `HandleCheckout` (POST, CSRF), `HandlePortal` (POST, CSRF), `HandleStripeWebhook` (POST, signature-verified), `HandlePremium` (demo, subscription-gated), `HandleGuide` (demo, entitlement-gated). The Billing tab handler (`HandleBilling`) is upgraded from the stub.
- **`config/`** — `StripeSecretKey`, `StripeWebhookSecret`, the `Products []Product` catalog, `StripeEnabled()`, and `ProductByKey`.
- **`views/`** — `account.go` Billing tab lists each available product with Subscribe/Manage (subscription) or Buy/Owned (one-time); two simple demo pages (`Premium`, `Guide`).
- **`main.go`** — register billing routes (checkout/portal/webhook + the two gated demos) when `StripeEnabled()`. `go mod tidy && go mod vendor` for stripe-go.

## Data model — `database/migrations/0004_billing.sql`

```sql
ALTER TABLE users ADD COLUMN stripe_customer_id  TEXT;
ALTER TABLE users ADD COLUMN subscription_status TEXT NOT NULL DEFAULT '';  -- '', active, trialing, past_due, canceled
ALTER TABLE users ADD COLUMN current_period_end  TIMESTAMP;

CREATE UNIQUE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

CREATE TABLE entitlements (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_key TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, product_key)
);
CREATE INDEX idx_entitlements_user_id ON entitlements(user_id);
```

`UpdateSubscription(ctx, customerID, status, periodEnd)` updates by
`stripe_customer_id`. `GrantEntitlement` is an idempotent upsert
(`INSERT ... ON CONFLICT(user_id, product_key) DO NOTHING`).

## Product catalog — `config.go`

```go
type ProductKind int
const ( Subscription ProductKind = iota; OneTime )

type Product struct {
	Key     string      // stable id used in URLs/entitlements, e.g. "pro", "ebook"
	Name    string
	Kind    ProductKind
	PriceID string      // Stripe Price ID (from env; empty = product unavailable)
}
```
`config.New` builds the catalog (the cloner edits this literal), e.g.:
```go
c.Products = []Product{
	{Key: "pro",   Name: "Pro",    Kind: Subscription, PriceID: os.Getenv("STRIPE_PRICE_PRO")},
	{Key: "ebook", Name: "E-book", Kind: OneTime,      PriceID: os.Getenv("STRIPE_PRICE_EBOOK")},
}
```
`ProductByKey(key) (Product, bool)`; a product is **available** when its
`PriceID != ""`. `StripeEnabled() = StripeSecretKey != ""`.

## Flows

All Checkout/Portal sessions are created server-side; the user is redirected to
the Stripe-hosted URL.

### Subscribe / Buy — `POST /account/billing/checkout` (CSRF)

Form field `product=<key>`. Resolve the product; 400 if unknown or unavailable.
Ensure the user has a `stripe_customer_id` (create a Stripe Customer with the
user's email + `metadata.user_id` and store it if absent). Create a Checkout
Session:
- `Subscription` product → `mode=subscription`, line item = the price.
- `OneTime` product → `mode=payment`, line item = the price.
- Both → `client_reference_id = user.ID`, `metadata{user_id, product_key}`,
  `success_url=/account/billing?status=success`, `cancel_url=/account/billing`.
Redirect (303) to the session URL.

### Manage — `POST /account/billing/portal` (CSRF)

Only meaningful for subscribers. Create a Billing Portal Session for the user's
customer (`return_url=/account/billing`), redirect (303) to it. If the user has
no customer id, redirect back to `/account/billing`.

### Webhook — `POST /webhooks/stripe`

Read the raw body, verify the signature against `STRIPE_WEBHOOK_SECRET`
(`billing.VerifyWebhook`). On failure → 400. On success, switch on event type:
- `checkout.session.completed` — look up the product by `metadata.product_key`
  in the catalog. If it is a **one-time** product, grant the entitlement to the
  user (looked up by `metadata.user_id`, fallback `client_reference_id`). If it
  is a **subscription** product, ensure the `stripe_customer_id` is linked to
  the user (status itself arrives via `customer.subscription.updated`). Keying
  off the catalog `Kind` avoids trusting the Stripe `mode` field.
- `customer.subscription.updated` / `customer.subscription.deleted` — update
  `subscription_status` + `current_period_end` for the user found by
  `stripe_customer_id`.
Unhandled event types → 200 (ignored). The handler is idempotent (repeated
delivery is safe: entitlement grant is upsert; status update is last-writer).
Always return 200 on a successfully processed/ignored event so Stripe stops
retrying.

## Gating

Two middlewares in `auth`, each mirroring `RequireUser` (they run *after*
`LoadUser`, so the user is in context):

- `RequireSubscription` — allow when `HasActiveSubscription(user)` (status ∈
  {`active`, `trialing`}); else redirect to `/account/billing`.
- `RequireEntitlement(productKey)` — allow when `store.HasEntitlement(user.ID,
  productKey)`; else redirect to `/account/billing`. (This middleware needs the
  store; it is constructed as `RequireEntitlement(store, productKey)`.)

Demo pages (behind `RequireUser` first):
- `/premium` — `RequireSubscription`; renders a simple "Pro members" page.
- `/guide` — `RequireEntitlement(store, "ebook")`; renders the purchased guide.

The Billing tab (`/account/billing`) lists each **available** product:
- Subscription product: shows status (Free vs Pro · *renews/ends* `current_period_end`) with a **Subscribe** button (not subscribed) or **Manage billing** button (subscribed, → Portal).
- One-time product: **Buy** button, or an **Owned** badge when the user has the entitlement.

## Config & enablement (graceful degradation)

`config` reads `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, and each product's
price env. `StripeEnabled()` is true when the secret key is set. When disabled,
billing routes are not registered and the Billing tab renders the existing stub.
Individual product buttons render only when that product's `PriceID` is set. The
webhook route is registered when `StripeEnabled()`; it rejects (400) when
`STRIPE_WEBHOOK_SECRET` is unset. No prod fail-fast (billing is optional like
Google login).

## Security notes

- **Webhook signature verification is mandatory**; the raw request body is used
  (not a re-encoded form) for `ConstructEvent`.
- Checkout and Portal sessions are created server-side; the client cannot choose
  prices or customers — only a product `key` from the configured catalog.
- **Access is derived from webhook-updated local state only**, never from the
  `success_url` redirect (which an attacker could forge).
- CSRF on the checkout/portal POST forms (the existing per-session HMAC token).
- Session `metadata`/`client_reference_id` carry our `user_id` + `product_key`
  so webhooks attribute the purchase to the right user/product.
- Entitlement grants are idempotent; subscription updates are last-writer-wins.

## Testing

Automated (Go tests, no network — stripe-go is wrapped, a `FakeClient` backs the handlers):

- `database` — `entitlements`: grant is idempotent (second grant no-ops via the
  unique index), `HasEntitlement` true/false, cascade on user delete;
  `users`: `SetStripeCustomerID`/`UserByStripeCustomerID`/`UpdateSubscription`
  round-trip; the `stripe_customer_id` partial-unique index.
- `auth` — `HasActiveSubscription` truth table; `RequireSubscription` and
  `RequireEntitlement` allow/redirect.
- `billing` — `VerifyWebhook` rejects a bad signature and parses a known-good
  signed payload into the normalized `Event` (using stripe-go's webhook test
  signing); `CheckoutInput`→mode mapping (subscription vs payment).
- Handlers — `HandleCheckout` with a `FakeClient` ensures a customer is created
  and the response redirects to the fake session URL, with the right mode per
  product kind; `HandlePortal` redirects to the fake portal URL; the webhook
  handler maps `checkout.session.completed` (payment) → entitlement granted and
  `customer.subscription.updated` → status updated; CSRF rejection on the POSTs.

Manual (documented in README, not automated): the live round-trip in Stripe
**test mode** with `stripe listen --forward-to localhost:PORT/webhooks/stripe`,
exercising both a subscription and a one-time purchase, then confirming
`/premium` and `/guide` unlock.

## Documentation

README "Accounts & auth" (or a new "Billing" section): document
`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, and the per-product price envs
(`STRIPE_PRICE_PRO`, `STRIPE_PRICE_EBOOK`); how to edit the `Products` catalog in
`config.go`; and the `stripe listen` dev-webhook flow. Dockerfile env comment
updated. Note that unset Stripe envs disable billing (Billing tab shows the
stub).
