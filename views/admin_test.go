package views

import (
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func TestAdminUsers_RendersRowsAndSearch(t *testing.T) {
	users := []database.User{
		{ID: 7, Email: "alice@example.com", Name: "Alice", SubscriptionStatus: "active"},
		{ID: 8, Email: "bob@example.com", Name: "Bob"},
	}
	html := render(AdminUsers(testPageData(), config.Meta{Title: "Users"}, "ali", users, 1, 3))
	if !strings.Contains(html, `href="/admin/users/7"`) {
		t.Error("missing link to user detail")
	}
	if !strings.Contains(html, "alice@example.com") || !strings.Contains(html, "bob@example.com") {
		t.Error("missing user rows")
	}
	if !strings.Contains(html, `name="q"`) || !strings.Contains(html, `value="ali"`) {
		t.Error("missing search box preserving query")
	}
	if !strings.Contains(html, `aria-label="Pagination"`) {
		t.Error("missing pagination")
	}
	// Pagination links must compose the query correctly: a single separator
	// between the existing ?q= param and the &page= param. The dom library
	// HTML-escapes & to &amp; inside attribute values.
	if !strings.Contains(html, `href="/admin/users?q=ali&amp;page=2"`) {
		t.Errorf("page-2 link missing or has wrong separator; want q=ali&amp;page=2 in href, got:\n%s", html)
	}
	// Regression guard: broken double-? must not appear.
	if strings.Contains(html, "?q=ali&amp;?page") || strings.Contains(html, "??") {
		t.Error("pagination link has double-? separator (broken URL composition)")
	}
}

func TestAdminUserDetail_RendersActions(t *testing.T) {
	v := AdminUserView{
		User:         database.User{ID: 7, Email: "alice@example.com", Name: "Alice", SubscriptionStatus: "active"},
		Entitlements: []string{"ebook"},
		Sessions:     []database.Session{{ID: "s1", UserAgent: "ua", IP: "1.2.3.4"}},
		Products:     []config.Product{{Key: "ebook", Name: "E-book"}, {Key: "pro", Name: "Pro"}},
		CSRF:         "tok",
		Flash:        "Granted ebook",
	}
	html := render(AdminUserDetail(testPageData(), config.Meta{Title: "User"}, v))
	for _, want := range []string{
		"alice@example.com",
		`value="tok"`,                                        // csrf input
		`action="/admin/users/7/entitlements/grant"`,         // grant form
		`action="/admin/users/7/entitlements/revoke"`,        // revoke form
		`action="/admin/users/7/sessions/revoke"`,            // revoke sessions
		"Granted ebook",                                       // flash
	} {
		if !strings.Contains(html, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
}
