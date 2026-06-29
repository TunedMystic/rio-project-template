# Stripe Billing (Subscriptions + One-Time Purchases) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Billing stub with a real billing foundation: recurring subscriptions (status-gated) and one-time product purchases (ownership-gated), via hosted Stripe Checkout + Billing Portal, synced by signed webhooks.

**Architecture:** A new `billing/` package fully wraps `stripe-go` (Checkout/Portal/Customer + a webhook verifier returning a normalized event), so the rest of the app never imports Stripe and tests use a `FakeClient`. Subscription state lives on `users`; one-time ownership lives in an `entitlements` table. Two gating middlewares (`RequireSubscription`, `RequireEntitlement`) protect demo pages `/premium` and `/guide`.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio` v0.26.0 (`rio`, `rio/dom`, `rio/ui`), `modernc.org/sqlite`, and **one new dependency: `github.com/stripe/stripe-go`** (vendored).

## Global Constraints

- Module path `app`; internal imports `app/config`, `app/database`, `app/auth`, `app/billing`, `app/views`.
- **Exactly one new Go dependency: `github.com/stripe/stripe-go/vNN`.** `vNN` is the major version `go get` pulls — the import-path suffix in code MUST equal the version in `go.mod` (adjust the `vNN` shown in this plan to match). After any go.mod change: `go mod tidy && go mod vendor`. Only the `billing/` package may import stripe-go.
- Raw SQL over `database/sql`; methods hang off the existing `*database.Store`. Migrations forward-only, embedded; add `database/migrations/0004_billing.sql` (numbered 0004 to slot after sub-project #3's `0003`; the runner applies any unapplied file regardless of gaps).
- One Stripe **Customer per user, shared** across subscriptions and one-time purchases (`stripe_customer_id` on `users`, nullable, partial-unique).
- **Webhooks are the source of truth.** Access is derived from webhook-updated local state only — never from the `success_url` redirect. Webhook **signature verification is mandatory** (`STRIPE_WEBHOOK_SECRET`); use the raw request body. Webhook handling is idempotent.
- **Gating fails closed:** no active subscription / no entitlement (including when Stripe is unconfigured) → redirect to `/account/billing`. `HasActiveSubscription` = `subscription_status` ∈ {`active`, `trialing`}. Entitlements are permanent (no expiry in v1). No free trial.
- **Graceful degradation:** billing routes register only when `Conf.StripeEnabled()` (= `STRIPE_SECRET_KEY` set); per-product buttons render only when that product's `PriceID` is set; the Billing tab shows the existing stub when disabled.
- Config catalog: `Products []Product` literal in `config.go` is the per-clone seam; two demo products ship — `pro` (Subscription) and `ebook` (OneTime). Price IDs from env: `STRIPE_PRICE_PRO`, `STRIPE_PRICE_EBOOK`.
- CSRF (the existing per-session HMAC `_csrf`) on the checkout/portal POST forms.
- Run all tests with `go test ./...` from the repo root. TDD: failing test first. Commit after each task.

---

### Task 1: Migration + user billing fields + subscription store methods

**Files:**
- Create: `database/migrations/0004_billing.sql`
- Modify: `database/users.go`
- Test: `database/users_test.go` (append)

**Interfaces:**
- Consumes: existing `Store`, `CreateUser`, `UserByEmail`, `UserByID`, `scanUser`, `rowScanner`.
- Produces:
  - `User` gains `StripeCustomerID, SubscriptionStatus string` and `CurrentPeriodEnd time.Time`.
  - `func (s *Store) SetStripeCustomerID(ctx context.Context, id int64, customerID string) error`
  - `func (s *Store) UserByStripeCustomerID(ctx context.Context, customerID string) (User, error)` (`sql.ErrNoRows` if absent)
  - `func (s *Store) UpdateSubscription(ctx context.Context, customerID, status string, periodEnd time.Time) error`

- [ ] **Step 1: Write the migration**

```sql
-- database/migrations/0004_billing.sql
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

- [ ] **Step 2: Write the failing test**

```go
// database/users_test.go  (append)

func TestUsers_BillingFields(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "b@example.com", "B")
	if u.StripeCustomerID != "" || u.SubscriptionStatus != "" {
		t.Fatalf("new user billing fields not empty: %+v", u)
	}

	if err := s.SetStripeCustomerID(ctx, u.ID, "cus_123"); err != nil {
		t.Fatalf("SetStripeCustomerID: %v", err)
	}
	got, err := s.UserByStripeCustomerID(ctx, "cus_123")
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserByStripeCustomerID = %+v, err %v", got, err)
	}

	end := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	if err := s.UpdateSubscription(ctx, "cus_123", "active", end); err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}
	got, _ = s.UserByID(ctx, u.ID)
	if got.SubscriptionStatus != "active" || !got.CurrentPeriodEnd.Equal(end) {
		t.Errorf("subscription not updated: status=%q end=%v want %v", got.SubscriptionStatus, got.CurrentPeriodEnd, end)
	}

	// One customer id maps to one user (partial unique index).
	u2, _ := s.CreateUser(ctx, "c@example.com", "C")
	if err := s.SetStripeCustomerID(ctx, u2.ID, "cus_123"); err == nil {
		t.Error("expected unique-constraint error on duplicate stripe_customer_id")
	}
}
```

(`context`, `database/sql`, `errors`, `testing`, `time` are already imported by `users_test.go`.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./database/ -run TestUsers_BillingFields -v`
Expected: FAIL — `u.StripeCustomerID undefined` / `undefined: (*Store).SetStripeCustomerID`.

- [ ] **Step 4: Write the implementation**

Edit `database/users.go`. Add `"database/sql"` to the imports. Add fields to `User`:

```go
type User struct {
	ID                 int64
	Email              string
	Name               string
	CreatedAt          time.Time
	StripeCustomerID   string    // empty when no Stripe customer yet
	SubscriptionStatus string    // '', active, trialing, past_due, canceled
	CurrentPeriodEnd   time.Time // zero when no subscription
}
```

Update the two existing lookups to select the new columns (keep `CreateUser` 4-column):

```go
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, stripe_customer_id, subscription_status, current_period_end FROM users WHERE email = ?", email))
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, stripe_customer_id, subscription_status, current_period_end FROM users WHERE id = ?", id))
}
```

Add the new methods:

```go
// SetStripeCustomerID links a Stripe customer to the user.
func (s *Store) SetStripeCustomerID(ctx context.Context, id int64, customerID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET stripe_customer_id = ? WHERE id = ?", customerID, id)
	return err
}

// UserByStripeCustomerID looks up a user by their Stripe customer id.
func (s *Store) UserByStripeCustomerID(ctx context.Context, customerID string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, stripe_customer_id, subscription_status, current_period_end FROM users WHERE stripe_customer_id = ?", customerID))
}

