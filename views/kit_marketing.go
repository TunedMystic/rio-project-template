package views

import (
	"github.com/tunedmystic/rio/dom"
)

type featureItem struct {
	Icon  string
	Title string
	Blurb string
}

type plan struct {
	Name        string
	Price       string
	Period      string
	Features    []string
	Highlighted bool
	CTA         dom.Node
}

type faqItem struct {
	Q string
	A string
}

// svgPanel is a decorative placeholder visual (no external asset) for heroes
// and feature highlights.
func svgPanel() dom.Node {
	return dom.Div(
		dom.Class("aspect-[4/3] w-full overflow-hidden rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface-raised)]"),
		dom.Raw(`<svg viewBox="0 0 400 300" width="100%" height="100%" preserveAspectRatio="xMidYMid slice" aria-hidden="true">`+
			`<rect width="400" height="300" fill="var(--color-surface-raised)"/>`+
			`<rect x="28" y="28" width="160" height="14" rx="7" fill="var(--chart-1)"/>`+
			`<rect x="28" y="56" width="240" height="10" rx="5" fill="var(--color-border)"/>`+
			`<rect x="28" y="120" width="100" height="150" rx="8" fill="var(--chart-2)"/>`+
			`<rect x="150" y="160" width="100" height="110" rx="8" fill="var(--chart-3)"/>`+
			`<rect x="272" y="100" width="100" height="170" rx="8" fill="var(--chart-4)"/>`+
			`</svg>`),
	)
}

// hero renders the landing hero: eyebrow, headline, subhead, two CTAs, visual.
func hero(eyebrowText, headline, sub string, primaryCTA, secondaryCTA, visual dom.Node) dom.Node {
	return dom.Section(
		dom.Class("py-16 sm:py-24"),
		shell(
			dom.Div(
				dom.Class("grid items-center gap-12 lg:grid-cols-2"),
				dom.Div(
					eyebrow(eyebrowText),
					dom.H1(
						dom.Class("mt-4 text-[length:var(--font-size-2xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)] sm:text-5xl"),
						dom.Text(headline),
					),
					dom.P(
						dom.Class("mt-5 max-w-xl text-[var(--color-text-muted)]"),
						dom.Text(sub),
					),
					dom.Div(
						dom.Class("mt-8 flex flex-wrap items-center gap-4"),
						primaryCTA,
						secondaryCTA,
					),
				),
				visual,
			),
		),
	)
}

