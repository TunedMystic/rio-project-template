package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// Page wraps body content in the full HTML document: head (tokens + meta +
// stylesheet), navbar, main, footer.
func Page(pd config.PageData, meta config.Meta, body ...dom.Node) dom.Node {
	return dom.Doctype(dom.Html(
		dom.Lang("en"),
		dom.Head(
			dom.Meta(dom.Charset("utf-8")),
			dom.Meta(dom.Name("viewport"), dom.Content("width=device-width, initial-scale=1")),
			dom.TitleEl(dom.Text(meta.Title)),
			dom.Meta(dom.Name("description"), dom.Content(meta.Description)),
			pd.Tokens.StyleVars(),
			dom.Link(dom.Rel("stylesheet"), dom.Href("/static/css/styles.css")),
		),
		dom.Body(
			dom.Class("min-h-screen flex flex-col bg-[var(--color-background)] text-[var(--color-text)] font-[family-name:var(--font-family)] text-[length:var(--font-size-base)] leading-relaxed antialiased"),
			navbar(pd),
			dom.Main(dom.Class("flex-1 py-10"), ui.Container(body...)),
			footer(pd),
		),
	))
}
