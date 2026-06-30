package views

import (
	"strings"

	"app/config"

	"github.com/tunedmystic/rio/dom"
)

// Page wraps body content in the full HTML document: head (tokens + meta +
// stylesheet), navbar, main, footer. Body sections are placed full-width so
// each page can run header bands edge-to-edge and constrain its own content.
func Page(pd config.PageData, meta config.Meta, body ...dom.Node) dom.Node {
	main := make([]dom.Node, 0, len(body)+1)
	main = append(main, dom.Class("flex-1"))
	main = append(main, body...)

	return dom.Doctype(dom.Html(
		dom.Lang("en"),
		dom.Head(
			dom.Meta(dom.Charset("utf-8")),
			dom.Meta(dom.Name("viewport"), dom.Content("width=device-width, initial-scale=1")),
			dom.TitleEl(dom.Text(meta.Title)),
			dom.Meta(dom.Name("description"), dom.Content(meta.Description)),
			dom.Link(dom.Rel("icon"), dom.Href("/static/img/favicon.webp")),
			pd.Tokens.StyleVars(),
			themeVarsStyle(pd.ThemeVars),
			dom.Link(dom.Rel("stylesheet"), dom.Href(cssHref(pd.AssetVersion))),
		),
		dom.Body(
			dom.Class("min-h-screen flex flex-col bg-[var(--color-background)] text-[var(--color-text)] font-[family-name:var(--font-family)] text-[length:var(--font-size-base)] leading-relaxed"),
			navbar(pd),
			dom.Main(main...),
			footer(pd),
		),
	))
}

// cssHref appends the asset version as a query string so each deploy busts the
// browser cache; without a version it returns the bare path.
func cssHref(version string) string {
	if version == "" {
		return "/static/css/styles.css"
	}
	return "/static/css/styles.css?v=" + version
}

// shell centers page content at a comfortable reading width.
func shell(children ...dom.Node) dom.Node {
	return dom.Div(withClass("mx-auto w-full max-w-5xl px-5", children)...)
}

// withClass prepends a class attribute to a children slice.
func withClass(class string, children []dom.Node) []dom.Node {
	out := make([]dom.Node, 0, len(children)+1)
	out = append(out, dom.Class(class))
	out = append(out, children...)
	return out
}

// themeVarsStyle emits the extended theme CSS variables into :root, alongside
// ui.Tokens.StyleVars (which the data/chart components read but Tokens does not
// cover). No vendored code changes; this is a second :root block.
func themeVarsStyle(vars []config.ThemeVar) dom.Node {
	var b strings.Builder
	b.WriteString(":root{")
	for _, v := range vars {
		b.WriteString(v.Name)
		b.WriteString(":")
		b.WriteString(v.Value)
		b.WriteString(";")
	}
	b.WriteString("}")
	return dom.StyleEl(dom.Raw(b.String()))
}
