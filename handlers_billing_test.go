// handlers_billing_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/auth"
	"app/billing"
	"app/config"
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
