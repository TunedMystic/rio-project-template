package views

import (
	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

type tabItem struct {
	Key   string
	Label string
}

// formShowcase renders the ui form fields in a representative layout, including
// one field shown in its error state.
func formShowcase() dom.Node {
	return dom.Form(
		dom.Class("flex max-w-lg flex-col gap-4"),
		ui.TextField("full_name", "Full name", "Ada Lovelace", ""),
		ui.TextField("email", "Email", "ada@example.com", "Enter a valid email address"),
		selectField("plan", "Plan", "pro", []ui.Option{
			{Value: "starter", Label: "Starter"},
			{Value: "pro", Label: "Pro"},
		}, false),
		ui.Textarea("bio", "Bio", "Building things with Go.", ""),
		toggle("notify", "Email notifications", true),
		dom.Div(dom.Class("pt-2"), ui.Button(ui.ButtonPrimary, "Save")),
	)
}

// toggle renders a switch-styled checkbox (no JS); peer classes drive the knob.
func toggle(name, label string, on bool) dom.Node {
	input := []dom.Node{
		dom.Type("checkbox"),
		dom.Name(name),
		dom.Class("peer sr-only"),
	}
	if on {
		input = append(input, dom.Checked())
	}
	return dom.Label(
		dom.Class("inline-flex cursor-pointer items-center gap-3"),
		dom.Input(input...),
		dom.Span(dom.Class("relative h-6 w-11 rounded-full bg-[var(--color-surface-raised)] transition-colors peer-checked:bg-[var(--color-primary)] after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-[var(--color-surface)] after:shadow after:transition-transform peer-checked:after:translate-x-5")),
		dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text)]"), dom.Text(label)),
	)
}

// tabStrip renders a horizontal tab strip; the active tab carries the primary
// underline (matching the account-tab style).
func tabStrip(items []tabItem, active string) dom.Node {
	tabs := make([]dom.Node, 0, len(items)+1)
	tabs = append(tabs, dom.Class("flex gap-2 border-b border-[var(--color-border)]"))
	for _, it := range items {
		cls := "px-4 py-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"
		if it.Key == active {
			cls = "px-4 py-2 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-primary)] border-b-2 border-[var(--color-primary)] -mb-px focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"
		}
		tabs = append(tabs, dom.A(dom.Class(cls), dom.Href("#"), dom.Text(it.Label)))
	}
	return dom.Div(tabs...)
}
