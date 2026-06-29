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
