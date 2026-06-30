package views

import (
	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// colorSwatches renders a grid of the theme's color tokens as labeled chips.
func colorSwatches() dom.Node {
	swatches := []struct{ label, varName string }{
		{"Primary", "--color-primary"},
		{"Secondary", "--color-secondary"},
		{"Surface", "--color-surface"},
		{"Surface raised", "--color-surface-raised"},
		{"Border", "--color-border"},
		{"Success", "--color-success"},
		{"Warning", "--color-warning"},
		{"Danger", "--color-danger"},
		{"Info", "--color-info"},
		{"Ring", "--color-ring"},
	}
	cells := make([]dom.Node, 0, len(swatches)+1)
	cells = append(cells, dom.Class("grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-5"))
	for _, s := range swatches {
		cells = append(cells, dom.Div(
			dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] overflow-hidden"),
			dom.Div(dom.Class("h-14 w-full"), dom.Style("background:var("+s.varName+")")),
			dom.Div(
				dom.Class("px-3 py-2 text-[length:var(--font-size-sm)]"),
				dom.Div(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(s.label)),
				dom.Div(dom.Class("text-[var(--color-text-muted)]"), dom.Text(s.varName)),
			),
		))
	}
	return dom.Div(cells...)
}

// typeScale renders one specimen line per font-size token.
func typeScale() dom.Node {
	steps := []struct{ size, label string }{
		{"--font-size-2xl", "font-size-2xl"},
		{"--font-size-xl", "font-size-xl"},
		{"--font-size-lg", "font-size-lg"},
		{"--font-size-base", "font-size-base"},
		{"--font-size-sm", "font-size-sm"},
	}
	rows := make([]dom.Node, 0, len(steps)+1)
	rows = append(rows, dom.Class("flex flex-col gap-3"))
	for _, s := range steps {
		rows = append(rows, dom.Div(
			dom.Class("flex items-baseline gap-4 border-b border-[var(--color-border)] pb-3"),
			dom.Span(
				dom.Class("tracking-tight text-[var(--color-text)]"),
				dom.Style("font-size:var("+s.size+")"),
				dom.Text("The quick brown fox"),
			),
			dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(s.label)),
		))
	}
	return dom.Div(rows...)
}

// buttonSet renders the four button variants from the ui kit.
func buttonSet() dom.Node {
	return dom.Div(
		dom.Class("flex flex-wrap items-center gap-3"),
		ui.Button(ui.ButtonPrimary, "Primary"),
		ui.Button(ui.ButtonSecondary, "Secondary"),
		ui.Button(ui.ButtonGhost, "Ghost"),
		ui.Button(ui.ButtonDanger, "Danger"),
	)
}

// pill renders a small rounded status label using a semantic color token.
func pill(label, colorVar string) dom.Node {
	return dom.Span(
		dom.Class("inline-flex items-center rounded-full px-2.5 py-0.5 text-[length:var(--font-size-sm)] font-medium"),
		dom.Style("background:color-mix(in srgb, var("+colorVar+") 14%, transparent);color:var("+colorVar+")"),
		dom.Text(label),
	)
}

// statusBadges renders the badge variants plus an Info pill (ui.Badge has no
// Info variant, so it is rendered as a token-driven pill).
func statusBadges() dom.Node {
	return dom.Div(
		dom.Class("flex flex-wrap items-center gap-3"),
		ui.Badge(ui.BadgeSuccess, "Success"),
		ui.Badge(ui.BadgeWarning, "Warning"),
		ui.Badge(ui.BadgeDanger, "Danger"),
		ui.Badge(ui.BadgeNeutral, "Neutral"),
		pill("Info", "--color-info"),
	)
}

// avatar renders a circular initial badge for a name.
func avatar(name string) dom.Node {
	return dom.Div(
		dom.Class("flex h-9 w-9 items-center justify-center rounded-full border-2 border-[var(--color-surface)] bg-[var(--color-primary)] text-[var(--color-on-primary)] text-[length:var(--font-size-sm)] font-semibold"),
		dom.Text(initial(name)),
	)
}

// avatarGroup renders overlapping avatars for a list of names.
func avatarGroup(names []string) dom.Node {
	items := make([]dom.Node, 0, len(names)+1)
	items = append(items, dom.Class("flex -space-x-2"))
	for _, n := range names {
		items = append(items, avatar(n))
	}
	return dom.Div(items...)
}