// featureHighlight renders an alternating image/text row; reverse swaps sides.
func featureHighlight(eyebrowText, title, body string, reverse bool) dom.Node {
	textCol := dom.Div(
		eyebrow(eyebrowText),
		dom.H2(
			dom.Class("mt-3 text-[length:var(--font-size-xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
			dom.Text(title),
		),
		dom.P(dom.Class("mt-3 text-[var(--color-text-muted)]"), dom.Text(body)),
	)
	cols := []dom.Node{dom.Class("grid items-center gap-10 lg:grid-cols-2")}
	if reverse {
		cols = append(cols, svgPanel(), textCol)
	} else {
		cols = append(cols, textCol, svgPanel())
	}
	return dom.Section(dom.Class("py-12"), shell(dom.Div(cols...)))
}

// featureGrid renders icon + title + blurb cards.
func featureGrid(items []featureItem) dom.Node {
	cards := make([]dom.Node, 0, len(items)+1)
	cards = append(cards, dom.Class("grid gap-6 sm:grid-cols-2 lg:grid-cols-3"))
	for _, it := range items {
		cards = append(cards, dom.Div(
			dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-sm"),
			dom.Div(
				dom.Class("flex h-11 w-11 items-center justify-center rounded-[var(--radius-base)] bg-[var(--color-primary)]/10 text-[var(--color-primary)]"),
				icon(it.Icon, 22),
			),
			dom.Div(
				dom.Class("mt-4 font-semibold tracking-tight text-[var(--color-text)]"),
				dom.Text(it.Title),
			),
			dom.P(
				dom.Class("mt-1 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
				dom.Text(it.Blurb),
			),
		))
	}
	return dom.Section(dom.Class("py-12"), shell(dom.Div(cards...)))
}

// pricingTable renders plan columns with a feature checklist; the highlighted
// plan carries a "Popular" badge and an accent ring.
func pricingTable(plans []plan) dom.Node {
	cols := make([]dom.Node, 0, len(plans)+1)
	cols = append(cols, dom.Class("grid gap-6 md:grid-cols-2 lg:grid-cols-3"))
	for _, p := range plans {
		cls := "flex flex-col rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-sm"
		if p.Highlighted {
			cls = "flex flex-col rounded-[var(--radius-base)] border-2 border-[var(--color-primary)] bg-[var(--color-surface)] p-6 shadow-md ring-1 ring-[var(--color-ring)]"
		}
		head := []dom.Node{dom.Class("flex items-center justify-between")}
		head = append(head, dom.Span(
			dom.Class("font-semibold tracking-tight text-[var(--color-text)]"),
			dom.Text(p.Name),
		))
		if p.Highlighted {
			head = append(head, dom.Span(
				dom.Class("rounded-full bg-[var(--color-primary)] px-2.5 py-0.5 text-[length:var(--font-size-sm)] font-medium text-[var(--color-on-primary)]"),
				dom.Text("Popular"),
			))
		}
		feats := make([]dom.Node, 0, len(p.Features)+1)
		feats = append(feats, dom.Class("mt-6 flex flex-1 flex-col gap-2 text-[length:var(--font-size-sm)] text-[var(--color-text)]"))
		for _, f := range p.Features {
			feats = append(feats, dom.Li(
				dom.Class("flex items-center gap-2"),
				dom.Span(dom.Class("text-[var(--color-success)]"), icon("check", 16)),
				dom.Text(f),
			))
		}
		cols = append(cols, dom.Div(
			dom.Class(cls),
			dom.Div(head...),
			dom.Div(
				dom.Class("mt-4 flex items-baseline gap-1"),
				dom.Span(
					dom.Class("text-[length:var(--font-size-2xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
					dom.Text(p.Price),
				),
				dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(p.Period)),
			),
			dom.Ul(feats...),
			dom.Div(dom.Class("mt-6"), p.CTA),
		))
	}
	return dom.Section(dom.Class("py-12"), shell(dom.Div(cols...)))
}

// testimonial renders a single quote with author and role.
func testimonial(quote, author, role string) dom.Node {
	return dom.Section(
		dom.Class("py-12"),
		shell(dom.Blockquote(
			dom.Class("mx-auto max-w-2xl rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-8 text-center shadow-sm"),
			dom.P(
				dom.Class("text-[length:var(--font-size-lg)] tracking-tight text-[var(--color-text)]"),
				dom.Text("“"+quote+"”"),
			),
			dom.Div(
				dom.Class("mt-4 text-[length:var(--font-size-sm)]"),
				dom.Span(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(author)),
				dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(" — "+role)),
			),
		)),
	)
}

// logoCloud renders a muted row of partner/customer wordmarks (text labels).
func logoCloud(labels []string) dom.Node {
	items := make([]dom.Node, 0, len(labels)+1)
	items = append(items, dom.Class("flex flex-wrap items-center justify-center gap-x-10 gap-y-4 opacity-70"))
	for _, l := range labels {
		items = append(items, dom.Span(
			dom.Class("text-[length:var(--font-size-lg)] font-semibold tracking-tight text-[var(--color-text-muted)]"),
			dom.Text(l),
		))
	}
	return dom.Section(dom.Class("py-10"), shell(dom.Div(items...)))
}

// ctaBand renders a full-width call-to-action band.
func ctaBand(title, body string, cta dom.Node) dom.Node {
	return dom.Section(
		dom.Class("py-12"),
		shell(dom.Div(
			dom.Class("flex flex-col items-center gap-5 rounded-[var(--radius-base)] bg-[var(--color-primary)] px-8 py-12 text-center"),
			dom.H2(
				dom.Class("text-[length:var(--font-size-xl)] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-on-primary)]"),
				dom.Text(title),
			),
			dom.P(
				dom.Class("max-w-xl text-[var(--color-on-primary)]/80"),
				dom.Text(body),
			),
			cta,
		)),
	)
}

// faq renders a <details> accordion.
func faq(items []faqItem) dom.Node {
	rows := make([]dom.Node, 0, len(items)+1)
	rows = append(rows, dom.Class("mx-auto max-w-2xl divide-y divide-[var(--color-border)] rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)]"))
	for _, it := range items {
		rows = append(rows, dom.Details(
			dom.Class("group px-5"),
			dom.Summary(
				dom.Class("flex cursor-pointer list-none items-center justify-between py-4 font-medium text-[var(--color-text)] [&::-webkit-details-marker]:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
				dom.Text(it.Q),
				dom.Span(
					dom.Class("text-[var(--color-text-muted)] transition-transform group-open:rotate-180"),
					icon("chevron-down", 18),
				),
			),
			dom.P(
				dom.Class("pb-4 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
				dom.Text(it.A),
			),
		))
	}
	return dom.Section(dom.Class("py-12"), shell(dom.Div(rows...)))
}
