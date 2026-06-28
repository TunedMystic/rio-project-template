package views

import (
	"strconv"

	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

func Home(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		// Hero — the page's thesis: a clean, ready-to-build starting point.
		dom.Section(
			dom.Class("py-16 sm:py-20"),
			shell(
				eyebrow("Project template"),
				dom.H1(
					dom.Class("mt-3 max-w-2xl text-4xl sm:text-5xl [font-weight:var(--font-weight-heading)] tracking-tight leading-[1.1] text-[var(--color-text)]"),
					dom.Text("A clean starting point for your next app."),
				),
				dom.P(
					dom.Class("mt-5 max-w-xl text-[length:var(--font-size-lg)] leading-relaxed text-[var(--color-text-muted)]"),
					dom.Text("Server-rendered with rio/dom, themed with rio/ui, and backed by SQLite. Clone it, set your brand in one file, and start building."),
				),
				dom.Div(
					dom.Class("mt-8 flex flex-wrap items-center gap-x-6 gap-y-3"),
					ui.ButtonLink(ui.ButtonPrimary, "/messages", "Explore the demo"),
					ghostLink("/about", "Read about it"),
				),
			),
		),

		// Get started — the signature stepper, housed in a bordered card.
		dom.Section(
			dom.Class("pb-4"),
			shell(
				card(
					ruledHeading("Get started in three steps"),
					dom.Div(
						dom.Class("mt-8"),
						steps(
							step{"1", "Clone", "Fork the template and open it."},
							step{"2", "Configure", "Set ProjectName and your brand tokens."},
							step{"3", "Build", "Run it and start shipping."},
						),
					),
				),
			),
		),

		// What's inside — the feature rows.
		dom.Section(
			dom.Class("py-12"),
			shell(
				sectionLabel("What's inside"),
				dom.Div(
					dom.Class("mt-5 grid gap-3 sm:grid-cols-2"),
					featureRow("message", "Live SQLite demo", "Post a message and watch it persist.", "/messages"),
					featureRow("layers", "Server-rendered UI", "Built with rio/dom and rio/ui — no templates.", "/about"),
					featureRow("database", "Built-in migrations", "Embedded SQL runs at startup, forward-only.", "/about"),
					featureRow("check", "One dependency", "Just modernc.org/sqlite beyond rio.", "/about"),
				),
			),
		),
	)
}

func About(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		pageHeader("About", "What this template is — and how to make it yours."),
		dom.Section(
			dom.Class("py-12"),
			shell(
				dom.Div(
					dom.Class("max-w-2xl space-y-5"),
					ui.Text(ui.TextDefault, "This is a small, opinionated starting point for building products with rio. It renders pages as Go — no HTML templates — themes them with rio/ui components, and persists data in SQLite with migrations that run on startup."),
					ui.Text(ui.TextDefault, "To make it yours, open config/config.go: set ProjectName, edit the brand tokens, and reshape these pages. Delete the messages demo whenever you're ready."),
				),
				dom.Div(
					dom.Class("mt-10 max-w-2xl"),
					ruledHeading("What you get"),
					dom.Ul(
						dom.Class("mt-5 grid gap-3 sm:grid-cols-2"),
						checkItem("Server-rendered with rio/dom"),
						checkItem("Themed rio/ui components"),
						checkItem("SQLite with built-in migrations"),
						checkItem("One dependency beyond rio"),
						checkItem("Minimal scratch Docker image"),
						checkItem("Tailwind v4 styling"),
					),
				),
			),
		),
	)
}

func PrivacyPolicy(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		pageHeader("Privacy Policy", "Replace this with your product's privacy policy."),
		dom.Section(
			dom.Class("py-12"),
			shell(
				dom.Div(
					dom.Class("max-w-2xl space-y-5"),
					ui.Text(ui.TextDefault, "This placeholder describes how a product might collect and use information. Swap it for your real policy before you ship."),
				),
			),
		),
	)
}

func NotFound(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		dom.Section(
			dom.Class("py-24"),
			shell(
				dom.Div(
					dom.Class("max-w-xl"),
					eyebrow("Error 404"),
					dom.H1(
						dom.Class("mt-3 text-4xl sm:text-5xl [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
						dom.Text("This page wandered off."),
					),
					dom.P(
						dom.Class("mt-4 text-[length:var(--font-size-lg)] text-[var(--color-text-muted)]"),
						dom.Text("The page you're looking for doesn't exist or has moved."),
					),
					dom.Div(dom.Class("mt-8"), ui.ButtonLink(ui.ButtonPrimary, "/", "Back home")),
				),
			),
		),
	)
}

func Messages(pd config.PageData, meta config.Meta, msgs []database.Message, bodyValue, bodyErr string) dom.Node {
	return Page(pd, meta,
		pageHeader("Messages", "A SQLite-backed demo. Add a message and it persists across restarts."),
		dom.Section(
			dom.Class("py-12"),
			shell(
				dom.Div(
					dom.Class("grid items-start gap-6 lg:grid-cols-[5fr_3fr]"),

					// Left: the form, then the recent messages.
					dom.Div(
						card(
							ruledHeading("Add a message"),
							dom.Form(
								dom.Method("post"),
								dom.Action("/messages"),
								dom.Class("mt-6"),
								ui.TextField("body", "Message", bodyValue, bodyErr),
								submitButton("Add message"),
							),
						),
						messagesList(msgs),
					),

					// Right: a summary card echoing the checkout "order summary".
					card(
						ruledHeading("About this demo"),
						dom.Div(
							dom.Class("mt-2"),
							summaryRow("Storage", "SQLite"),
							summaryRow("Driver", "modernc (cgo-free)"),
							summaryRow("Persistence", "Across restarts"),
							summaryRow("Rendering", "rio/dom"),
							summaryTotal("Messages stored", strconv.Itoa(len(msgs))),
						),
					),
				),
			),
		),
	)
}

