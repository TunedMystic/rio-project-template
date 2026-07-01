package views

import (
	"fmt"
	"strings"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// tableRow is one row of dataTable: leading text cells, a status label rendered
// as a badge, and a row-action menu.
type tableRow struct {
	Cells   []string
	Status  string
	Variant ui.BadgeVariant
}

// metricCard renders a labeled big tabular number, a colored delta, and an
// inline SVG sparkline using the chart ramp.
func metricCard(label, value string, deltaPct float64, spark []int) dom.Node {
	arrow, deltaColor := "▲", "var(--color-success)"
	if deltaPct < 0 {
		arrow, deltaColor = "▼", "var(--color-danger)"
	}
	return dom.Div(
		dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm"),
		dom.Div(
			dom.Class("text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)]"),
			dom.Text(label),
		),
		dom.Div(
			dom.Class("mt-1 flex items-end justify-between gap-3"),
			dom.Div(
				dom.Class("text-[length:var(--font-size-2xl)] [font-variant-numeric:tabular-nums] [font-weight:var(--font-weight-heading)] tracking-tight text-[var(--color-text)]"),
				dom.Text(value),
			),
			sparkline(spark),
		),
		dom.Div(
			dom.Class("mt-2 text-[length:var(--font-size-sm)] font-medium [font-variant-numeric:tabular-nums]"),
			dom.Style("color:"+deltaColor),
			dom.Text(fmt.Sprintf("%s %.1f%%", arrow, absFloat(deltaPct))),
		),
	)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// sparkline renders a polyline SVG over the data points using the chart ramp.
func sparkline(data []int) dom.Node {
	if len(data) == 0 {
		return dom.Raw("")
	}
	const w, h = 120, 32
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span == 0 {
		span = 1
	}
	denom := len(data) - 1
	if denom == 0 {
		denom = 1
	}
	var pts strings.Builder
	for i, v := range data {
		x := float64(i) * float64(w) / float64(denom)
		y := float64(h) - float64(v-min)/float64(span)*float64(h-4) - 2
		if i > 0 {
			pts.WriteByte(' ')
		}
		fmt.Fprintf(&pts, "%.1f,%.1f", x, y)
	}
	svg := fmt.Sprintf(
		`<svg viewBox="0 0 %d %d" width="%d" height="%d" fill="none" aria-hidden="true"><polyline points="%s" stroke="var(--chart-4)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`,
		w, h, w, h, pts.String())
	return dom.Raw(svg)
}

// dataTable renders a header, body rows with status badges + a row-action menu,
// hairline rows, and a pagination footer.
func dataTable(cols []string, rows []tableRow, rangeLabel string) dom.Node {
	headCells := make([]dom.Node, 0, len(cols)+1)
	headCells = append(headCells, dom.Class("border-b border-[var(--color-border)]"))
	for _, c := range cols {
		headCells = append(headCells, dom.Th(
			dom.Class("px-4 py-3 text-left text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text-muted)]"),
			dom.Text(c),
		))
	}

	bodyRows := make([]dom.Node, 0, len(rows))
	for _, r := range rows {
		cells := make([]dom.Node, 0, len(r.Cells)+2)
		for _, c := range r.Cells {
			cells = append(cells, dom.Td(
				dom.Class("px-4 py-3 text-[length:var(--font-size-sm)] [font-variant-numeric:tabular-nums] text-[var(--color-text)]"),
				dom.Text(c),
			))
		}
		cells = append(cells, dom.Td(dom.Class("px-4 py-3"), ui.Badge(r.Variant, r.Status)))
		cells = append(cells, dom.Td(dom.Class("px-4 py-3 text-right"), rowActions()))
		bodyRows = append(bodyRows, dom.Tr(
			withClass("border-b border-[var(--color-border)] last:border-0", cells)...))
	}

	return dom.Div(
		dom.Class("overflow-hidden rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)]"),
		dom.Div(
			dom.Class("overflow-x-auto"),
			dom.Table(
				dom.Class("w-full border-collapse"),
				dom.Thead(dom.Tr(headCells...)),
				dom.Tbody(bodyRows...),
			),
		),
		dom.Div(
			dom.Class("flex items-center justify-between border-t border-[var(--color-border)] px-4 py-3 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Span(dom.Text(rangeLabel)),
			dom.Div(
				dom.Class("flex items-center gap-2"),
				ui.Button(ui.ButtonGhost, "Prev"),
				ui.Button(ui.ButtonGhost, "Next"),
			),
		),
	)
}

// rowActions is the per-row <details> action menu (no JS).
func rowActions() dom.Node {
	return dom.Details(
		dom.Class("relative inline-block text-left"),
		dom.Summary(
			dom.Class("flex h-8 w-8 cursor-pointer list-none items-center justify-center rounded-[var(--radius-base)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-raised)] [&::-webkit-details-marker]:hidden focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
			dom.Aria("label", "Row actions"),
			icon("more", 18),
		),
		dom.Div(
			dom.Class("absolute right-0 z-10 mt-1 flex w-36 flex-col gap-1 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-1 text-[length:var(--font-size-sm)] shadow-lg"),
			actionItem("View"),
			actionItem("Edit"),
			actionItem("Delete"),
		),
	)
}

func actionItem(label string) dom.Node {
	return dom.A(
		dom.Class("rounded-[calc(var(--radius-base)-2px)] px-3 py-1.5 text-[var(--color-text)] hover:bg-[var(--color-surface-raised)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
		dom.Href("#"),
		dom.Text(label),
	)
}

// barChart renders vertical SVG bars with an axis baseline using the chart ramp.
func barChart(series []int) dom.Node {
	if len(series) == 0 {
		return dom.Raw("")
	}
	const w, h, gap = 240, 120, 8
	max := series[0]
	for _, v := range series {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}
	n := len(series)
	bw := (float64(w) - float64(gap)*float64(n-1)) / float64(n)
	baseline := h - 12
	var bars strings.Builder
	for i, v := range series {
		bh := float64(v) / float64(max) * float64(baseline)
		x := float64(i) * (bw + float64(gap))
		y := float64(baseline) - bh
		fmt.Fprintf(&bars, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="3" fill="var(--chart-3)"/>`, x, y, bw, bh)
	}
	svg := fmt.Sprintf(
		`<svg viewBox="0 0 %d %d" width="100%%" height="%d" preserveAspectRatio="none" aria-hidden="true">%s<line x1="0" y1="%d" x2="%d" y2="%d" stroke="var(--color-border)" stroke-width="1"/></svg>`,
		w, h, h, bars.String(), baseline, w, baseline)
	return dom.Raw(svg)
}

// usageMeter renders a labeled progress bar with value text.
func usageMeter(label string, used, limit int) dom.Node {
	pct := 0
	if limit > 0 {
		pct = used * 100 / limit
	}
	if pct > 100 {
		pct = 100
	}
	return dom.Div(
		dom.Class("flex flex-col gap-2"),
		dom.Div(
			dom.Class("flex items-center justify-between text-[length:var(--font-size-sm)]"),
			dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text(label)),
			dom.Span(
				dom.Class("[font-variant-numeric:tabular-nums] text-[var(--color-text-muted)]"),
				dom.Text(fmt.Sprintf("%d / %d", used, limit)),
			),
		),
		dom.Div(
			dom.Class("h-2 w-full overflow-hidden rounded-full bg-[var(--color-surface-raised)]"),
			dom.Div(
				dom.Class("h-full rounded-full bg-[var(--color-primary)]"),
				dom.Style(fmt.Sprintf("width:%d%%", pct)),
			),
		),
	)
}

// pageWindow returns the page numbers to render: always page 1, the last page,
// and a window of current ±1. A 0 entry marks an ellipsis gap between numbers.
func pageWindow(current, total int) []int {
	if total < 1 {
		total = 1
	}
	if current < 1 {
		current = 1
	}
	if current > total {
		current = total
	}
	show := map[int]bool{1: true, total: true, current: true}
	if current-1 >= 1 {
		show[current-1] = true
	}
	if current+1 <= total {
		show[current+1] = true
	}
	out := make([]int, 0, total)
	prev := 0
	for p := 1; p <= total; p++ {
		if !show[p] {
			continue
		}
		if prev != 0 && p-prev > 1 {
			out = append(out, 0) // ellipsis gap
		}
		out = append(out, p)
		prev = p
	}
	return out
}

// pagination renders a numbered pager with Prev/Next controls. The current page
// is a filled primary chip; other pages link to baseHref?page=N. Prev/Next
// disable at the ends. A 0 from pageWindow renders as an ellipsis.
func pagination(current, total int, baseHref string) dom.Node {
	if total < 1 {
		total = 1
	}
	if current < 1 {
		current = 1
	}
	if current > total {
		current = total
	}

	kids := []dom.Node{
		dom.Class("flex items-center gap-1"),
		dom.Aria("label", "Pagination"),
	}

	if current > 1 {
		kids = append(kids, pageLink(fmt.Sprintf("%s?page=%d", baseHref, current-1), "Prev", false))
	} else {
		kids = append(kids, pageDisabled("Prev"))
	}

	for _, p := range pageWindow(current, total) {
		if p == 0 {
			kids = append(kids, dom.Span(
				dom.Class("px-2 text-[var(--color-text-muted)]"),
				dom.Text("…"),
			))
			continue
		}
		kids = append(kids, pageLink(
			fmt.Sprintf("%s?page=%d", baseHref, p),
			fmt.Sprintf("%d", p),
			p == current,
		))
	}

	if current < total {
		kids = append(kids, pageLink(fmt.Sprintf("%s?page=%d", baseHref, current+1), "Next", false))
	} else {
		kids = append(kids, pageDisabled("Next"))
	}

	return dom.Nav(kids...)
}

// pageLink renders one pager control. When current, it renders a filled primary
// chip marked aria-current instead of a link.
func pageLink(href, label string, current bool) dom.Node {
	if current {
		return dom.Span(
			dom.Class("inline-flex h-8 min-w-8 items-center justify-center rounded-[var(--radius-base)] bg-[var(--color-primary)] px-2 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-on-primary)] [font-variant-numeric:tabular-nums]"),
			dom.Aria("current", "page"),
			dom.Text(label),
		)
	}
	return dom.A(
		dom.Class("inline-flex h-8 min-w-8 items-center justify-center rounded-[var(--radius-base)] px-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] [font-variant-numeric:tabular-nums] transition-colors hover:bg-[var(--color-surface-raised)] hover:text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
		dom.Href(href),
		dom.Text(label),
	)
}

// pageDisabled renders a muted, non-interactive Prev/Next at the ends.
func pageDisabled(label string) dom.Node {
	return dom.Span(
		dom.Class("inline-flex h-8 min-w-8 cursor-not-allowed items-center justify-center rounded-[var(--radius-base)] px-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] opacity-50 [font-variant-numeric:tabular-nums]"),
		dom.Aria("disabled", "true"),
		dom.Text(label),
	)
}

// emptyState renders a centered empty state with an icon, copy, and optional CTA.
func emptyState(iconName, title, body string, cta dom.Node) dom.Node {
	children := []dom.Node{
		dom.Class("flex flex-col items-center justify-center rounded-[var(--radius-base)] border border-dashed border-[var(--color-border)] px-6 py-12 text-center"),
		dom.Div(
			dom.Class("flex h-12 w-12 items-center justify-center rounded-full bg-[var(--color-surface-raised)] text-[var(--color-text-muted)]"),
			icon(iconName, 24),
		),
		dom.Div(
			dom.Class("mt-4 font-semibold tracking-tight text-[var(--color-text)]"),
			dom.Text(title),
		),
		dom.P(
			dom.Class("mt-1 max-w-sm text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Text(body),
		),
	}
	if cta != nil {
		children = append(children, dom.Div(dom.Class("mt-5"), cta))
	}
	return dom.Div(children...)
}