// UpdateSubscription sets the subscription status + period end for the user with
// the given Stripe customer id.
func (s *Store) UpdateSubscription(ctx context.Context, customerID, status string, periodEnd time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET subscription_status = ?, current_period_end = ? WHERE stripe_customer_id = ?",
		status, periodEnd, customerID)
	return err
}
```

Update `scanUser` to scan the three new (nullable where applicable) columns:

```go
func (s *Store) scanUser(row rowScanner) (User, error) {
	var u User
	var cust sql.NullString
	var pend sql.NullTime
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &cust, &u.SubscriptionStatus, &pend)
	u.StripeCustomerID = cust.String
	u.CurrentPeriodEnd = pend.Time
	return u, err
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./database/ -v`
Expected: PASS (all database tests — the existing user/session/token suites use the updated `scanUser`).

- [ ] **Step 6: Commit**

```bash
git add database/migrations/0004_billing.sql database/users.go database/users_test.go
git commit -m "feat(db): billing migration, user stripe/subscription fields, store methods"
```

---

### Task 2: Entitlements store

**Files:**
- Create: `database/entitlements.go`
- Test: `database/entitlements_test.go`

**Interfaces:**
- Consumes: `Store`, `CreateUser`, `DeleteUser` (the entitlements table from Task 1's migration).
- Produces:
  - `func (s *Store) GrantEntitlement(ctx context.Context, userID int64, productKey string) error` (idempotent)
  - `func (s *Store) HasEntitlement(ctx context.Context, userID int64, productKey string) (bool, error)`
  - `func (s *Store) ListEntitlements(ctx context.Context, userID int64) ([]string, error)`

- [ ] **Step 1: Write the failing test**

```go
// database/entitlements_test.go
package database

import (
	"context"
	"testing"
)

func TestEntitlements_GrantIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "e@example.com", "E")

	has, _ := s.HasEntitlement(ctx, u.ID, "ebook")
	if has {
		t.Fatal("unexpected entitlement before grant")
	}

	if err := s.GrantEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatalf("GrantEntitlement: %v", err)
	}
	// Granting again is a no-op (unique index), not an error.
	if err := s.GrantEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatalf("second GrantEntitlement: %v", err)
	}

	has, _ = s.HasEntitlement(ctx, u.ID, "ebook")
	if !has {
		t.Error("entitlement missing after grant")
	}
	if has, _ := s.HasEntitlement(ctx, u.ID, "other"); has {
		t.Error("unrelated entitlement reported present")
	}

	list, _ := s.ListEntitlements(ctx, u.ID)
	if len(list) != 1 || list[0] != "ebook" {
		t.Errorf("ListEntitlements = %v, want [ebook]", list)
	}

	// Deleting the user cascades to entitlements.
	_ = s.DeleteUser(ctx, u.ID)
	if has, _ := s.HasEntitlement(ctx, u.ID, "ebook"); has {
		t.Error("entitlement not cascaded on user delete")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./database/ -run TestEntitlements -v`
Expected: FAIL — `undefined: (*Store).GrantEntitlement`.

- [ ] **Step 3: Write the implementation**

```go
// database/entitlements.go
package database

import "context"

// GrantEntitlement records that a user owns a one-time product. Idempotent: a
// repeated grant is a no-op (the unique index on (user_id, product_key)).
func (s *Store) GrantEntitlement(ctx context.Context, userID int64, productKey string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO entitlements (user_id, product_key) VALUES (?, ?) ON CONFLICT(user_id, product_key) DO NOTHING",
		userID, productKey)
	return err
}

// HasEntitlement reports whether the user owns the product.
func (s *Store) HasEntitlement(ctx context.Context, userID int64, productKey string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM entitlements WHERE user_id = ? AND product_key = ?)",
		userID, productKey).Scan(&exists)
	return exists, err
}

