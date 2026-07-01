# Dashboard Kit Components — Design

**Date:** 2026-06-30
**Status:** Approved (pending spec review)

## Goal

Complete the "Data & dashboard" section of the component kit by adding the three
components that finish what the existing kit already starts. All three are pure
server-rendered, token-driven, and require no JavaScript — consistent with every
other component in the system.

Chosen direction: **complete the dashboard** (over broad rounding-out or a JS
interactive layer).

## Scope

Three new components:

1. **Pagination** — numbered pager; **replaces** the data table's minimal
   Prev/Next footer and is reusable on any list page.
2. **Activity feed / timeline** — vertical event rail; fills the currently-empty
   "Activity" tab concept.
3. **Breadcrumbs** — navigation trail; demoed in `/kit` **and** wired into the
   real `/account` pages.

Explicitly **out of scope:** modals, toasts, tooltips, command palettes, date
pickers, and anything requiring a client-side JS layer. These conflict with the
template's no-JS ethos and are deferred.

## Components

### 1. Pagination

**Signature:** `pagination(current, total int, baseHref string) dom.Node`
**Location:** `views/kit_data.go` (lives with the table it serves).

Renders: `‹ Prev  1 … 4 [5] 6 … 20  Next ›`

- Current page: filled primary chip (`bg-[var(--color-primary)]
  text-[var(--color-on-primary)]`), not a link.
- Other pages: ghost links to `baseHref?page=N`, styled like the existing
  ghost/action controls (hover uses `bg-[var(--color-surface-raised)]`).
- Prev/Next: at the ends, render disabled — muted text, no `href`,
  `aria-disabled="true"`.
- Ellipsis: always show page 1 and page `total`; show a window of pages around
  `current` (current ±1); insert a non-interactive `…` where the sequence
  skips.
- Nav semantics: wrap in `<nav aria-label="Pagination">`; mark the current chip
  with `aria-current="page"`.
- Edge cases: `total <= 1` renders a single page 1 chip with both Prev/Next
  disabled; `current` is clamped into `[1, total]`.

**`dataTable` change:** the inline footer's `ui.Button(ui.ButtonGhost, "Prev")`
/ `"Next"` pair is replaced by a call to `pagination(...)`. The range label
(`"1–10 of 240"`) stays on the left of the footer; pagination sits on the right.
Because the pager still renders the words "Prev" and "Next", the existing
`dataTable` test assertions for those strings continue to hold; the test is
updated to also assert a numbered page and `aria-current`.

### 2. Activity feed / timeline

**Type:**

```go
type activityItem struct {
    Icon    string          // reuses the existing icon() set
    Title   string
    Meta    string          // e.g. "Ada Lovelace · invited a member"
    Time    string          // e.g. "2h ago"
    Variant ui.BadgeVariant // drives the dot accent color
}
```

**Signature:** `activityFeed(items []activityItem) dom.Node`
**Location:** new file `views/kit_activity.go`.

Renders a vertical list on a continuous rail:

- A left vertical hairline (`border-l border-[var(--color-border)]`) runs behind
  the items.
- Each item: a small round dot marker sitting on the rail, colored by
  `Variant` (mapping badge variants to `--color-success` / `--color-warning` /
  `--color-danger` / `--color-primary`), containing the `Icon`.
- Body: bold `Title`, muted `Meta` line, and a right-aligned tabular `Time`
  (`[font-variant-numeric:tabular-nums] text-[var(--color-text-muted)]`).
- Wrapped in the same surface-card chrome used elsewhere
  (`rounded-[var(--radius-base)] border ... bg-[var(--color-surface)] p-5
  shadow-sm`).

### 3. Breadcrumbs

**Signature:** `breadcrumbs(trail []config.Link) dom.Node`
**Location:** `views/components.go` (navigation chrome, with navbar/footer).

- Renders `<nav aria-label="Breadcrumb">` containing an ordered trail.
- All items except the last: muted `<a>` links
  (`text-[var(--color-text-muted)] hover:text-[var(--color-text)]`), separated
  by the existing `icon("chevron-right", …)`.
- Last item: bold current page (`text-[var(--color-text)]`), rendered as text
  (not a link), with `aria-current="page"`.

**`/account` wiring:** in `accountShell` (`views/account.go`), render a
breadcrumb trail at the top of the section content, above the tab nav:
`Home › Account › <active tab label>`. The active tab label is looked up from
`accountTabs` by `av.Active`. Trail: `[{Home, /}, {Account, /account},
{<label>, <current href>}]`.

## Placement in `/kit`

All three go inside the existing **Data & dashboard** `kitSection` in
`views/kit.go`:

- **Breadcrumbs** — at the top of the section, as a navigation example
  (e.g. `Home › Dashboard › Invoices`).
- **Activity feed** — after the charts + usage-meter row, before the empty
  state.
- **Pagination** — arrives via the updated `dataTable` footer (no separate demo
  needed), matching the "replace table footer" decision.

## Testing

Following `views/kit_data_test.go` conventions (`render(...)` →
`strings.Contains`):

- **pagination:** renders `Prev`/`Next`; renders numbered pages; marks the
  current page with `aria-current="page"`; a disabled Prev at page 1 has no
  `href`/carries `aria-disabled`.
- **activityFeed:** renders each item's `Title` and `Time`; renders a dot marker.
- **breadcrumbs:** renders every trail label; the last label carries
  `aria-current="page"` and is not wrapped in an `<a href>`.
- **dataTable (updated):** keep the existing badge/range/details assertions; add
  assertions for a numbered page and `aria-current` now that the footer uses
  `pagination`.

## Non-goals / YAGNI

- No query-param parsing or real server-side paging logic — `pagination` only
  emits the correct links/markup; handlers wire real data later.
- No configurable window size for the ellipsis (fixed current ±1) until a second
  caller needs otherwise.
- Activity feed takes a static slice; no live/streaming updates.
