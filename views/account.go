// views/account.go
package views

import (
	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// AccountView is the per-request chrome state for the account area.
type AccountView struct {
	Active string // "profile" | "security" | "billing" | "danger"
	CSRF   string
	Flash  string
}

type accountTab struct{ key, label, href string }

var accountTabs = []accountTab{
	{"profile", "Profile", "/account"},
	{"security", "Security", "/account/security"},
	{"billing", "Billing", "/account/billing"},
	{"danger", "Danger", "/account/delete"},
}

// accountShell wraps a tab's content with the page header, flash, and tab nav.
func accountShell(pd config.PageData, meta config.Meta, av AccountView, body ...dom.Node) dom.Node {
	tabs := make([]dom.Node, 0, len(accountTabs)+1)
	tabs = append(tabs, dom.Class("flex gap-2 border-b border-[var(--color-border)]"))
	for _, tb := range accountTabs {
		cls := "px-4 py-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
		if tb.key == av.Active {
			cls = "px-4 py-2 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-primary)] border-b-2 border-[var(--color-primary)] -mb-px"
		}
		tabs = append(tabs, dom.A(dom.Class(cls), dom.Href(tb.href), dom.Text(tb.label)))
	}

	content := make([]dom.Node, 0, len(body)+2)
	if av.Flash != "" {
		content = append(content, ui.Alert(ui.AlertSuccess, dom.Text(av.Flash)))
	}
	content = append(content, dom.Nav(tabs...))
	content = append(content, body...)

	return Page(pd, meta,
		pageHeader("Account", "Manage your profile, security, and billing."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("max-w-2xl space-y-6", content)...))),
	)
}

func csrfInput(token string) dom.Node {
	return dom.Input(dom.Type("hidden"), dom.Name("_csrf"), dom.Value(token))
}

func Profile(pd config.PageData, meta config.Meta, av AccountView, name, email string) dom.Node {
	return accountShell(pd, meta, av,
		card(
			ruledHeading("Profile"),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				ui.TextField("name", "Display name", name, ""),
				dom.Div(
					dom.Class("mb-4"),
					ui.Label("email_display", "Email"),
					dom.P(dom.Class("text-[var(--color-text-muted)]"), dom.Text(email)),
				),
				submitButton("Save changes"),
			),
		),
	)
}

func Security(pd config.PageData, meta config.Meta, av AccountView, sessions []database.Session, currentID string) dom.Node {
	rows := make([]dom.Node, 0, len(sessions))
	for _, s := range sessions {
		meta := s.IP
		if s.UserAgent != "" {
			meta = s.UserAgent + " · " + s.IP
		}
		badge := dom.Node(dom.Text(""))
		if s.ID == currentID {
			badge = ui.Badge(ui.BadgeSuccess, "This device")
		}
		right := dom.Node(dom.Text(""))
		if s.ID != currentID {
			right = dom.Form(
				dom.Method("post"),
				dom.Action("/account/sessions/revoke"),
				csrfInput(av.CSRF),
				dom.Input(dom.Type("hidden"), dom.Name("id"), dom.Value(s.ID)),
				dom.Button(dom.Type("submit"),
					dom.Class("text-[length:var(--font-size-sm)] font-medium text-[var(--color-danger)] hover:underline cursor-pointer"),
					dom.Text("Sign out")),
			)
		}
		rows = append(rows, dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-3"),
			dom.Div(dom.Class("flex items-center gap-3"),
				dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(meta)), badge),
			right,
		))
	}

	return accountShell(pd, meta, av,
		card(
			ruledHeading("Active sessions"),
			dom.Div(withClass("mt-2", rows)...),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/sessions/revoke-all"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				submitButton("Sign out everywhere else"),
			),
		),
	)
}

func Billing(pd config.PageData, meta config.Meta, av AccountView) dom.Node {
	return accountShell(pd, meta, av,
		card(
			ruledHeading("Billing"),
			dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"), dom.Text("You're on the free plan.")),
			dom.Div(dom.Class("mt-4"),
				dom.Span(
					dom.Class("inline-flex items-center rounded-[var(--radius-base)] border border-[var(--color-border)] px-4 py-2 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
					dom.Text("Manage billing (coming soon)"),
				),
			),
		),
	)
}

func Danger(pd config.PageData, meta config.Meta, av AccountView, email string) dom.Node {
	return accountShell(pd, meta, av,
		card(
			ruledHeading("Delete account"),
			dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("This permanently deletes your account and all sessions. Type your email to confirm.")),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/delete"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				ui.TextField("confirm_email", "Confirm your email ("+email+")", "", ""),
				dom.Button(dom.Type("submit"),
					dom.Class("inline-flex items-center justify-center rounded-[var(--radius-base)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold bg-[var(--color-danger)] text-white shadow-sm hover:brightness-105 cursor-pointer"),
					dom.Text("Delete my account")),
			),
		),
	)
}