// ListEntitlements returns the product keys the user owns, oldest first.
func (s *Store) ListEntitlements(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT product_key FROM entitlements WHERE user_id = ? ORDER BY created_at", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./database/ -run TestEntitlements -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/entitlements.go database/entitlements_test.go
git commit -m "feat(db): entitlements store (grant/has/list, idempotent)"
```

---

### Task 3: Config — Stripe creds + product catalog

**Files:**
- Modify: `config/config.go`
- Test: `config/config_test.go` (append)

**Interfaces:**
- Consumes: existing `Config`, `New`.
- Produces:
  - `type ProductKind int` with `const ( Subscription ProductKind = iota; OneTime )`
  - `type Product struct { Key, Name string; Kind ProductKind; PriceID string }` + `func (p Product) Available() bool`
  - `Config` gains `StripeSecretKey, StripeWebhookSecret string` and `Products []Product`.
  - `func (c Config) StripeEnabled() bool`
  - `func (c Config) ProductByKey(key string) (Product, bool)`

- [ ] **Step 1: Write the failing test**

```go
// config/config_test.go  (append)

func TestStripeConfig(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	t.Setenv("STRIPE_PRICE_PRO", "price_pro")
	t.Setenv("STRIPE_PRICE_EBOOK", "") // unavailable

	c := New("debug", "h")
	if c.StripeSecretKey != "sk_test_123" || c.StripeWebhookSecret != "whsec_123" {
		t.Fatalf("stripe creds not loaded: %+v", c)
	}
	if !c.StripeEnabled() {
		t.Error("StripeEnabled should be true when the secret key is set")
	}

	pro, ok := c.ProductByKey("pro")
	if !ok || pro.Kind != Subscription || pro.PriceID != "price_pro" || !pro.Available() {
		t.Errorf("pro product = %+v ok=%v", pro, ok)
	}
	ebook, ok := c.ProductByKey("ebook")
	if !ok || ebook.Kind != OneTime || ebook.Available() {
		t.Errorf("ebook should exist but be unavailable: %+v ok=%v", ebook, ok)
	}
	if _, ok := c.ProductByKey("nope"); ok {
		t.Error("ProductByKey returned ok for an unknown key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestStripeConfig -v`
Expected: FAIL — `undefined: Subscription` / `c.StripeEnabled undefined`.

- [ ] **Step 3: Write the implementation**

In `config/config.go`, add the product types (near the top, after the `Link` type):

```go
// ProductKind distinguishes a recurring subscription from a one-time purchase.
type ProductKind int

const (
	Subscription ProductKind = iota
	OneTime
)

// Product is a sellable item in the catalog. PriceID comes from env; an empty
// PriceID means the product is not available (button hidden).
type Product struct {
	Key     string // stable id used in URLs/entitlements, e.g. "pro", "ebook"
	Name    string
	Kind    ProductKind
	PriceID string
}

// Available reports whether the product has a configured Stripe price.
func (p Product) Available() bool { return p.PriceID != "" }
```

Add to the `Config` struct:

```go
	StripeSecretKey     string
	StripeWebhookSecret string
	Products            []Product
```

In `New`, after the existing auth-env block (where `TrustProxy` is set), add:

```go
	c.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	c.StripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	// Product catalog — the per-clone seam. Edit this list per product; each
	// price id comes from env so the same binary works across environments.
	c.Products = []Product{
		{Key: "pro", Name: "Pro", Kind: Subscription, PriceID: os.Getenv("STRIPE_PRICE_PRO")},
		{Key: "ebook", Name: "E-book", Kind: OneTime, PriceID: os.Getenv("STRIPE_PRICE_EBOOK")},
	}
```

Add the methods:

```go
// StripeEnabled reports whether billing is configured.
func (c Config) StripeEnabled() bool { return c.StripeSecretKey != "" }

// ProductByKey finds a catalog product by its key.
func (c Config) ProductByKey(key string) (Product, bool) {
	for _, p := range c.Products {
		if p.Key == key {
			return p, true
		}
	}
	return Product{}, false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "feat(config): Stripe creds + product catalog (StripeEnabled, ProductByKey)"
```

---

### Task 4: `billing/` package — stripe-go wrapper + webhook verifier + fake

**Files:**
- Modify: `go.mod`, `go.sum`, `vendor/` (add `github.com/stripe/stripe-go/vNN`)
- Create: `billing/billing.go`, `billing/fake.go`
- Test: `billing/billing_test.go`

**Interfaces:**
- Consumes: stdlib; `github.com/stripe/stripe-go/vNN` (+ `/customer`, `/checkout/session`, `/billingportal/session`, `/webhook`).
- Produces:
  - `type CheckoutInput struct { CustomerID, PriceID string; Subscription bool; UserID int64; ProductKey, SuccessURL, CancelURL string }`
  - `type Event struct { Type, CustomerID, UserID, Status string; CurrentPeriodEnd time.Time; ProductKey string }`
  - `type Client interface { EnsureCustomer(ctx, email string, userID int64, existingID string) (string, error); CreateCheckoutSession(ctx, in CheckoutInput) (string, error); CreatePortalSession(ctx, customerID, returnURL string) (string, error); VerifyWebhook(payload []byte, sigHeader, secret string) (Event, error) }`
  - `type StripeClient struct{}` implementing `Client`; `func New(secretKey string) *StripeClient`
  - `type FakeClient struct{...}` implementing `Client` (for handler tests)

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/stripe/stripe-go/v82@latest
go mod tidy
go mod vendor
```
Expected: `go.mod` gains `github.com/stripe/stripe-go/vNN` (note the actual `vNN` major it resolves — **use that same `vNN` in every import path below**). If `go get` fails due to no network/proxy access, STOP and report BLOCKED with the exact error (the controller may run it).

- [ ] **Step 2: Write the failing test**

Replace `vNN` with the major version from Step 1.

```go
// billing/billing_test.go
package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

// stripeSignature builds a valid Stripe-Signature header for payload (matches
// Stripe's scheme: HMAC-SHA256 of "timestamp.payload", hex-encoded).
func stripeSignature(payload []byte, secret string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, payload)))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

func TestVerifyWebhook_GoodAndBadSignature(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_1","type":"customer.subscription.updated","data":{"object":{"customer":"cus_9","status":"active","current_period_end":1893456000}}}`)

	sig := stripeSignature(payload, secret, time.Now().Unix())
	ev, err := (&StripeClient{}).VerifyWebhook(payload, sig, secret)
	if err != nil {
		t.Fatalf("VerifyWebhook (good): %v", err)
	}
	if ev.Type != "customer.subscription.updated" || ev.CustomerID != "cus_9" || ev.Status != "active" {
		t.Errorf("event = %+v", ev)
	}
	if ev.CurrentPeriodEnd.Unix() != 1893456000 {
		t.Errorf("period end = %v", ev.CurrentPeriodEnd)
	}

	// A tampered/wrong signature is rejected.
	if _, err := (&StripeClient{}).VerifyWebhook(payload, sig, "whsec_other"); err == nil {
		t.Error("VerifyWebhook accepted a payload signed with another secret")
	}
}

func TestVerifyWebhook_CheckoutMetadata(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_2","type":"checkout.session.completed","data":{"object":{"customer":"cus_5","metadata":{"user_id":"42","product_key":"ebook"}}}}`)
	sig := stripeSignature(payload, secret, time.Now().Unix())
	ev, err := (&StripeClient{}).VerifyWebhook(payload, sig, secret)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if ev.Type != "checkout.session.completed" || ev.UserID != "42" || ev.ProductKey != "ebook" || ev.CustomerID != "cus_5" {
		t.Errorf("checkout event = %+v", ev)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./billing/ -v`
Expected: FAIL — `undefined: StripeClient`.

- [ ] **Step 4: Write the implementation**

Replace `vNN` with the major version from Step 1 in all import paths.

```go
// billing/billing.go
package billing

import (
	"context"
	"time"

	stripe "github.com/stripe/stripe-go/vNN"
	portalsession "github.com/stripe/stripe-go/vNN/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/vNN/checkout/session"
	"github.com/stripe/stripe-go/vNN/customer"
	"github.com/stripe/stripe-go/vNN/webhook"
)

// CheckoutInput describes a Checkout Session to create.
type CheckoutInput struct {
	CustomerID   string
	PriceID      string
	Subscription bool // true → mode=subscription, false → mode=payment
	UserID       int64
	ProductKey   string
	SuccessURL   string
	CancelURL    string
}

// Event is a normalized Stripe webhook event (only the fields the app needs).
type Event struct {
	Type             string
	CustomerID       string
	UserID           string // from checkout session metadata.user_id
	Status           string // subscription status
	CurrentPeriodEnd time.Time
	ProductKey       string // from checkout session metadata.product_key
}

// Client is the billing operations the app depends on (a fake backs tests).
type Client interface {
	EnsureCustomer(ctx context.Context, email string, userID int64, existingID string) (string, error)
	CreateCheckoutSession(ctx context.Context, in CheckoutInput) (string, error)
	CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error)
	VerifyWebhook(payload []byte, sigHeader, secret string) (Event, error)
}

// StripeClient talks to the real Stripe API.
type StripeClient struct{}

// New sets the global Stripe key and returns a StripeClient.
func New(secretKey string) *StripeClient {
	stripe.Key = secretKey
	return &StripeClient{}
}

// EnsureCustomer returns existingID if set, else creates a Stripe Customer
// carrying the app user id in metadata.
func (c *StripeClient) EnsureCustomer(ctx context.Context, email string, userID int64, existingID string) (string, error) {
	if existingID != "" {
		return existingID, nil
	}
	params := &stripe.CustomerParams{Email: stripe.String(email)}
	params.AddMetadata("user_id", itoa(userID))
	cust, err := customer.New(params)
	if err != nil {
		return "", err
	}
	return cust.ID, nil
}

func (c *StripeClient) CreateCheckoutSession(ctx context.Context, in CheckoutInput) (string, error) {
	mode := stripe.CheckoutSessionModePayment
	if in.Subscription {
		mode = stripe.CheckoutSessionModeSubscription
	}
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(mode)),
		Customer:          stripe.String(in.CustomerID),
		ClientReferenceID: stripe.String(itoa(in.UserID)),
		SuccessURL:        stripe.String(in.SuccessURL),
		CancelURL:         stripe.String(in.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(in.PriceID), Quantity: stripe.Int64(1)},
		},
	}
	params.AddMetadata("user_id", itoa(in.UserID))
	params.AddMetadata("product_key", in.ProductKey)
	s, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (c *StripeClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	}
	ps, err := portalsession.New(params)
	if err != nil {
		return "", err
	}
	return ps.URL, nil
}

