# Dashboard Kit Components Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three no-JS, token-driven components — breadcrumbs, an activity feed/timeline, and a numbered pagination control — to complete the "Data & dashboard" section of the `/kit` showcase, and wire breadcrumbs into the real `/account` pages.

**Architecture:** Each component is a plain Go function returning `dom.Node`, following the existing `views/` patterns (Tailwind utility classes referencing CSS token vars like `var(--color-primary)`; SVG/markup via `dom.*` builders; the shared `icon()` helper). Pagination replaces the data table's inline Prev/Next footer. Tests follow the `render(...)` → `strings.Contains` convention already used in `views/*_test.go`.

**Tech Stack:** Go, `github.com/tunedmystic/rio/dom`, `github.com/tunedmystic/rio/ui`, Tailwind v4 (compile-time; no runtime JS).

## Global Constraints

- No JavaScript. All components render static, server-side HTML only.
- Token-driven only: colors/sizes/radii come from CSS vars (`var(--color-*)`, `var(--radius-base)`, `var(--font-size-*)`, `var(--font-weight-*)`) so both theme presets render correctly with no per-theme code.
- Follow existing `views/` idioms: `dom.Class(...)`, `dom.Text(...)`, `withClass(class, []dom.Node)`, the shared `icon(name string, size int)` helper, and surface-card chrome `rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm`.
- `ui.BadgeVariant` values available: `BadgeNeutral`, `BadgeSuccess`, `BadgeWarning`, `BadgeDanger` (there is **no** `BadgePrimary`).
- `config.Link` has exactly two fields: `Text string`, `Href string`.
- Run all tests with `go test ./views/`. Build CSS after markup changes with `make tailwind` (only needed to view in-browser, not for `go test`).

---

### Task 1: Breadcrumbs component

**Files:**
- Modify: `views/components.go` (add `breadcrumbs`; import `app/config` is already present)
- Test: `views/components_test.go`

**Interfaces:**
- Consumes: `config.Link{Text, Href}`, existing `icon(name, size)` helper.
- Produces: `breadcrumbs(trail []config.Link) dom.Node`

- [ ] **Step 1: Write the failing test**

Add to `views/components_test.go`:

```go
func TestBreadcrumbs_RendersTrailWithCurrent(t *testing.T) {
	trail := []config.Link{
		{Text: "Home", Href: "/"},
		{Text: "Account", Href: "/account"},
		{Text: "Security", Href: "/account/security"},
	}
	html := render(breadcrumbs(trail))
	if !strings.Contains(html, `aria-label="Breadcrumb"`) {
		t.Error("breadcrumbs missing nav aria-label")
	}
	for _, want := range []string{"Home", "Account", "Security"} {
		if !strings.Contains(html, want) {
			t.Errorf("breadcrumbs missing crumb %q", want)
		}
	}
	// First crumb is a link; last crumb is the current page (not a link).
	if !strings.Contains(html, `href="/"`) {
		t.Error("breadcrumbs missing link on non-final crumb")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("breadcrumbs missing aria-current on final crumb")
	}
	if strings.Contains(html, `href="/account/security"`) {
		t.Error("final crumb should not be a link")
	}
}
```

