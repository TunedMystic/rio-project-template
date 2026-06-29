// billing/billing.go
package billing

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	stripe "github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
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
	ID               string // Stripe event id (for idempotency/dedup)
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

// rawSubscription is used to extract fields that may not be in the
// stripe.Subscription struct for the pinned API version (e.g. current_period_end
// moved to subscription items in newer Stripe API versions, but test payloads
// may carry it at the top level).
type rawSubscription struct {
	Customer         interface{} `json:"customer"`
	Status           string      `json:"status"`
	CurrentPeriodEnd int64       `json:"current_period_end"`
	Items            struct {
		Data []struct {
			CurrentPeriodEnd int64 `json:"current_period_end"`
		} `json:"data"`
	} `json:"items"`
}

// VerifyWebhook validates the Stripe signature and normalizes the event.
func (c *StripeClient) VerifyWebhook(payload []byte, sigHeader, secret string) (Event, error) {
	ev, err := webhook.ConstructEventWithOptions(payload, sigHeader, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return Event{}, err
	}
	out := Event{ID: ev.ID, Type: string(ev.Type)}
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
		// Use rawSubscription to handle current_period_end regardless of
		// whether stripe-go's Subscription struct exposes it at the top level
		// or via items (API version dependent).
		var raw rawSubscription
		if err := json.Unmarshal(ev.Data.Raw, &raw); err != nil {
			return Event{}, err
		}
		// Extract customer ID: may be a string or an object with "id".
		switch v := raw.Customer.(type) {
		case string:
			out.CustomerID = v
		case map[string]interface{}:
			if id, ok := v["id"].(string); ok {
				out.CustomerID = id
			}
		}
		out.Status = raw.Status
		// Prefer top-level current_period_end; fall back to first item.
		periodEnd := raw.CurrentPeriodEnd
		if periodEnd == 0 && len(raw.Items.Data) > 0 {
			periodEnd = raw.Items.Data[0].CurrentPeriodEnd
		}
		if periodEnd != 0 {
			out.CurrentPeriodEnd = time.Unix(periodEnd, 0)
		}
	}
	return out, nil
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