// VerifyWebhook validates the Stripe signature and normalizes the event.
func (c *StripeClient) VerifyWebhook(payload []byte, sigHeader, secret string) (Event, error) {
	ev, err := webhook.ConstructEvent(payload, sigHeader, secret)
	if err != nil {
		return Event{}, err
	}
	out := Event{Type: string(ev.Type)}
	switch ev.Type {
	case "checkout.session.completed":
		var s stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Raw, &s); err != nil {
			return Event{}, err
		}
		if s.Customer != nil {
			out.CustomerID = s.Customer.ID
		}
		out.UserID = s.Metadata["user_id"]
		out.ProductKey = s.Metadata["product_key"]
	case "customer.subscription.updated", "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil {
			return Event{}, err
		}
		if sub.Customer != nil {
			out.CustomerID = sub.Customer.ID
		}
		out.Status = string(sub.Status)
		out.CurrentPeriodEnd = time.Unix(sub.CurrentPeriodEnd, 0)
	}
	return out, nil
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
```

Add `"encoding/json"` and `"strconv"` to the imports above (used by `VerifyWebhook` and `itoa`).

> Note on stripe-go versions: field names here (`CheckoutSession.Customer.ID`, `Subscription.CurrentPeriodEnd` as unix int64, `Subscription.Status`, `ev.Data.Raw`, the `…ModeSubscription/ModePayment` and param helpers) follow the long-stable stripe-go API. If the pulled `vNN` differs on a specific name, adjust to that version's symbol — the **public `billing` interface (Client/Event/CheckoutInput) must not change**. Verify with `go build ./billing/`.

```go
// billing/fake.go
package billing

import "context"

// FakeClient is an in-memory Client for tests.
type FakeClient struct {
	CustomerID    string // returned by EnsureCustomer when existingID == ""
	CheckoutURL   string
	PortalURL     string
	CreatedCust   bool
	LastCheckout  CheckoutInput
	NextEvent     Event
	NextWebhookErr error
}

func (f *FakeClient) EnsureCustomer(ctx context.Context, email string, userID int64, existingID string) (string, error) {
	if existingID != "" {
		return existingID, nil
	}
	f.CreatedCust = true
	return f.CustomerID, nil
}

func (f *FakeClient) CreateCheckoutSession(ctx context.Context, in CheckoutInput) (string, error) {
	f.LastCheckout = in
	return f.CheckoutURL, nil
}

func (f *FakeClient) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	return f.PortalURL, nil
}

func (f *FakeClient) VerifyWebhook(payload []byte, sigHeader, secret string) (Event, error) {
	return f.NextEvent, f.NextWebhookErr
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./billing/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum vendor billing/
git commit -m "feat(billing): stripe-go wrapper (checkout/portal/customer + webhook verifier) and fake"
```

---

### Task 5: Auth gating — subscription + entitlement middleware

**Files:**
- Modify: `auth/middleware.go`
- Test: `auth/gating_test.go`

**Interfaces:**
- Consumes: `database.Store`, `database.User`, existing `UserFrom`, `userKey`.
- Produces:
  - `func HasActiveSubscription(u database.User) bool`
  - `func RequireSubscription(next http.Handler) http.Handler`
  - `func RequireEntitlement(store *database.Store, productKey string) func(http.Handler) http.Handler`

- [ ] **Step 1: Write the failing test**

```go
// auth/gating_test.go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"app/database"
)

func TestHasActiveSubscription(t *testing.T) {
	for status, want := range map[string]bool{"active": true, "trialing": true, "past_due": false, "canceled": false, "": false} {
		if got := HasActiveSubscription(database.User{SubscriptionStatus: status}); got != want {
			t.Errorf("HasActiveSubscription(%q) = %v, want %v", status, got, want)
		}
	}
}

func reqWithUser(u database.User) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/premium", nil)
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
}