Ensure `views/components_test.go` imports `strings`, `testing`, and `app/config`. Check the top of the file; add any missing imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestBreadcrumbs -v`
Expected: FAIL — `undefined: breadcrumbs`.

- [ ] **Step 3: Write minimal implementation**

Add to `views/components.go` (place near `navbar`/`footer`, the navigation chrome):

```go
// breadcrumbs renders a navigation trail: muted links separated by chevrons,
// with the final crumb rendered as the bold current page (not a link).
func breadcrumbs(trail []config.Link) dom.Node {
	items := make([]dom.Node, 0, len(trail)*2+2)
	items = append(items,
		dom.Class("flex flex-wrap items-center gap-2 text-[length:var(--font-size-sm)]"),
		dom.Aria("label", "Breadcrumb"),
	)
	for i, l := range trail {
		if i > 0 {
			items = append(items, dom.Span(
				dom.Class("text-[var(--color-border)]"),
				icon("chevron-right", 16),
			))
		}
		if i == len(trail)-1 {
			items = append(items, dom.Span(
				dom.Class("font-semibold text-[var(--color-text)]"),
				dom.Aria("current", "page"),
				dom.Text(l.Text),
			))
		} else {
			items = append(items, dom.A(
				dom.Class("text-[var(--color-text-muted)] transition-colors hover:text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
				dom.Href(l.Href),
				dom.Text(l.Text),
			))
		}
	}
	return dom.Nav(items...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./views/ -run TestBreadcrumbs -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add views/components.go views/components_test.go
git commit -m "feat: add breadcrumbs navigation component"
```

---

### Task 2: Wire breadcrumbs into the account area

**Files:**
- Modify: `views/account.go` (`accountShell`, ~lines 32-57)
- Test: `views/account_test.go`

**Interfaces:**
- Consumes: `breadcrumbs(trail []config.Link)` from Task 1; existing `accountTabs` slice and `AccountView.Active`.
- Produces: no new exported symbols; adds a breadcrumb trail to every account page.

- [ ] **Step 1: Write the failing test**

Add to `views/account_test.go`:

```go
func TestAccountShell_RendersBreadcrumbs(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "security", CSRF: "c"}
	var b bytes.Buffer
	_ = Security(pd, config.Meta{Title: "Security"}, av, nil, "", false).Render(&b)
	html := b.String()
	if !strings.Contains(html, `aria-label="Breadcrumb"`) {
		t.Error("account page missing breadcrumbs")
	}
	if !strings.Contains(html, "Home") {
		t.Error("account breadcrumb missing Home crumb")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("account breadcrumb missing current page marker")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestAccountShell_RendersBreadcrumbs -v`
Expected: FAIL — no `aria-label="Breadcrumb"` in output.

- [ ] **Step 3: Write minimal implementation**

In `views/account.go`, inside `accountShell`, after building `tabs` and before building `content`, compute the active crumb and build the trail. Then insert `breadcrumbs(trail)` as the **first** content item (before flash/error). Replace the `content` assembly block:

```go
	// Active-tab crumb label for the breadcrumb trail.
	crumbLabel, crumbHref := "Account", "/account"
	for _, tb := range accountTabs {
		if tb.key == av.Active {
			crumbLabel, crumbHref = tb.label, tb.href
			break
		}
	}
	trail := []config.Link{
		{Text: "Home", Href: "/"},
		{Text: "Account", Href: "/account"},
		{Text: crumbLabel, Href: crumbHref},
	}

	content := make([]dom.Node, 0, len(body)+4)
	content = append(content, breadcrumbs(trail))
	if av.Flash != "" {
		content = append(content, ui.Alert(ui.AlertSuccess, dom.Text(av.Flash)))
	}
	if av.Error != "" {
		content = append(content, ui.Alert(ui.AlertError, dom.Text(av.Error)))
	}
	content = append(content, dom.Nav(tabs...))
	content = append(content, body...)
```

(This replaces the existing `content := make(...)` through `content = append(content, dom.Nav(tabs...))` block; keep the trailing `content = append(content, body...)` — shown above for clarity.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestAccountShell_RendersBreadcrumbs|TestProfile|TestSecurity' -v`
Expected: PASS (existing account tests still green — breadcrumbs are additive).

- [ ] **Step 5: Commit**

```bash
git add views/account.go views/account_test.go
git commit -m "feat: render breadcrumb trail on account pages"
```

---

### Task 3: Pagination component

**Files:**
- Modify: `views/kit_data.go` (add `pageWindow`, `pagination`, `pageLink`, `pageDisabled`; `fmt` already imported)
- Test: `views/kit_data_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `pageWindow(current, total int) []int` — page numbers to show; `0` marks an ellipsis gap.
  - `pagination(current, total int, baseHref string) dom.Node`
  - `pageLink(href, label string, current bool) dom.Node`
  - `pageDisabled(label string) dom.Node`

- [ ] **Step 1: Write the failing tests**

Add to `views/kit_data_test.go`:

```go
func TestPageWindow_CollapsesLongRanges(t *testing.T) {
	// current=5 of 20 → 1, ellipsis(0), 4, 5, 6, ellipsis(0), 20
	got := pageWindow(5, 20)
	want := []int{1, 0, 4, 5, 6, 0, 20}
	if len(got) != len(want) {
		t.Fatalf("pageWindow(5,20) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pageWindow(5,20) = %v, want %v", got, want)
		}
	}
}

func TestPageWindow_ShortRangeNoEllipsis(t *testing.T) {
	got := pageWindow(1, 3)
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("pageWindow(1,3) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pageWindow(1,3) = %v, want %v", got, want)
		}
	}
}

func TestPagination_MarksCurrentAndDisablesPrevAtStart(t *testing.T) {
	html := render(pagination(1, 5, "/list"))
	if !strings.Contains(html, "Prev") || !strings.Contains(html, "Next") {
		t.Error("pagination missing Prev/Next controls")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("pagination missing aria-current on the active page")
	}
	if !strings.Contains(html, `href="/list?page=2"`) {
		t.Error("pagination missing numbered page link")
	}
	// At page 1, Prev is disabled: rendered as a non-link with aria-disabled.
	if !strings.Contains(html, `aria-disabled="true"`) {
		t.Error("pagination should disable Prev at the first page")
	}
	if strings.Contains(html, `href="/list?page=0"`) {
		t.Error("pagination should not link to page 0")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestPageWindow|TestPagination' -v`
Expected: FAIL — `undefined: pageWindow` / `pagination`.

- [ ] **Step 3: Write minimal implementation**

Add to `views/kit_data.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestPageWindow|TestPagination' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add views/kit_data.go views/kit_data_test.go
git commit -m "feat: add numbered pagination component"
```

---

### Task 4: Replace the data table footer with pagination

**Files:**
- Modify: `views/kit_data.go` (`dataTable`, ~lines 95-140)
- Modify: `views/kit.go` (the `dataTable(...)` call, ~line 59)
- Test: `views/kit_data_test.go` (update `TestDataTable_RendersBadgesAndPagination`)

**Interfaces:**
- Consumes: `pagination(current, total int, baseHref string)` from Task 3.
- Produces: updated signature `dataTable(cols []string, rows []tableRow, rangeLabel string, current, total int, baseHref string) dom.Node`.

- [ ] **Step 1: Update the failing test**

In `views/kit_data_test.go`, replace `TestDataTable_RendersBadgesAndPagination` with:

```go
func TestDataTable_RendersBadgesAndPagination(t *testing.T) {
	rows := []tableRow{
		{Cells: []string{"INV-001", "Acme"}, Status: "Paid", Variant: ui.BadgeSuccess},
		{Cells: []string{"INV-002", "Globex"}, Status: "Overdue", Variant: ui.BadgeDanger},
	}
	html := render(dataTable([]string{"Invoice", "Customer", "Status", ""}, rows, "1–10 of 240", 5, 20, "#"))
	if !strings.Contains(html, "Paid") || !strings.Contains(html, "Overdue") {
		t.Error("dataTable missing status badges")
	}
	if !strings.Contains(html, "1–10 of 240") {
		t.Error("dataTable missing pagination range label")
	}
	if !strings.Contains(html, "<details") {
		t.Error("dataTable missing row-action <details> menu")
	}
	if !strings.Contains(html, "Prev") || !strings.Contains(html, "Next") {
		t.Error("dataTable missing Prev/Next controls")
	}
	if !strings.Contains(html, `aria-current="page"`) {
		t.Error("dataTable footer missing numbered pagination")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestDataTable_RendersBadgesAndPagination -v`
Expected: FAIL — too many arguments to `dataTable` (compile error).

- [ ] **Step 3: Update the implementation**

In `views/kit_data.go`, change the `dataTable` signature and its footer. New signature line:

```go
func dataTable(cols []string, rows []tableRow, rangeLabel string, current, total int, baseHref string) dom.Node {
```

Replace the footer `dom.Div(...)` block (the one containing `dom.Span(dom.Text(rangeLabel))` and the two `ui.Button(ui.ButtonGhost, ...)` controls) with:

```go
		dom.Div(
			dom.Class("flex items-center justify-between border-t border-[var(--color-border)] px-4 py-3 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Span(dom.Text(rangeLabel)),
			pagination(current, total, baseHref),
		),
```

- [ ] **Step 4: Update the caller in kit.go**

In `views/kit.go`, update the `dataTable` call to pass the new args:

```go
			dataTable([]string{"Invoice", "Customer", "Status", ""}, tableRows, "1–10 of 240", 5, 20, "#"),
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./views/`
Expected: PASS (all view tests, including the updated data-table test).

- [ ] **Step 6: Commit**

```bash
git add views/kit_data.go views/kit.go views/kit_data_test.go
git commit -m "refactor: use numbered pagination in data table footer"
```

---

### Task 5: Activity feed / timeline component

**Files:**
- Create: `views/kit_activity.go`
- Test: `views/kit_activity_test.go`

**Interfaces:**
- Consumes: existing `icon(name, size)` helper; `ui.BadgeVariant`.
- Produces:
  - `type activityItem struct { Icon, Title, Meta, Time string; Variant ui.BadgeVariant }`
  - `activityFeed(items []activityItem) dom.Node`
  - `dotColor(v ui.BadgeVariant) string`

- [ ] **Step 1: Write the failing test**

Create `views/kit_activity_test.go`:

```go
package views

import (
	"strings"
	"testing"

	"github.com/tunedmystic/rio/ui"
)

func TestActivityFeed_RendersItems(t *testing.T) {
	items := []activityItem{
		{Icon: "check", Title: "Invoice paid", Meta: "Acme Inc. · $1,200", Time: "2h ago", Variant: ui.BadgeSuccess},
		{Icon: "message", Title: "New comment", Meta: "Ada Lovelace", Time: "5h ago", Variant: ui.BadgeNeutral},
	}
	html := render(activityFeed(items))
	for _, want := range []string{"Invoice paid", "New comment", "2h ago", "5h ago"} {
		if !strings.Contains(html, want) {
			t.Errorf("activityFeed missing %q", want)
		}
	}
	// Timeline rail + round dot markers.
	if !strings.Contains(html, "border-l") {
		t.Error("activityFeed missing timeline rail")
	}
	if !strings.Contains(html, "rounded-full") {
		t.Error("activityFeed missing round dot marker")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestActivityFeed -v`
Expected: FAIL — `undefined: activityItem` / `activityFeed`.

- [ ] **Step 3: Write minimal implementation**

Create `views/kit_activity.go`:

```go
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
		rail = append(rail, dom.Div(
			dom.Class("relative"),
			// Dot marker straddling the rail; colored by variant.
			dom.Div(
				dom.Class("absolute -left-[37px] flex h-6 w-6 items-center justify-center rounded-full text-white shadow-sm"),
				dom.Style("background-color:"+dotColor(it.Variant)),
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./views/ -run TestActivityFeed -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add views/kit_activity.go views/kit_activity_test.go
git commit -m "feat: add activity feed timeline component"
```

---

### Task 6: Show breadcrumbs and activity feed in the kit

**Files:**
- Modify: `views/kit.go` (the `kitSection("Data & dashboard", ...)` block, ~lines 52-75)
- Test: `views/kit_test.go` (verify the kit page renders the new pieces)

**Interfaces:**
- Consumes: `breadcrumbs`, `activityFeed`, `activityItem` from Tasks 1 & 5.
- Produces: no new symbols; updates the kit showcase page.

- [ ] **Step 1: Write the failing test**

First check `views/kit_test.go` for how it renders the full kit page (it calls `Kit(testPageData(), config.Meta{...})` or similar). Add a test matching that pattern:

```go
func TestKit_RendersDashboardBreadcrumbsAndActivity(t *testing.T) {
	html := render(Kit(testPageData(), config.Meta{Title: "Kit"}))
	if !strings.Contains(html, `aria-label="Breadcrumb"`) {
		t.Error("kit missing breadcrumbs demo")
	}
	if !strings.Contains(html, "Activity") {
		t.Error("kit missing activity feed")
	}
}
```

If `views/kit_test.go` lacks imports for `strings`, `testing`, or `app/config`, add them. Match the exact `Kit(...)` argument style already used in that file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestKit_RendersDashboardBreadcrumbsAndActivity -v`
Expected: FAIL — no `aria-label="Breadcrumb"` in kit output (activity string may already appear via other copy; the breadcrumb assertion is the reliable failure).

- [ ] **Step 3: Update the implementation**

In `views/kit.go`, define sample activity data near the other kit sample slices (after `faqs`):

```go
	activity := []activityItem{
		{Icon: "check", Title: "Invoice INV-1001 paid", Meta: "Acme Inc. · $1,200", Time: "2h ago", Variant: ui.BadgeSuccess},
		{Icon: "database", Title: "Nightly backup completed", Meta: "System", Time: "6h ago", Variant: ui.BadgeNeutral},
		{Icon: "message", Title: "New comment on project", Meta: "Ada Lovelace", Time: "1d ago", Variant: ui.BadgeWarning},
	}
```

Then in the `kitSection("Data & dashboard", ...)` call, add breadcrumbs as the first child and the activity feed after the charts/usage row (before `emptyState`). The section becomes:

```go
		kitSection("Data & dashboard",
			breadcrumbs([]config.Link{
				{Text: "Home", Href: "#"},
				{Text: "Dashboard", Href: "#"},
				{Text: "Invoices", Href: "#"},
			}),
			dom.Div(
				dom.Class("grid gap-4 sm:grid-cols-2 lg:grid-cols-3"),
				metricCard("Revenue", "$48.2k", 12.5, []int{12, 14, 13, 18, 22, 20, 26}),
				metricCard("Active users", "3,182", 4.1, []int{30, 28, 33, 31, 35, 40, 44}),
				metricCard("Churn", "1.2%", -0.6, []int{9, 8, 8, 7, 6, 5, 5}),
			),
			dataTable([]string{"Invoice", "Customer", "Status", ""}, tableRows, "1–10 of 240", 5, 20, "#"),
			dom.Div(
				dom.Class("grid gap-6 lg:grid-cols-2"),
				dom.Div(
					dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm"),
					dom.Div(dom.Class("mb-4 font-semibold text-[var(--color-text)]"), dom.Text("Weekly signups")),
					barChart([]int{8, 14, 10, 18, 12, 22, 16}),
				),
				dom.Div(
					dom.Class("flex flex-col gap-5 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-sm"),
					usageMeter("Storage", 18, 50),
					usageMeter("API calls", 82000, 100000),
					usageMeter("Seats", 7, 10),
				),
			),
			activityFeed(activity),
			emptyState("layers", "No projects yet", "Create your first project to see it here.", ui.ButtonLink(ui.ButtonPrimary, "#", "New project")),
		),
```

(This mirrors the existing block; the only additions are the leading `breadcrumbs(...)`, the updated `dataTable(...)` args from Task 4, and `activityFeed(activity)` before `emptyState`. `config` is already imported in `kit.go`.)

- [ ] **Step 4: Run the full view suite**

Run: `go test ./views/`
Expected: PASS.

- [ ] **Step 5: Build CSS and eyeball the page (visual check)**

Run: `make tailwind && make run` (or the project's run command), open `http://localhost:3000/kit`, and confirm in the **Data & dashboard** section: the breadcrumb trail renders with chevrons; the data table footer shows numbered pages with the active page highlighted; the activity feed shows a vertical rail with colored dots. Adjust the dot offset (`-left-[37px]`) if the marker doesn't sit centered on the rail under the active theme.

- [ ] **Step 6: Commit**

```bash
git add views/kit.go views/kit_test.go
git commit -m "feat: showcase breadcrumbs and activity feed in the kit"
```

---

## Self-Review

**Spec coverage:**
- Pagination component + replaces table footer → Tasks 3, 4. ✓
- Activity feed/timeline with `activityItem` type → Task 5. ✓
- Breadcrumbs component → Task 1; wired into `/account` → Task 2; demoed in `/kit` → Task 6. ✓
- Placement in Data & dashboard section (breadcrumbs top, activity after charts, pagination via table footer) → Task 6 + Task 4. ✓
- Tests per component following `kit_data_test.go` conventions → each task's Step 1. ✓
- No-JS / token-driven / aria semantics → enforced in Global Constraints and each implementation. ✓
- Non-goals (no query parsing, fixed ellipsis window ±1, static activity slice) → respected: `pagination` only emits links; `pageWindow` hardcodes current ±1; `activityFeed` takes a static slice. ✓

**Placeholder scan:** No TBD/TODO/"similar to"/"add error handling" placeholders; every code step shows complete code. ✓

**Type consistency:** `pagination(current, total int, baseHref string)`, `pageLink(href, label string, current bool)`, `pageWindow(current, total int) []int`, `activityFeed([]activityItem)`, `breadcrumbs([]config.Link)`, and the updated `dataTable(cols, rows, rangeLabel, current, total, baseHref)` are referenced identically across Tasks 3–6. `dotColor` uses only existing `ui.BadgeVariant` values (no `BadgePrimary`). ✓