// --- small page-level building blocks ---

// card is a crisp white panel defined by a thin border — the Northwest surface.
func card(children ...dom.Node) dom.Node {
	return dom.Div(withClass("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-sm sm:p-7", children)...)
}

// eyebrow is a short accent label that sits above a heading.
func eyebrow(text string) dom.Node {
	return dom.Span(
		dom.Class("text-[length:var(--font-size-sm)] font-semibold uppercase tracking-wider text-[var(--color-primary)]"),
		dom.Text(text),
	)
}

// sectionLabel is a quiet uppercase label introducing a group of content.
func sectionLabel(text string) dom.Node {
	return dom.H2(
		dom.Class("text-[length:var(--font-size-sm)] font-semibold uppercase tracking-wider text-[var(--color-text-muted)]"),
		dom.Text(text),
	)
}

// ghostLink is a borderless text link with a nudging arrow, for secondary CTAs.
func ghostLink(href, label string) dom.Node {
	return dom.A(
		dom.Class("group inline-flex items-center gap-1.5 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text)] transition-colors hover:text-[var(--color-primary)]"),
		dom.Href(href),
		dom.Text(label),
		dom.Span(dom.Class("transition-transform group-hover:translate-x-0.5"), icon("arrow-right", 16)),
	)
}

// checkItem is a single ticked line in a feature list.
func checkItem(text string) dom.Node {
	return dom.Li(
		dom.Class("flex items-start gap-3"),
		dom.Span(
			dom.Class("mt-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[var(--color-primary)]/10 text-[var(--color-primary)]"),
			icon("check", 13),
		),
		dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(text)),
	)
}

// step is one numbered stage in the getting-started stepper.
type step struct{ n, title, desc string }

// steps lays the stages out horizontally with connecting rules.
func steps(items ...step) dom.Node {
	nodes := make([]dom.Node, 0, len(items)+1)
	nodes = append(nodes, dom.Class("flex items-start"))
	for i, s := range items {
		nodes = append(nodes, stepNode(s, i == 0, i == len(items)-1))
	}
	return dom.Div(nodes...)
}

func stepNode(s step, first, last bool) dom.Node {
	line := "h-0.5 flex-1 bg-[var(--color-primary)]/30"
	left, right := line, line
	if first {
		left = "h-0.5 flex-1 bg-transparent"
	}
	if last {
		right = "h-0.5 flex-1 bg-transparent"
	}
	return dom.Div(
		dom.Class("flex-1"),
		dom.Div(
			dom.Class("flex items-center"),
			dom.Div(dom.Class(left)),
			dom.Div(
				dom.Class("flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[var(--color-primary)] text-[var(--color-on-primary)] text-[length:var(--font-size-sm)] font-semibold"),
				dom.Text(s.n),
			),
			dom.Div(dom.Class(right)),
		),
		dom.Div(
			dom.Class("mt-3 px-2 text-center"),
			dom.Div(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(s.title)),
			dom.Div(dom.Class("mt-1 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(s.desc)),
		),
	)
}

// summaryRow is a label/value line in the summary card.
func summaryRow(label, value string) dom.Node {
	return dom.Div(
		dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-3"),
		dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(label)),
		dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text(value)),
	)
}

// summaryTotal is the highlighted bottom row of the summary card.
func summaryTotal(label, value string) dom.Node {
	return dom.Div(
		dom.Class("mt-3 flex items-center justify-between rounded-[var(--radius-base)] bg-[var(--color-background)] px-4 py-3"),
		dom.Span(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(label)),
		dom.Span(dom.Class("text-[length:var(--font-size-lg)] font-bold text-[var(--color-primary)]"), dom.Text(value)),
	)
}

func messagesList(msgs []database.Message) dom.Node {
	if len(msgs) == 0 {
		return dom.Div(
			dom.Class("mt-6 rounded-[var(--radius-base)] border border-dashed border-[var(--color-border)] p-8 text-center"),
			dom.P(dom.Class("text-[var(--color-text-muted)]"), dom.Text("No messages yet. Add the first one above.")),
		)
	}

	items := make([]dom.Node, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, messageCard(m))
	}
	return dom.Div(
		dom.Class("mt-8"),
		sectionLabel("Recent"),
		dom.Div(withClass("mt-4 space-y-3", items)...),
	)
}

func messageCard(m database.Message) dom.Node {
	body := []dom.Node{
		dom.P(dom.Class("text-[var(--color-text)]"), dom.Text(m.Body)),
	}
	if !m.CreatedAt.IsZero() {
		body = append(body, dom.P(
			dom.Class("mt-1 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Text(m.CreatedAt.Format("Jan 2, 2006 · 3:04 PM")),
		))
	}
	return dom.Div(withClass("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-4 shadow-sm", body)...)
}