func TestRequireSubscription(t *testing.T) {
	ok := RequireSubscription(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	rec := httptest.NewRecorder()
	ok.ServeHTTP(rec, reqWithUser(database.User{SubscriptionStatus: "active"}))
	if rec.Code != 200 {
		t.Errorf("active subscriber blocked: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	ok.ServeHTTP(rec, reqWithUser(database.User{SubscriptionStatus: ""}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account/billing" {
		t.Errorf("non-subscriber not redirected: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRequireEntitlement(t *testing.T) {
	db, _ := database.Open(filepath.Join(t.TempDir(), "g.db"))
	t.Cleanup(func() { db.Close() })
	_ = database.MigrateUp(db)
	store := database.NewStore(db)
	u, _ := store.CreateUser(context.Background(), "g@example.com", "G")
	_ = store.GrantEntitlement(context.Background(), u.ID, "ebook")

	guard := RequireEntitlement(store, "ebook")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))

	rec := httptest.NewRecorder()
	guard.ServeHTTP(rec, reqWithUser(u))
	if rec.Code != 200 {
		t.Errorf("owner blocked: %d", rec.Code)
	}

	other, _ := store.CreateUser(context.Background(), "h@example.com", "H")
	rec = httptest.NewRecorder()
	guard.ServeHTTP(rec, reqWithUser(other))
	if rec.Code != http.StatusSeeOther {
		t.Errorf("non-owner not redirected: %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./auth/ -run 'TestHasActiveSubscription|TestRequireSubscription|TestRequireEntitlement' -v`
Expected: FAIL — `undefined: HasActiveSubscription`.

- [ ] **Step 3: Write the implementation**

Append to `auth/middleware.go` (it already imports `net/http` and `app/database`):

```go
// HasActiveSubscription reports whether the user's subscription grants access.
func HasActiveSubscription(u database.User) bool {
	return u.SubscriptionStatus == "active" || u.SubscriptionStatus == "trialing"
}

// RequireSubscription gates a handler to users with an active subscription,
// redirecting others to the billing page.
func RequireSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFrom(r.Context())
		if !ok || !HasActiveSubscription(u) {
			http.Redirect(w, r, "/account/billing", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireEntitlement gates a handler to users who own productKey, redirecting
// others to the billing page.
func RequireEntitlement(store *database.Store, productKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFrom(r.Context())
			if !ok {
				http.Redirect(w, r, "/account/billing", http.StatusSeeOther)
				return
			}
			has, err := store.HasEntitlement(r.Context(), u.ID, productKey)
			if err != nil || !has {
				http.Redirect(w, r, "/account/billing", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./auth/ -v`
Expected: PASS (all auth tests).

- [ ] **Step 5: Commit**

```bash
git add auth/middleware.go auth/gating_test.go
git commit -m "feat(auth): RequireSubscription and RequireEntitlement gating"
```

---

### Task 6: Handlers — checkout + portal + main wiring

**Files:**
- Create: `handlers_billing.go`
- Modify: `main.go`
- Test: `handlers_billing_test.go`

**Interfaces:**
- Consumes: `database.Store`, `billing.Client`/`billing.FakeClient`/`billing.CheckoutInput`, `config` (`Conf.ProductByKey`, `config.Subscription`, `Conf.BaseURL`, `Conf.StripeEnabled`, `Conf.StripeSecretKey`), `auth.UserFrom`, `auth.RequireUser`, `requireCSRF`, `account` helper.
- Produces:
  - `func HandleCheckout(store *database.Store, bc billing.Client) http.Handler` (POST, CSRF)
  - `func HandlePortal(store *database.Store, bc billing.Client) http.Handler` (POST, CSRF)
  - `main.go` builds `billing.New(...)` and registers `/account/billing/checkout` + `/account/billing/portal` under `auth.RequireUser` when `Conf.StripeEnabled()`.

- [ ] **Step 1: Write the failing test**

The test reuses `authTestStore`, `loggedInRequestSession`, `loggedInWith` from sub-project #1 (in `handlers_account_test.go`).

```go
// handlers_billing_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/auth"
	"app/billing"
)

func TestHandleCheckout_SubscriptionProduct(t *testing.T) {
	t.Setenv("STRIPE_PRICE_PRO", "price_pro")
	useConf(t) // rebuild Conf so the catalog sees the price (restored after the test)

	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	csrf := auth.CSRFToken(Conf.AppSecret, sess.ID)
	fake := &billing.FakeClient{CustomerID: "cus_new", CheckoutURL: "https://stripe/checkout"}

	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/billing/checkout", "product=pro&_csrf="+csrf)
	rec := httptest.NewRecorder()
	HandleCheckout(store, fake).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://stripe/checkout" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
	if !fake.LastCheckout.Subscription {
		t.Error("expected subscription mode for the pro product")
	}
	if !fake.CreatedCust {
		t.Error("expected a customer to be created")
	}
	got, _ := store.UserByID(context.Background(), u.ID)
	if got.StripeCustomerID != "cus_new" {
		t.Errorf("customer id not stored: %q", got.StripeCustomerID)
	}
}

func TestHandleCheckout_BadCSRF(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/billing/checkout", "product=pro&_csrf=wrong")
	rec := httptest.NewRecorder()
	HandleCheckout(store, &billing.FakeClient{}).ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestHandlePortal_RedirectsToPortal(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	_ = store.SetStripeCustomerID(context.Background(), u.ID, "cus_x")
	// Reload the user so the cookie's context carries the customer id.
	csrf := auth.CSRFToken(Conf.AppSecret, sess.ID)
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/billing/portal", "_csrf="+csrf)
	rec := httptest.NewRecorder()
	HandlePortal(store, &billing.FakeClient{PortalURL: "https://stripe/portal"}).ServeHTTP(rec, r)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://stripe/portal" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
}

// useConf rebuilds the global Conf from current env for the duration of the
// test, restoring it afterward so other tests are unaffected.
func useConf(t *testing.T) {
	t.Helper()
	old := Conf
	Conf = config.New("debug", "test")
	t.Cleanup(func() { Conf = old })
}
```

> Implementer note: `loggedInWith` runs the request through `auth.LoadUser`, which loads the user **from the DB** into context — so `HandlePortal`'s `user.StripeCustomerID` reflects the `SetStripeCustomerID` call above. Add `"app/config"` to the test imports for `useConf`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'TestHandleCheckout|TestHandlePortal' -v`
Expected: FAIL — `undefined: HandleCheckout`.

- [ ] **Step 3: Write the handlers**

```go
// handlers_billing.go
package main

import (
	"net/http"

	"app/auth"
	"app/billing"
	"app/config"
	"app/database"

	"github.com/tunedmystic/rio"
)

func HandleCheckout(store *database.Store, bc billing.Client) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		product, ok := Conf.ProductByKey(r.FormValue("product"))
		if !ok || !product.Available() {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
		customerID, err := bc.EnsureCustomer(r.Context(), user.Email, user.ID, user.StripeCustomerID)
		if err != nil {
			return err
		}
		if user.StripeCustomerID == "" {
			if err := store.SetStripeCustomerID(r.Context(), user.ID, customerID); err != nil {
				return err
			}
		}
		url, err := bc.CreateCheckoutSession(r.Context(), billing.CheckoutInput{
			CustomerID:   customerID,
			PriceID:      product.PriceID,
			Subscription: product.Kind == config.Subscription,
			UserID:       user.ID,
			ProductKey:   product.Key,
			SuccessURL:   Conf.BaseURL + "/account/billing?status=success",
			CancelURL:    Conf.BaseURL + "/account/billing",
		})
		if err != nil {
			return err
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandlePortal(store *database.Store, bc billing.Client) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		if user.StripeCustomerID == "" {
			http.Redirect(w, r, "/account/billing", http.StatusSeeOther)
			return nil
		}
		url, err := bc.CreatePortalSession(r.Context(), user.StripeCustomerID, Conf.BaseURL+"/account/billing")
		if err != nil {
			return err
		}
		http.Redirect(w, r, url, http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Wire `main.go`**

In `run()`, after the account routes, add a billing block. First, near the other constructions, build the billing client when enabled and keep it in scope for Tasks 7 + the demos:

```go
	// Billing (optional: only when Stripe is configured)
	if Conf.StripeEnabled() {
		bc := billing.New(Conf.StripeSecretKey)
		s.Handle("/account/billing/checkout", auth.RequireUser(HandleCheckout(store, bc)))
		s.Handle("/account/billing/portal", auth.RequireUser(HandlePortal(store, bc)))
	}
```
Add `"app/billing"` to `main.go` imports.

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && go test ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_billing.go handlers_billing_test.go main.go
git commit -m "feat: checkout + portal handlers with Stripe customer creation"
```

---

### Task 7: Handler — Stripe webhook

**Files:**
- Modify: `handlers_billing.go`, `main.go`
- Test: `handlers_billing_test.go` (append)

**Interfaces:**
- Consumes: `database.Store` (`GrantEntitlement`, `SetStripeCustomerID`, `UpdateSubscription`), `billing.Client` (`VerifyWebhook`), `config` (`Conf.ProductByKey`, `config.OneTime`, `Conf.StripeWebhookSecret`), `billing.Event`.
- Produces:
  - `func HandleStripeWebhook(store *database.Store, bc billing.Client) http.Handler`
  - `main.go` registers `/webhooks/stripe` (public) when `Conf.StripeEnabled()`.

- [ ] **Step 1: Write the failing test**

```go
// handlers_billing_test.go  (append)

func TestHandleStripeWebhook_GrantsEntitlement(t *testing.T) {
	t.Setenv("STRIPE_PRICE_EBOOK", "price_ebook")
	useConf(t)
	store := authTestStore(t)
	u, _ := store.CreateUser(context.Background(), "w@example.com", "W")

	fake := &billing.FakeClient{NextEvent: billing.Event{
		Type: "checkout.session.completed", CustomerID: "cus_w",
		UserID: itoaTest(u.ID), ProductKey: "ebook",
	}}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	HandleStripeWebhook(store, fake).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if has, _ := store.HasEntitlement(context.Background(), u.ID, "ebook"); !has {
		t.Error("entitlement not granted from checkout.session.completed")
	}
}

func TestHandleStripeWebhook_UpdatesSubscription(t *testing.T) {
	store := authTestStore(t)
	u, _ := store.CreateUser(context.Background(), "s@example.com", "S")
	_ = store.SetStripeCustomerID(context.Background(), u.ID, "cus_s")

	fake := &billing.FakeClient{NextEvent: billing.Event{
		Type: "customer.subscription.updated", CustomerID: "cus_s", Status: "active",
	}}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	HandleStripeWebhook(store, fake).ServeHTTP(rec, req)

	got, _ := store.UserByID(context.Background(), u.ID)
	if got.SubscriptionStatus != "active" {
		t.Errorf("status = %q, want active", got.SubscriptionStatus)
	}
}

func TestHandleStripeWebhook_BadSignature(t *testing.T) {
	store := authTestStore(t)
	fake := &billing.FakeClient{NextWebhookErr: http.ErrBodyNotAllowed} // any non-nil error
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	HandleStripeWebhook(store, fake).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func itoaTest(n int64) string { return strconv.FormatInt(n, 10) }
```

Add `"strconv"` to the test file imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestHandleStripeWebhook -v`
Expected: FAIL — `undefined: HandleStripeWebhook`.

- [ ] **Step 3: Write the handler**

Append to `handlers_billing.go`. Add `"io"`, `"strconv"`, and `"time"` is not needed; add `"io"` and `"strconv"` to its imports.

```go
func HandleStripeWebhook(store *database.Store, bc billing.Client) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}
		event, err := bc.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), Conf.StripeWebhookSecret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return nil
		}

		switch event.Type {
		case "checkout.session.completed":
			uid, _ := strconv.ParseInt(event.UserID, 10, 64)
			product, ok := Conf.ProductByKey(event.ProductKey)
			if ok && product.Kind == config.OneTime {
				if uid != 0 {
					_ = store.GrantEntitlement(r.Context(), uid, event.ProductKey)
				}
			} else if uid != 0 && event.CustomerID != "" {
				// Subscription checkout: ensure the customer is linked (status
				// itself arrives via customer.subscription.updated).
				_ = store.SetStripeCustomerID(r.Context(), uid, event.CustomerID)
			}
		case "customer.subscription.updated", "customer.subscription.deleted":
			if event.CustomerID != "" {
				_ = store.UpdateSubscription(r.Context(), event.CustomerID, event.Status, event.CurrentPeriodEnd)
			}
		}
		w.WriteHeader(http.StatusOK)
		return nil
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Wire `main.go`**

Inside the existing `if Conf.StripeEnabled() {` block in `run()`, add (the webhook is public — no `RequireUser`):

```go
		s.Handle("/webhooks/stripe", HandleStripeWebhook(store, bc))
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && go test ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_billing.go handlers_billing_test.go main.go
git commit -m "feat: Stripe webhook handler (entitlement grant + subscription sync)"
```

---

### Task 8: Views + handler — Billing tab

**Files:**
- Modify: `views/account.go`, `handlers_account.go`, `main.go`
- Test: `views/account_test.go` (append)

**Interfaces:**
- Consumes: `config.Product`, `config.Subscription`, `database.Store` (`ListEntitlements`), existing view helpers, `auth.UserFrom`, `account` helper, `accountView`.
- Produces:
  - `type BillingView struct { StripeEnabled bool; Products []config.Product; Status string; PeriodEnd time.Time; Owned map[string]bool }`
  - `func Billing(pd config.PageData, meta config.Meta, av AccountView, bv BillingView) dom.Node` (replaces the stub signature)
  - `func HandleBilling(store *database.Store) http.Handler` (replaces `HandleBilling()`)
  - `main.go` updates the `/account/billing` registration to `HandleBilling(store)`.

- [ ] **Step 1: Write the failing test**

```go
// views/account_test.go  (append)

func TestBilling_SubscribeAndBuy(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "billing", CSRF: "c"}
	bv := BillingView{
		StripeEnabled: true,
		Products: []config.Product{
			{Key: "pro", Name: "Pro", Kind: config.Subscription, PriceID: "price_pro"},
			{Key: "ebook", Name: "E-book", Kind: config.OneTime, PriceID: "price_ebook"},
		},
		Status: "",
		Owned:  map[string]bool{},
	}
	var b bytes.Buffer
	_ = Billing(pd, config.Meta{Title: "Billing"}, av, bv).Render(&b)
	html := b.String()
	for _, want := range []string{
		`action="/account/billing/checkout"`, // subscribe + buy post here
		`value="pro"`,                         // subscribe button carries the product key
		`value="ebook"`,                       // buy button carries the product key
		"E-book",                              // product name shown
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Billing missing %q", want)
		}
	}

	// Subscribed → Manage (portal); owned one-time → "Owned".
	bv.Status = "active"
	bv.Owned = map[string]bool{"ebook": true}
	var b2 bytes.Buffer
	_ = Billing(pd, config.Meta{Title: "Billing"}, av, bv).Render(&b2)
	html2 := b2.String()
	if !strings.Contains(html2, `action="/account/billing/portal"`) {
		t.Error("subscribed account should show the Manage (portal) form")
	}
	if !strings.Contains(html2, "Owned") {
		t.Error("owned product should show an Owned badge")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestBilling -v`
Expected: FAIL — `undefined: BillingView` / `Billing` arity mismatch.

- [ ] **Step 3: Rewrite the Billing view**

In `views/account.go`, add `"time"` and ensure `"app/config"` is imported (it is). Replace the stub `Billing` function with:

```go
// BillingView is the billing tab's per-request data.
type BillingView struct {
	StripeEnabled bool
	Products      []config.Product
	Status        string // subscription_status
	PeriodEnd     time.Time
	Owned         map[string]bool
}

func Billing(pd config.PageData, meta config.Meta, av AccountView, bv BillingView) dom.Node {
	if !bv.StripeEnabled {
		return accountShell(pd, meta, av,
			card(
				ruledHeading("Billing"),
				dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"), dom.Text("Billing is not configured.")),
			),
		)
	}

	rows := make([]dom.Node, 0, len(bv.Products))
	for _, p := range bv.Products {
		if !p.Available() {
			continue
		}
		rows = append(rows, billingRow(av, p, bv))
	}

	return accountShell(pd, meta, av,
		card(
			ruledHeading("Billing"),
			dom.Div(withClass("mt-2", rows)...),
		),
	)
}

// billingRow renders one product with the right action for its kind/state.
func billingRow(av AccountView, p config.Product, bv BillingView) dom.Node {
	var right dom.Node
	switch {
	case p.Kind == config.Subscription && bv.Status == "active" || p.Kind == config.Subscription && bv.Status == "trialing":
		right = billingForm("/account/billing/portal", av.CSRF, "", "Manage billing")
	case p.Kind == config.Subscription:
		right = billingForm("/account/billing/checkout", av.CSRF, p.Key, "Subscribe")
	case bv.Owned[p.Key]:
		right = dom.Span(
			dom.Class("inline-flex shrink-0 items-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-[length:var(--font-size-sm)] font-medium ring-1 ring-inset bg-[var(--color-success)]/12 text-[var(--color-success)] ring-[var(--color-success)]/25"),
			dom.Text("Owned"))
	default:
		right = billingForm("/account/billing/checkout", av.CSRF, p.Key, "Buy")
	}

	sub := "One-time purchase"
	if p.Kind == config.Subscription {
		sub = "Subscription"
		if bv.Status == "active" || bv.Status == "trialing" {
			sub = "Active"
		}
	}
	return dom.Div(
		dom.Class("flex items-center justify-between gap-4 border-b border-[var(--color-border)] py-4 last:border-0"),
		dom.Div(dom.Class("min-w-0"),
			dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text(p.Name)),
			dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(sub)),
		),
		right,
	)
}

// billingForm is a small POST form with the CSRF token and an optional product key.
func billingForm(action, csrf, productKey, label string) dom.Node {
	children := []dom.Node{
		dom.Method("post"),
		dom.Action(action),
		csrfInput(csrf),
	}
	if productKey != "" {
		children = append(children, dom.Input(dom.Type("hidden"), dom.Name("product"), dom.Value(productKey)))
	}
	children = append(children,
		dom.Button(dom.Type("submit"),
			dom.Class("shrink-0 inline-flex items-center justify-center rounded-[var(--radius-base)] px-4 py-2 text-[length:var(--font-size-sm)] font-semibold bg-[var(--color-primary)] text-[var(--color-on-primary)] shadow-sm transition hover:shadow-md hover:brightness-105 cursor-pointer"),
			dom.Text(label)))
	return dom.Form(children...)
}
```

- [ ] **Step 4: Upgrade `HandleBilling` in `handlers_account.go`**

Replace the stub handler. Add `"app/views"` is already imported; add `"app/database"` is already imported.

```go
func HandleBilling(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		owned := map[string]bool{}
		keys, err := store.ListEntitlements(r.Context(), user.ID)
		if err != nil {
			return err
		}
		for _, k := range keys {
			owned[k] = true
		}
		bv := views.BillingView{
			StripeEnabled: Conf.StripeEnabled(),
			Products:      Conf.Products,
			Status:        user.SubscriptionStatus,
			PeriodEnd:     user.CurrentPeriodEnd,
			Owned:         owned,
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Billing")
		return render(w, http.StatusOK, views.Billing(Conf.PageDataFor(account(r)), meta, accountView(r, "billing"), bv))
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 5: Update the `main.go` registration**

Change the existing account billing route from `HandleBilling()` to `HandleBilling(store)`:

```go
	s.Handle("/account/billing", auth.RequireUser(HandleBilling(store)))
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && go test ./views/ ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add views/account.go handlers_account.go main.go views/account_test.go
git commit -m "feat: billing tab UI (subscribe/manage/buy/owned)"
```

---

### Task 9: Demo gated pages — `/premium` and `/guide`

**Files:**
- Create: `views/billing.go`
- Modify: `handlers_billing.go`, `main.go`
- Test: `views/billing_test.go`

**Interfaces:**
- Consumes: `config.PageData`, `config.Meta`, `Page`, `pageHeader`/`shell`/`card` helpers, `auth.RequireUser`/`RequireSubscription`/`RequireEntitlement`, `account` helper.
- Produces:
  - `func Premium(pd config.PageData, meta config.Meta) dom.Node`
  - `func Guide(pd config.PageData, meta config.Meta) dom.Node`
  - `func HandlePremium() http.Handler`, `func HandleGuide() http.Handler`
  - `main.go` registers `/premium` (RequireUser → RequireSubscription) and `/guide` (RequireUser → RequireEntitlement(store, "ebook")) when `Conf.StripeEnabled()`.

- [ ] **Step 1: Write the failing test**

```go
// views/billing_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestPremiumAndGuide_Render(t *testing.T) {
	pd := testPageData()
	var b bytes.Buffer
	_ = Premium(pd, config.Meta{Title: "Premium"}).Render(&b)
	if !strings.Contains(b.String(), "Pro members") {
		t.Error("Premium page missing expected copy")
	}
	var b2 bytes.Buffer
	_ = Guide(pd, config.Meta{Title: "Guide"}).Render(&b2)
	if !strings.Contains(b2.String(), "guide") {
		t.Error("Guide page missing expected copy")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestPremiumAndGuide -v`
Expected: FAIL — `undefined: Premium`.

- [ ] **Step 3: Write the demo views**

```go
// views/billing.go
package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
)

// Premium is the subscription-gated demo page.
func Premium(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		pageHeader("Premium", "Subscriber-only content."),
		dom.Section(dom.Class("py-12"), shell(
			card(
				ruledHeading("Pro members"),
				dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"),
					dom.Text("You're a Pro member — this page is gated by an active subscription.")),
			),
		)),
	)
}

// Guide is the entitlement-gated demo page (requires owning the "ebook").
func Guide(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		pageHeader("The Guide", "Your purchased digital product."),
		dom.Section(dom.Class("py-12"), shell(
			card(
				ruledHeading("Thanks for your purchase"),
				dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"),
					dom.Text("This guide is gated by a one-time purchase entitlement.")),
			),
		)),
	)
}
```

- [ ] **Step 4: Write the handlers**

Append to `handlers_billing.go` (imports `app/views` — add it):

```go
func HandlePremium() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Premium")
		return render(w, http.StatusOK, views.Premium(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleGuide() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "The Guide")
		return render(w, http.StatusOK, views.Guide(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}
```

Add `"app/views"` to `handlers_billing.go` imports.

- [ ] **Step 5: Wire `main.go`**

Inside the `if Conf.StripeEnabled() {` block, add the two gated demos:

```go
		s.Handle("/premium", auth.RequireUser(auth.RequireSubscription(HandlePremium())))
		s.Handle("/guide", auth.RequireUser(auth.RequireEntitlement(store, "ebook")(HandleGuide())))
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && go test ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add views/billing.go views/billing_test.go handlers_billing.go main.go
git commit -m "feat: subscription-gated /premium and entitlement-gated /guide demos"
```

---

### Task 10: Docs, CSS rebuild, and full verification

**Files:**
- Modify: `README.md`, `Dockerfile`
- Rebuild: `static/css/styles.css`

**Interfaces:**
- Produces: documented Stripe env + a clean build with the new view classes emitted.

- [ ] **Step 1: Document the env in README**

Add a "Billing" subsection to `README.md` (under the "Accounts & auth" section):

```markdown
## Billing (Stripe)

Subscriptions and one-time purchases via Stripe Checkout + Billing Portal. Config via env:

| Env | Purpose | Default |
|-----|---------|---------|
| `STRIPE_SECRET_KEY` | Stripe secret key (enables billing) | unset → billing disabled |
| `STRIPE_WEBHOOK_SECRET` | Webhook signing secret | unset → webhook rejects events |
| `STRIPE_PRICE_PRO` | Price ID for the `pro` subscription | unset → Subscribe hidden |
| `STRIPE_PRICE_EBOOK` | Price ID for the `ebook` one-time product | unset → Buy hidden |

Edit the `Products` catalog in `config/config.go` to add/rename products (each is
`{Key, Name, Kind, PriceID}`; `Kind` is `Subscription` or `OneTime`). Gating:
`/premium` requires an active subscription; `/guide` requires owning `ebook`.

**Local webhooks:** run `stripe listen --forward-to localhost:3000/webhooks/stripe`
and put the printed `whsec_...` in `STRIPE_WEBHOOK_SECRET`. Use Stripe **test mode**
keys/prices; the live consent round-trip can't be automated.
```

- [ ] **Step 2: Document env in the Dockerfile**

Extend the auth/email env comment block in `Dockerfile`:

```dockerfile
# Billing (set at runtime): STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET,
# STRIPE_PRICE_PRO, STRIPE_PRICE_EBOOK.
```

- [ ] **Step 3: Rebuild Tailwind CSS**

```bash
./bin/tailwind --input ./tailwind.input.css --output ./static/css/styles.css --minify
```
Expected: `static/css/styles.css` updates (the `@source` globs scan `./**/*.go`).

- [ ] **Step 4: Full verification**

```bash
go vet ./...
go test ./...
go build -ldflags="-X 'main.BuildEnv=debug'" -o /tmp/sapp .
```
Expected: vet clean, all tests pass, binary builds.

Optional live check (requires Stripe test-mode keys + `stripe listen`): with the
four envs set, confirm the Billing tab shows Subscribe/Buy, and that completing a
test checkout (card `4242 4242 4242 4242`) flips `/premium` (subscription) or
`/guide` (ebook) to accessible after the webhook arrives. Without keys, confirm
the Billing tab shows "Billing is not configured" and `/premium` 404s.

- [ ] **Step 5: Commit**

```bash
git add README.md Dockerfile static/css/styles.css
git commit -m "docs: document Stripe billing env and dev setup; rebuild CSS"
```

---

## Self-Review

**Spec coverage:**
- `stripe_customer_id` + subscription fields + `entitlements` table + store methods → Tasks 1, 2 ✓
- Stripe creds + `Products` catalog + `StripeEnabled`/`ProductByKey` → Task 3 ✓
- `billing/` wraps stripe-go (Checkout/Portal/Customer + normalized webhook verifier) + fake → Task 4 ✓
- `RequireSubscription` + `RequireEntitlement` + `HasActiveSubscription` → Task 5 ✓
- Subscribe/Buy (mode per kind) + Portal, server-created sessions, customer ensured → Task 6 ✓
- Webhook = source of truth; checkout.session.completed (grant one-time / link subscription by catalog Kind), subscription.updated/deleted; signature-verified; idempotent → Task 7 ✓
- Billing tab (subscribe/manage/buy/owned) + `StripeEnabled` stub fallback → Task 8 ✓
- Gating demos `/premium` + `/guide`, registered only when enabled → Task 9 ✓
- Docs + manual `stripe listen` note + CSS rebuild → Task 10 ✓
- One new dependency, contained in `billing/` → Task 4 ✓
- One Customer per user shared; entitlements permanent; fail-closed gating; no trial; mandatory webhook verification → Tasks 1/4/5/7 ✓

**Placeholder scan:** Every code step has complete code; every test step has real assertions. The only deliberate variable is the stripe-go `vNN` import suffix, which Task 4 Step 1 pins to the resolved major version — a concrete instruction, not a placeholder.

**Type consistency:** `database.User.{StripeCustomerID,SubscriptionStatus,CurrentPeriodEnd}`, `SetStripeCustomerID`/`UserByStripeCustomerID`/`UpdateSubscription`, `GrantEntitlement`/`HasEntitlement`/`ListEntitlements`; `config.{ProductKind,Product,Subscription,OneTime,StripeEnabled,ProductByKey,StripeSecretKey,StripeWebhookSecret,Products}`; `billing.{Client,StripeClient,FakeClient,New,CheckoutInput,Event,EnsureCustomer,CreateCheckoutSession,CreatePortalSession,VerifyWebhook}`; `auth.{HasActiveSubscription,RequireSubscription,RequireEntitlement}`; `views.{BillingView,Billing,Premium,Guide}`; handlers `HandleCheckout`/`HandlePortal`/`HandleStripeWebhook`/`HandleBilling`/`HandlePremium`/`HandleGuide` — used consistently across tasks.

**Notes for the implementer:**
- Task 8 changes `views.Billing` arity and `HandleBilling`'s signature (was `HandleBilling()` from #1); update the single `main.go` registration in the same task.
- Only `billing/` may import stripe-go; everything else depends on the `billing.Client` interface (so handler tests use `billing.FakeClient`).
- The migration is `0004_billing.sql`; on this branch (off `accounts`, no #3) there is no `0003` file — the runner applies any unapplied migration regardless of the numbering gap.
- The webhook handler maps payment-vs-subscription off the catalog `Kind` (`Conf.ProductByKey(event.ProductKey)`), not the Stripe `mode` — matching the spec.
