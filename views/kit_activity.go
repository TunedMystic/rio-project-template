package views

import (
	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// activityItem is one event in an activityFeed: an icon marker (colored by
// Variant), a title, a muted meta line, and a right-aligned timestamp.
type activityItem struct {
	Icon    string
	Title   string
	Meta    string
	Time    string
	Variant ui.BadgeVariant
}

// dotColor maps a badge variant to its accent color var for the timeline dot.
// BadgeNeutral (and any unknown) falls back to the primary accent.
func dotColor(v ui.BadgeVariant) string {
	switch v {
	case ui.BadgeSuccess:
		return "var(--color-success)"
	case ui.BadgeWarning:
		return "var(--color-warning)"
	case ui.BadgeDanger:
		return "var(--color-danger)"
	default:
		return "var(--color-primary)"
	}
}

// activityFeed renders a vertical timeline of events on a continuous rail.
func activityFeed(items []activityItem) dom.Node {
	rail := make([]dom.Node, 0, len(items)+1)
	rail = append(rail, dom.Class("relative flex flex-col gap-6 border-l border-[var(--color-border)] pl-6"))
	for _, it := range items {
		c := dotColor(it.Variant)
		rail = append(rail, dom.Div(
			dom.Class("relative"),
			// Dot marker straddling the rail; colored by variant.
			dom.Div(
				dom.Class("absolute -left-[37px] flex h-6 w-6 items-center justify-center rounded-full shadow-sm"),
				dom.Style("color:"+c+";background-color:color-mix(in srgb, "+c+" 15%, transparent)"),
				icon(it.Icon, 13),
			),
			dom.Div(
				dom.Class("flex items-start justify-between gap-3"),
				dom.Div(
					dom.Div(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(it.Title)),
					dom.Div(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(it.Meta)),
				),
				dom.Span(
					dom.Class("shrink-0 text-[length:var(--font-size-sm)] [font-variant-numeric:tabular-nums] text-[var(--color-text-muted)]"),
					dom.Text(it.Time),
				),
			),
		))
	}
	return dom.Div(
		dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm"),
		dom.Div(dom.Class("mb-4 font-semibold text-[var(--color-text)]"), dom.Text("Activity")),
		dom.Div(rail...),
	)
}
