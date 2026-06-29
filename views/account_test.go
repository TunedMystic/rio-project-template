// views/account_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func TestProfile_RendersTabsAndForm(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "profile", CSRF: "csrf-token", Flash: "Saved"}
	var b bytes.Buffer
	_ = Profile(pd, config.Meta{Title: "Profile"}, av, "Sam", "sam@example.com").Render(&b)
	html := b.String()
	for _, want := range []string{
		`href="/account"`,          // profile tab
		`href="/account/security"`, // security tab
		`href="/account/billing"`,  // billing tab
		`href="/account/delete"`,   // danger tab
		`value="csrf-token"`,       // hidden CSRF
		`value="Sam"`,              // editable name
		"sam@example.com",          // email shown
		"Saved",                    // flash
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Profile missing %q", want)
		}
	}
}

func TestSecurity_ListsSessions(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "security", CSRF: "c"}
	sessions := []database.Session{{ID: "cur", IP: "1.1.1.1"}, {ID: "other", IP: "2.2.2.2"}}
	var b bytes.Buffer
	_ = Security(pd, config.Meta{Title: "Security"}, av, sessions, "cur").Render(&b)
	html := b.String()
	if !strings.Contains(html, "This device") {
		t.Error("current session not marked")
	}
	if !strings.Contains(html, `action="/account/sessions/revoke-all"`) {
		t.Error("missing sign-out-everywhere form")
	}
}

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
		`value="pro"`,                        // subscribe button carries the product key
		`value="ebook"`,                      // buy button carries the product key
		"E-book",                             // product name shown
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Billing missing %q", want)
		}
	}

	// Subscribed → Manage (portal); owned one-time → "Owned".
	bv.Status = "active"
	bv.HasCustomer = true
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

	// past_due with a Stripe customer → Manage (portal), NOT a second Subscribe form.
	bv3 := BillingView{
		StripeEnabled: true,
		Products: []config.Product{
			{Key: "pro", Name: "Pro", Kind: config.Subscription, PriceID: "price_pro"},
		},
		Status:      "past_due",
		HasCustomer: true,
		Owned:       map[string]bool{},
	}
	var b3 bytes.Buffer
	_ = Billing(pd, config.Meta{Title: "Billing"}, av, bv3).Render(&b3)
	html3 := b3.String()
	if !strings.Contains(html3, `action="/account/billing/portal"`) {
		t.Error("past_due subscriber should see Manage (portal) form")
	}
	if strings.Contains(html3, `value="pro"`) {
		t.Error("past_due subscriber should NOT see a Subscribe checkout form for the pro product")
	}
}
