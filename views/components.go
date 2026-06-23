package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

func navbar(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.HeaderLinks)+1)
	links = append(links, dom.Class("flex items-center gap-6"))
	for _, l := range pd.HeaderLinks {
		links = append(links, ui.Link(l.Href, l.Text))
	}
	return dom.Header(
		dom.Class("border-b border-[var(--color-border)]"),
		ui.Container(
			dom.Div(
				dom.Class("flex items-center justify-between py-4"),
				ui.Link("/", pd.SiteName),
				dom.Nav(links...),
			),
		),
	)
}

func footer(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.FooterLinks)+1)
	links = append(links, dom.Class("flex flex-wrap items-center gap-6"))
	for _, l := range pd.FooterLinks {
		links = append(links, ui.Link(l.Href, l.Text))
	}
	return dom.Footer(
		dom.Class("border-t border-[var(--color-border)] py-8 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
		ui.Container(dom.Nav(links...)),
	)
}

// submitButton renders a submit button styled like a ui primary button.
// ui.Button hardcodes type="button"; if submit buttons recur across products,
// promote a submit/type option into rio/ui (rule of three).
func submitButton(label string) dom.Node {
	return dom.Button(
		dom.Type("submit"),
		dom.Class("inline-flex items-center justify-center gap-2 rounded-[var(--radius-base)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold tracking-tight bg-[var(--color-primary)] text-[var(--color-on-primary)] shadow-sm hover:shadow-md hover:brightness-105 active:brightness-95 cursor-pointer"),
		dom.Text(label),
	)
}
