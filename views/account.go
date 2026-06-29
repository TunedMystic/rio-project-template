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
	Error  string
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
	if av.Error != "" {
		content = append(content, ui.Alert(ui.AlertError, dom.Text(av.Error)))
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

// deviceBadge is a single-line success pill marking the current session.
func deviceBadge(label string) dom.Node {
	return dom.Span(
		dom.Class("inline-flex shrink-0 items-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-[length:var(--font-size-sm)] font-medium ring-1 ring-inset bg-[var(--color-success)]/12 text-[var(--color-success)] ring-[var(--color-success)]/25"),
		dom.Text(label),
	)
}

// loginMethodsCard shows the account's sign-in methods with Google
// connect/disconnect controls.
func loginMethodsCard(pd config.PageData, av AccountView, googleLinked bool) dom.Node {
	googleRow := dom.Node(dom.Text(""))
	switch {
	case googleLinked:
		googleRow = dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-4 last:border-0"),
			dom.Div(dom.Class("min-w-0"),
				dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text("Google")),
				dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Connected")),
			),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/google/disconnect"),
				csrfInput(av.CSRF),
				dom.Button(dom.Type("submit"),
					dom.Class("shrink-0 rounded-[var(--radius-base)] border border-[var(--color-border)] px-3 py-1.5 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] transition hover:border-[var(--color-danger)] hover:text-[var(--color-danger)] cursor-pointer"),
					dom.Text("Disconnect")),
			),
		)
	case pd.GoogleEnabled:
		googleRow = dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-4 last:border-0"),
			dom.Div(dom.Class("min-w-0"),
				dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text("Google")),
				dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Not connected")),
			),
			dom.A(
				dom.Class("shrink-0 rounded-[var(--radius-base)] border border-[var(--color-border)] px-3 py-1.5 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text)] transition hover:border-[var(--color-primary)] hover:text-[var(--color-primary)] cursor-pointer"),
				dom.Href("/auth/google/login?mode=link"),
				dom.Text("Connect")),
		)
	}

	return card(
		ruledHeading("Login methods"),
		dom.Div(
			dom.Class("mt-2"),
			dom.Div(
				dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-4"),
				dom.Div(dom.Class("min-w-0"),
					dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text("Email magic link")),
					dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Always available")),
				),
			),
			googleRow,
		),
	)
}

func Security(pd config.PageData, meta config.Meta, av AccountView, sessions []database.Session, currentID string, googleLinked bool) dom.Node {
	rows := make([]dom.Node, 0, len(sessions))
	for _, s := range sessions {
		location := s.IP
		if location == "" {
			location = "Unknown location"
		}

		heading := []dom.Node{
			dom.Class("flex items-center gap-2"),
			dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text(deviceLabel(s.UserAgent))),
		}
		action := dom.Node(dom.Text(""))
		if s.ID == currentID {
			heading = append(heading, deviceBadge("This device"))
		} else {
			action = dom.Form(
				dom.Method("post"),
				dom.Action("/account/sessions/revoke"),
				csrfInput(av.CSRF),
				dom.Input(dom.Type("hidden"), dom.Name("id"), dom.Value(s.ID)),
				dom.Button(dom.Type("submit"),
					dom.Class("shrink-0 rounded-[var(--radius-base)] border border-[var(--color-border)] px-3 py-1.5 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] transition hover:border-[var(--color-danger)] hover:text-[var(--color-danger)] cursor-pointer"),
					dom.Text("Sign out")),
			)
		}

		rows = append(rows, dom.Div(
			dom.Class("flex items-center justify-between gap-4 border-b border-[var(--color-border)] py-4 last:border-0"),
			dom.Div(
				dom.Class("min-w-0"),
				dom.Div(heading...),
				dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(location)),
			),
			action,
		))
	}

	body := []dom.Node{
		ruledHeading("Active sessions"),
		dom.P(
			dom.Class("mt-3 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Text("Devices currently signed in to your account."),
		),
		dom.Div(withClass("mt-2", rows)...),
	}
	if len(sessions) > 1 {
		body = append(body,
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/sessions/revoke-all"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				dom.Button(dom.Type("submit"),
					dom.Class("inline-flex items-center justify-center rounded-[var(--radius-base)] border border-[var(--color-border)] px-4 py-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text)] transition hover:border-[var(--color-danger)] hover:text-[var(--color-danger)] cursor-pointer"),
					dom.Text("Sign out everywhere else")),
			),
		)
	}

	return accountShell(pd, meta, av, card(body...), loginMethodsCard(pd, av, googleLinked))
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
