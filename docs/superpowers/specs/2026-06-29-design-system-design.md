# Sub-project #6: Design System — Theme Presets + Component Kit + Showcase + Landing — Design

**Date:** 2026-06-29
**Status:** Approved, ready for implementation planning
**Branch:** `design-system` (off `main`)
**Target repo:** `github.com/TunedMystic/rio-project-template`
**Depends on:** sub-projects #1/#3/#4/#5 (all merged to `main`).

## Context

The template is functionally complete (auth, Google, billing, hardening) but its
visual identity is the original warm teal-on-cream starter look, and there is no
component library or showcase. This sub-project gives the template a
**world-class, data-forward SaaS look** with **two selectable theme presets**, a
reusable **component kit**, a **`/kit` showcase page**, and a **redesigned
landing page**. It does not add or change any product behavior or dependency.

## Decisions (locked during brainstorming, with the visual companion)

| Decision | Choice | Rationale |
|---|---|---|
| Visual direction | **Two presets: `ThemeSlateIndigo` (light) + `ThemeDusk` (dark)** | The two directions the user liked (mockups A and C). They are a natural light/dark pair. |
| Theme mechanism | **Compile-time presets via config** (no runtime toggle) | One `Theme` value in `config.New` selects the preset; it flows through the existing token→CSS-variable pipeline. Matches the clone-and-commit workflow; a runtime toggle is a future option. |
| Default preset | **`ThemeSlateIndigo` (light)** | Conventional default, broadest appeal; Dusk is one line away. |
| Component home | **`views/` (project-owned)** | Components live in the template, not the vendored `rio/ui`. Promote to `rio/ui` later by rule-of-three. |
| Charts | **Pure inline SVG, no JS** | Keeps the no-JS-framework posture; server-rendered sparklines/bars. |
| Landing | **In scope** (sequenced last) | The home page is the first impression; rebuild it as a real SaaS landing using the kit. |
| Theme forking | **Components stay theme-agnostic (token-driven only)** | No per-theme component code. The *token values* + tabular numerals + chart palette carry the "data" feel. (Glow effects from mockup C are out of scope to avoid theme-conditional code.) |

## Theming architecture

The current pipeline: `config` holds `ui.Tokens`; `views/layout.go` emits
`ui.StyleVars(tokens)` into the document `<style>`; every component reads
`var(--color-...)`. We extend this minimally:

- **`config.Theme`** — an enum: `ThemeSlateIndigo`, `ThemeDusk`. `config.New`
  sets `c.Theme` (default `ThemeSlateIndigo`) and builds `c.Tokens` from it via
  `themeSlateIndigo()` / `themeDusk()` (each returns a `ui.Tokens`).
