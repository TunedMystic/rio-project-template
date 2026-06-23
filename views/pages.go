package views

import (
	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

func Home(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Welcome to "+pd.SiteName),
			ui.Text(ui.TextDefault, "A starter built with rio, rio/dom and rio/ui. Clone it, set your ProjectName and tokens, and start building."),
			ui.Text(ui.TextMuted, "The Messages page below is a live SQLite demo — delete it to drop the example."),
			ui.ButtonLink(ui.ButtonPrimary, "/messages", "Open the messages demo"),
		),
	)
}

func About(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "About"),
			ui.Text(ui.TextDefault, "Replace this page with your product's story."),
		),
	)
}

func PrivacyPolicy(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Privacy Policy"),
			ui.Text(ui.TextDefault, "Replace this page with your product's privacy policy."),
		),
	)
}

func NotFound(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Page not found"),
			ui.Text(ui.TextMuted, "That page does not exist."),
			ui.ButtonLink(ui.ButtonPrimary, "/", "Go home"),
		),
	)
}

func Messages(pd config.PageData, meta config.Meta, msgs []database.Message) dom.Node {
	items := make([]dom.Node, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, ui.Card(ui.Text(ui.TextDefault, m.Body)))
	}

	form := dom.Form(
		dom.Method("post"),
		dom.Action("/messages"),
		dom.Class("mb-8"),
		ui.TextField("body", "New message", "", ""),
		submitButton("Add message"),
	)

	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Messages"),
			ui.Text(ui.TextMuted, "A SQLite-backed demo: add a message and it persists."),
			form,
			ui.Stack(ui.GapMd, items...),
		),
	)
}
