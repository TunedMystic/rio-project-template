package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
)

// Premium is the subscription-gated demo page.
func Premium(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		pageHeader("Premium", "Subscriber-only content."),
		dom.Section(dom.Class("py-12"), shell(
			card(
				ruledHeading("Pro members"),
				dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"),
					dom.Text("You're a Pro member — this page is gated by an active subscription.")),
			),
		)),
	)
}

// Guide is the entitlement-gated demo page (requires owning the "ebook").
func Guide(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		pageHeader("The Guide", "Your purchased digital product."),
		dom.Section(dom.Class("py-12"), shell(
			card(
				ruledHeading("Thanks for your purchase"),
				dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"),
					dom.Text("This guide is gated by a one-time purchase entitlement.")),
			),
		)),
	)
}
