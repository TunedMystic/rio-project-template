// handlers_billing_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"app/auth"
	"app/billing"
	"app/config"
	"app/database"
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

// TestHandleStripeWebhook_StoreError: store-error path (500 on write failure)
// is not tested here because *database.Store is a concrete type with no
// injectable interface; simulating a DB error would require closing the
// underlying DB, which would also break the FakeClient setup.  The behaviour
// is covered by the handler code change (rio.LogError + return err → 500).
func itoaTest(n int64) string { return strconv.FormatInt(n, 10) }

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
