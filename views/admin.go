package views

import (
	"fmt"
	"net/url"

	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// adminShell wraps admin page content with the header and a breadcrumb trail.
func adminShell(pd config.PageData, meta config.Meta, section string, body ...dom.Node) dom.Node {
	trail := []config.Link{
		{Text: "Home", Href: "/"},
		{Text: "Admin", Href: "/admin/users"},
		{Text: section, Href: "#"},
	}
	content := append([]dom.Node{breadcrumbs(trail)}, body...)
	return Page(pd, meta,
		pageHeader("Admin", "Operator tools — users and subscriptions."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("space-y-6", content)...))),
	)
}

// subStatusBadge maps a subscription status to a colored badge.
func subStatusBadge(status string) dom.Node {
	variant := ui.BadgeNeutral
	label := status
	switch status {
	case "active", "trialing":
		variant = ui.BadgeSuccess
	case "past_due":
		variant = ui.BadgeWarning
	case "canceled":
		variant = ui.BadgeDanger
	}
	if label == "" {
		label = "none"
	}
	return ui.Badge(variant, label)
}

// AdminUsers renders the searchable, paginated user list (the admin landing page).
func AdminUsers(pd config.PageData, meta config.Meta, query string, users []database.User, page, numPages int) dom.Node {
	search := dom.Form(
		dom.Method("get"), dom.Action("/admin/users"),
		dom.Class("flex gap-2"),
		dom.Input(
			dom.Type("search"), dom.Name("q"), dom.Value(query),
			dom.Placeholder("Search by email"),
			dom.Class("w-full max-w-sm rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-[length:var(--font-size-sm)] text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
		),
		submitButton("Search"),
	)

	head := dom.Tr(
		dom.Class("border-b border-[var(--color-border)]"),
		adminTh("Email"), adminTh("Name"), adminTh("Joined"), adminTh("Subscription"),
	)
	rows := make([]dom.Node, 0, len(users)+1)
	rows = append(rows, head)
	for _, u := range users {
		rows = append(rows, dom.Tr(
			dom.Class("border-b border-[var(--color-border)] last:border-0"),
			dom.Td(dom.Class("px-4 py-3 text-[length:var(--font-size-sm)]"),
				dom.A(
					dom.Class("font-medium text-[var(--color-primary)] hover:underline"),
					dom.Href(fmt.Sprintf("/admin/users/%d", u.ID)),
					dom.Text(u.Email),
				),
			),
			adminTd(u.Name),
			adminTd(u.CreatedAt.Format("2006-01-02")),
			dom.Td(dom.Class("px-4 py-3"), subStatusBadge(u.SubscriptionStatus)),
		))
	}

	table := dom.Div(
		dom.Class("overflow-hidden rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)]"),
		dom.Div(dom.Class("overflow-x-auto"),
			dom.Table(dom.Class("w-full border-collapse"),
				dom.Thead(head), dom.Tbody(rows[1:]...))),
	)

	base := "/admin/users"
	if query != "" {
		base = "/admin/users?q=" + url.QueryEscape(query)
	}

	body := []dom.Node{search, table}
	if numPages > 1 {
		body = append(body, pagination(page, numPages, base))
	}
	if len(users) == 0 {
		body = []dom.Node{search, emptyState("layers", "No users", "No users match this search.", nil)}
	}
	return adminShell(pd, meta, "Users", body...)
}

func adminTh(label string) dom.Node {
	return dom.Th(
		dom.Class("px-4 py-3 text-left text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text-muted)]"),
		dom.Text(label),
	)
}

func adminTd(text string) dom.Node {
	return dom.Td(
		dom.Class("px-4 py-3 text-[length:var(--font-size-sm)] text-[var(--color-text)]"),
		dom.Text(text),
	)
}

// AdminUserView is the data the user-detail page needs.
type AdminUserView struct {
	User         database.User
	Entitlements []string
	Sessions     []database.Session
	Products     []config.Product
	CSRF         string
	Flash        string
}

// AdminUserDetail renders one user's profile, entitlements, and sessions with
// safe admin actions.
func AdminUserDetail(pd config.PageData, meta config.Meta, v AdminUserView) dom.Node {
	u := v.User
	body := []dom.Node{}
	if v.Flash != "" {
		body = append(body, ui.Alert(ui.AlertSuccess, dom.Text(v.Flash)))
	}

	// Profile key-values.
	google := "no"
	if u.GoogleID != "" {
		google = "yes"
	}
	periodEnd := "—"
	if !u.CurrentPeriodEnd.IsZero() {
		periodEnd = u.CurrentPeriodEnd.Format("2006-01-02")
	}
	profile := card(
		ruledHeading("Profile"),
		dom.Div(dom.Class("mt-4"),
			kv("ID", fmt.Sprintf("%d", u.ID)),
			kv("Email", u.Email),
			kv("Name", u.Name),
			kv("Joined", u.CreatedAt.Format("2006-01-02 15:04")),
			kv("Google linked", google),
			kv("Stripe customer", orDash(u.StripeCustomerID)),
			kv("Subscription", orDash(u.SubscriptionStatus)),
			kv("Current period end", periodEnd),
		),
	)

	// Entitlements: current list with per-item revoke, plus a grant form.
	entItems := make([]dom.Node, 0, len(v.Entitlements))
	for _, key := range v.Entitlements {
		entItems = append(entItems, dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-2"),
			dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text)]"), dom.Text(key)),
			adminActionForm(fmt.Sprintf("/admin/users/%d/entitlements/revoke", u.ID), v.CSRF,
				dom.Input(dom.Type("hidden"), dom.Name("product_key"), dom.Value(key)),
				submitButton("Revoke"),
			),
		))
	}
	if len(entItems) == 0 {
		entItems = append(entItems, dom.P(dom.Class("py-2 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("No entitlements.")))
	}
	opts := make([]ui.Option, 0, len(v.Products))
	for _, p := range v.Products {
		opts = append(opts, ui.Option{Value: p.Key, Label: p.Name})
	}
	grant := adminActionForm(fmt.Sprintf("/admin/users/%d/entitlements/grant", u.ID), v.CSRF,
		dom.Div(dom.Class("flex items-end gap-2"),
			selectField("product_key", "Grant product", "", opts, true),
			submitButton("Grant"),
		),
	)
	entitlements := card(ruledHeading("Entitlements"), dom.Div(dom.Class("mt-4 space-y-3"), dom.Div(entItems...), grant))

	// Sessions with a revoke-all action.
	sessItems := make([]dom.Node, 0, len(v.Sessions))
	for _, sess := range v.Sessions {
		sessItems = append(sessItems, dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-2 text-[length:var(--font-size-sm)]"),
			dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(deviceLabel(sess.UserAgent))),
			dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(sess.IP)),
		))
	}
	if len(sessItems) == 0 {
		sessItems = append(sessItems, dom.P(dom.Class("py-2 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("No active sessions.")))
	}
	sessions := card(
		ruledHeading("Sessions"),
		dom.Div(dom.Class("mt-4 space-y-3"),
			dom.Div(sessItems...),
			adminActionForm(fmt.Sprintf("/admin/users/%d/sessions/revoke", u.ID), v.CSRF,
				submitButton("Revoke all sessions"),
			),
		),
	)

	body = append(body, profile, entitlements, sessions)
	return adminShell(pd, meta, u.Email, body...)
}

// kv renders a label/value row.
func kv(label, value string) dom.Node {
	return dom.Div(
		dom.Class("flex justify-between gap-4 border-b border-[var(--color-border)] py-2 text-[length:var(--font-size-sm)] last:border-0"),
		dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(label)),
		dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(value)),
	)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// adminActionForm builds a POST form with a CSRF field and the given controls.
func adminActionForm(action, csrf string, controls ...dom.Node) dom.Node {
	children := []dom.Node{dom.Method("post"), dom.Action(action), csrfInput(csrf)}
	children = append(children, controls...)
	return dom.Form(children...)
}