- **Extended theme vars** — a small set of CSS variables the data components need
  that `ui.Tokens` does not cover, emitted in the layout `<style>` alongside
  `ui.StyleVars`: `--color-surface-raised`, `--color-ring` (focus/accent ring),
  `--color-on-danger` (legible text on a solid danger button — needed because the
  dark theme's `ColorDanger` is a light red that white text fails on), and a
  4-step chart ramp `--chart-1..--chart-4`. `config` exposes these per theme as an
  ordered `[]struct{Name, Value string}` (or a small `ThemeVars` type); `PageData`
  carries them so the layout can render them. No vendored code changes.
- **Existing components become fully token-driven** — remove hardcoded colors so
  both presets render correctly: the cream nav background (`bg-[#f8f5ee]` →
  `bg-[var(--color-surface)]`), the danger delete button's `text-white` →
  `text-[var(--color-on-danger)]`, and any other literal hex. Audit `views/*.go`
  for `#`/`text-white`/`bg-white` and replace with tokens.

### Exact palettes (the implementer uses these verbatim)

**`ThemeSlateIndigo` (light, default):**
```
FontFamily        "Inter", ui-sans-serif, system-ui, sans-serif
FontWeightHeading 800
RadiusBase        0.625rem
ColorBackground   #f8fafc   ColorSurface     #ffffff
ColorText         #0f172a   ColorTextMuted   #64748b   ColorBorder #e2e8f0
ColorPrimary      #4f46e5   OnPrimary        #ffffff
ColorSecondary    #475569   OnSecondary      #ffffff
ColorSuccess      #16a34a   ColorWarning     #b45309   ColorDanger #dc2626   ColorInfo #2563eb
-- extended --
--color-surface-raised #f1f5f9   --color-ring #6366f1   --color-on-danger #ffffff
--chart-1 #c7d2fe  --chart-2 #a5b4fc  --chart-3 #6366f1  --chart-4 #4f46e5
```

**`ThemeDusk` (dark):**
```
FontFamily        "Inter", ui-sans-serif, system-ui, sans-serif
FontWeightHeading 800
RadiusBase        0.625rem
ColorBackground   #0b1020   ColorSurface     #0f172a
ColorText         #f1f5f9   ColorTextMuted   #94a3b8   ColorBorder #1e293b
ColorPrimary      #22d3ee   OnPrimary        #06283d
ColorSecondary    #818cf8   OnSecondary      #0b1020
ColorSuccess      #34d399   ColorWarning     #fbbf24   ColorDanger #f87171   ColorInfo #38bdf8
-- extended --
--color-surface-raised #1e293b   --color-ring #22d3ee   --color-on-danger #0b1020
--chart-1 #155e75  --chart-2 #0891b2  --chart-3 #22d3ee  --chart-4 #67e8f9
```

Both themes must meet **WCAG AA** contrast for body and muted text on their
surfaces. The `Tokens` set is otherwise identical in shape to today's, so
`ui.StyleVars` keeps working unchanged.

## Component kit (`views/`)

Built as `dom.Node`-returning helpers, grouped into focused files
(`views/kit_*.go`). Each takes plain data (no global state) and is render-tested.

**Foundations** — token/color swatch grid, type-scale specimen, button set
(primary, secondary, ghost, danger), status badges/pills (success/warn/danger/
neutral/info), avatar + avatar-group.

**Data & dashboard (the signature set)** —
- `metricCard(label, value, deltaPct, spark []int)` — small label, big tabular
  number, colored delta (▲/▼), and an inline **SVG sparkline** using the chart ramp.
- `dataTable(cols, rows)` — header, body with status badges + a row-action menu
  (the `<details>` pattern), zebra-free hairline rows, and a **pagination
  footer** (Prev/Next + "1–10 of 240").
- `barChart(series)` — pure **SVG** vertical bars with axis baseline.
- `usageMeter(label, used, limit)` — progress bar with value text.
- `emptyState(icon, title, body, cta)` — centered empty state.

**Marketing & layout** —
- `hero(eyebrow, headline, sub, primaryCTA, secondaryCTA, visual)` — the landing hero.
- `featureHighlight(...)` — alternating image/text rows (image is an SVG/placeholder
  panel so it needs no asset).
- `featureGrid(items)` — icon + title + blurb cards.
- `pricingTable(plans)` — 2–3 plan columns, feature checklist, a highlighted plan
  (ties conceptually to the billing products).
- `testimonial(...)` + `logoCloud(...)`.
- `ctaBand(...)` — full-width call-to-action.
- `faq(items)` — `<details>` accordion.

**Feedback & forms** — `alert(variant, ...)` (reuse/extend `ui.Alert`), the
existing `ui` form fields shown in a form layout, a `toggle(...)` switch
(checkbox styled, no JS), and a `tabs(...)` strip (reuse the account-tab style).

All sample data for the showcase is **static fixtures** defined in the view/
handler — no backend.

## Pages

- **`/kit`** — a public showcase/style-guide page (`HandleKit` → `views.Kit`) that
  renders every component above, grouped with section headings, under the active
  theme. Add a "Kit" (or "Components") link to the footer (and optionally the nav)
  so it's discoverable. This is the "demo page" the user asked for.
- **Home `/`** — rebuilt as a real SaaS landing: hero → logo cloud → feature
  highlights → feature grid → metric/stat band → pricing → FAQ → CTA band, using
  the kit. Replaces the current four-feature-row layout.
- **Existing pages** (account, auth, messages, billing, premium/guide, 404) —
  inherit the new theme automatically via tokens; apply only light polish where a
  hardcoded color must become a token.
- **Responsive nav** — the header collapses to a `<details>` "hamburger" menu on
  small screens (reusing the no-JS disclosure pattern). Today's nav does not
  collapse; world-class requires it.

## Architecture & files

- `config/config.go` — `Theme` enum, `themeSlateIndigo()`/`themeDusk()`, default
  selection, extended-vars accessor; `PageData` carries the extended vars.
- `views/layout.go` — emit the extended theme vars next to `ui.StyleVars`.
- `views/components.go` — de-hardcode colors; responsive nav.
- `views/kit_foundations.go`, `views/kit_data.go`, `views/kit_marketing.go`,
  `views/kit_feedback.go` (new) — the component helpers, kept focused per file.
- `views/kit.go` (new) — `Kit(pd, meta)` assembling the showcase.
- `views/home.go` or `views/pages.go` — the rebuilt landing (`Home`).
- `handlers.go` / a new `handlers_kit.go` — `HandleKit`; `main.go` registers
  `/kit`.
- `static/css/styles.css` — regenerated (Tailwind scans the new `.go`).
- README — document the `Theme` config seam and the `/kit` page.

No new Go dependency. Only `views/`, `config/`, one handler, `main.go`, README,
and the generated CSS change.

## Accessibility & quality floor

- Both themes meet WCAG AA text contrast; visible `focus-visible` rings (using
  `--color-ring`); `prefers-reduced-motion` respected on hover/transition
  niceties; all interactive controls keyboard-reachable.
- Responsive from mobile up (collapsing nav, fluid grids, tables scroll on small
  screens).
- Semantic HTML (headings in order, `<nav>`/`<main>`/`<footer>`, `alt`/`aria`
  where needed).

## Out of scope

- Runtime light/dark **toggle** (presets are compile-time; a toggle is a future
  sub-project).
- Per-theme component code / glow effects.
- Real dashboards wired to live data (the data components use static fixtures on
  `/kit`).
- New dependencies; any backend/data-model change.

## Testing

Per component: a render test asserting the key structural output (e.g.
`metricCard` renders the value + a `<svg>` sparkline; `dataTable` renders the
status badges + pagination text; `pricingTable` renders each plan + the
highlighted one; `faq` renders `<details>`). Theme: a `config` test that each
preset produces non-empty tokens + the extended vars, and that `Theme` selection
swaps them. Page: `Kit`, the rebuilt `Home`, and the responsive nav each render
without error and contain representative markers. `go vet` + `go test ./...`
clean; CSS regenerated. Manual: eyeball both presets at `/kit` and `/` (flip the
config `Theme` value) — the only step that truly needs human eyes.

## Documentation

README gains a "Theming" section: the two presets, how to switch
(`Theme: ThemeDusk` in `config.New`), and a pointer to `/kit` as the component
reference. The visual-direction palettes above are the source of truth for the
exact hex values.
