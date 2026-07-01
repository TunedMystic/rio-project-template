package views

import (
	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// selectField renders a labeled dropdown that uses the app's own chevron (a
// right-aligned background SVG) instead of the inconsistent native <select>
// arrow. The option whose value equals selected is pre-selected. When compact,
// the control is sized to its content (for inline use next to a button);
// otherwise it is full-width like the other form fields.
func selectField(name, label, selected string, opts []ui.Option, compact bool) dom.Node {
	const chevron = `appearance:none;-webkit-appearance:none;` +
		`background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2394a3b8' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E");` +
		`background-repeat:no-repeat;background-position:right 0.7rem center;background-size:16px 16px`

	width := "w-full"
	if compact {
		width = "w-fit"
	}

	sel := []dom.Node{
		dom.Class(width + " rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] py-2 pl-3 pr-9 text-[var(--color-text)] shadow-sm transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] focus:border-[var(--color-primary)]"),
		dom.Style(chevron),
		dom.Id(name), dom.Name(name),
	}
	for _, o := range opts {
		opt := []dom.Node{dom.Value(o.Value)}
		if o.Value == selected {
			opt = append(opt, dom.Selected())
		}
		opt = append(opt, dom.Text(o.Label))
		sel = append(sel, dom.Option(opt...))
	}

	return dom.Div(
		// items-start keeps a compact select from being stretched to the label
		// width by the surrounding flex layout.
		dom.Class("flex flex-col items-start gap-1"),
		dom.Label(
			dom.Class("text-[length:var(--font-size-sm)] font-medium text-[var(--color-text)]"),
			dom.For(name), dom.Text(label),
		),
		dom.Select(sel...),
	)
}
