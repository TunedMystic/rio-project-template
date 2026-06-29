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

func Login(pd config.PageData, meta config.Meta, email, errMsg, next string) dom.Node {
	return Page(pd, meta,
		authCard(
			ruledHeading("Log in"),
			dom.P(
				dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("Enter your email and we'll send you a magic link."),
			),
			dom.Form(
				dom.Method("post"),
				dom.Action("/login"),
				dom.Class("mt-6"),
				dom.Input(dom.Type("hidden"), dom.Name("next"), dom.Value(next)),
				ui.TextField("email", "Email address", email, errMsg),
				submitButton("Send magic link"),
			),
		),
	)
}

func LoginSent(pd config.PageData, meta config.Meta, email string) dom.Node {
	return Page(pd, meta,
		authCard(
			ruledHeading("Check your email"),
			dom.P(
				dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("If an account exists for "+email+", a magic link is on its way. The link expires in 15 minutes."),
			),
			dom.Div(
				dom.Class("mt-6"),
				ghostLink("/login", "Use a different email"),
			),
		),
	)
}

func VerifyError(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		authCard(
			ruledHeading("Link expired"),
			dom.P(
				dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("That magic link is invalid or has already been used. Request a fresh one."),
			),
			dom.Div(
				dom.Class("mt-6"),
				ui.ButtonLink(ui.ButtonPrimary, "/login", "Back to log in"),
			),
		),
	)
}
