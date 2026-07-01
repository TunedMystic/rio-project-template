// views/auth.go
package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// authCard centers a narrow card for the auth screens.
func authCard(children ...dom.Node) dom.Node {
	return dom.Section(
		dom.Class("py-16"),
		dom.Div(
			dom.Class("mx-auto w-full max-w-md px-5"),
			card(children...),
		),
	)
}

// authHeading is the centered title at the top of an auth card.
func authHeading(title string) dom.Node {
	return dom.H1(
		dom.Class("text-center text-[length:var(--font-size-2xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
		dom.Text(title),
	)
}

// authText is a centered, muted line of supporting copy for an auth card.
// text-balance evens the line lengths so a wrapped sentence doesn't look
// lopsided.
func authText(text string) dom.Node {
	return dom.P(
		dom.Class("mt-2 text-center text-balance text-[var(--color-text-muted)]"),
		dom.Text(text),
	)
}

// authSubmit is the full-width primary submit button used on the auth forms.
func authSubmit(label string) dom.Node {
	return dom.Button(
		dom.Type("submit"),
		dom.Class("inline-flex w-full items-center justify-center gap-2 rounded-[var(--radius-base)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold tracking-tight bg-[var(--color-primary)] text-[var(--color-on-primary)] shadow-sm transition hover:shadow-md hover:brightness-105 active:brightness-95 cursor-pointer"),
		dom.Text(label),
		icon("arrow-right", 18),
	)
}

// googleButton links to the Google OAuth login, styled as a neutral outline
// button with the Google "G" mark.
func googleButton() dom.Node {
	const g = `<svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.76h3.56c2.08-1.92 3.28-4.74 3.28-8.09z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.56-2.76c-.98.66-2.23 1.06-3.72 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23z"/><path fill="#FBBC05" d="M5.84 14.11a6.6 6.6 0 0 1 0-4.22V7.05H2.18a11 11 0 0 0 0 9.9z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.05l3.66 2.84C6.71 7.31 9.14 5.38 12 5.38z"/></svg>`
	return dom.A(
		dom.Class("inline-flex w-full items-center justify-center gap-2 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text)] shadow-sm transition hover:shadow-md hover:brightness-[0.99] cursor-pointer"),
		dom.Href("/auth/google/login"),
		dom.Raw(g),
		dom.Text("Continue with Google"),
	)
}

// orDivider is a horizontal rule with a centered "or" label.
func orDivider() dom.Node {
	return dom.Div(
		dom.Class("my-6 flex items-center gap-3"),
		dom.Div(dom.Class("h-px flex-1 bg-[var(--color-border)]")),
		dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("or")),
		dom.Div(dom.Class("h-px flex-1 bg-[var(--color-border)]")),
	)
}

func Login(pd config.PageData, meta config.Meta, email, errMsg, next string) dom.Node {
	children := []dom.Node{
		authHeading("Log in"),
		// Non-breaking hyphen keeps "sign-in" from splitting across lines.
		authText("Enter your email to receive a sign‑in link."),
	}
	if pd.GoogleEnabled {
		children = append(children, dom.Div(dom.Class("mt-8"), googleButton()), orDivider())
	}
	children = append(children,
		dom.Form(
			dom.Method("post"),
			dom.Action("/login"),
			dom.Class(map[bool]string{true: "", false: "mt-8"}[pd.GoogleEnabled]),
			dom.Input(dom.Type("hidden"), dom.Name("next"), dom.Value(next)),
			Honeypot(),
			ui.TextField("email", "Email address", email, errMsg,
				dom.Placeholder("you@example.com"),
				dom.Autocomplete("email"),
				dom.Autofocus(),
			),
			authSubmit("Send login link"),
		),
		dom.Hr(dom.Class("my-6 border-[var(--color-border)]")),
		dom.P(
			dom.Class("text-center text-balance text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Text("We'll send you a magic link to sign in instantly. No password needed."),
		),
	)
	return Page(pd, meta, authCard(children...))
}

func LoginSent(pd config.PageData, meta config.Meta, email string) dom.Node {
	return Page(pd, meta,
		authCard(
			authHeading("Check your email"),
			authText("If an account exists for "+email+", a magic link is on its way. The link expires in 15 minutes."),
			dom.Div(
				dom.Class("mt-6 text-center"),
				ghostLink("/login", "Use a different email"),
			),
		),
	)
}

func VerifyError(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		authCard(
			authHeading("Link expired"),
			authText("That magic link is invalid or has already been used. Request a fresh one."),
			dom.Div(
				dom.Class("mt-6 text-center"),
				ui.ButtonLink(ui.ButtonPrimary, "/login", "Back to log in"),
			),
		),
	)
}
