// billing/fake.go
package billing

import "context"

// FakeClient is an in-memory Client for tests.
type FakeClient struct {
	CustomerID     string // returned by EnsureCustomer when existingID == ""
	CheckoutURL    string
	PortalURL      string
	CreatedCust    bool
	LastCheckout   CheckoutInput
	NextEvent      Event
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
