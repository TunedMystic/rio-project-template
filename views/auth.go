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

func Login(pd config.PageData, meta config.Meta, email, errMsg, next string) dom.Node {
	return Page(pd, meta,
		authCard(
			authHeading("Log in"),
			// Non-breaking hyphen keeps "sign-in" from splitting across lines.
			authText("Enter your email to receive a sign‑in link."),
			dom.Form(
				dom.Method("post"),
				dom.Action("/login"),
				dom.Class("mt-8"),
				dom.Input(dom.Type("hidden"), dom.Name("next"), dom.Value(next)),
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
		),
	)
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
