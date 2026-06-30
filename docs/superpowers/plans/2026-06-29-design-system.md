# Design System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the template a world-class, data-forward SaaS look via two compile-time theme presets, a reusable component kit, a `/kit` showcase page, and a rebuilt landing page — with no new product behavior or dependency.

**Architecture:** Extend the existing `config.Tokens` → `ui.StyleVars` → `var(--color-*)` pipeline with a `config.Theme` enum that selects a preset (`ThemeSlateIndigo` default, or `ThemeDusk`) and a small set of extended CSS vars the data components need. Components live in `views/` as `dom.Node`-returning helpers grouped into focused `kit_*.go` files; charts are pure inline SVG (no JS). The `/kit` page renders every component under the active theme using static fixtures.

**Tech Stack:** Go 1.26.4 (module `app`), the vendored `github.com/tunedmystic/rio` `dom`/`ui` packages, Tailwind v4 (via `make tailwind`), stdlib `testing` + `httptest` (no testify).

## Global Constraints

- No new Go dependency; only `views/`, `config/`, one handler, `main.go`, README, and generated CSS change. (spec: "Architecture & files")
- Charts are **pure inline SVG via `dom.Raw`** — no JS, no typed `Svg`/`Path` constructors exist. (spec: "Charts")
- Components stay **theme-agnostic / token-driven only** — no per-theme component code, no glow effects. (spec: decisions table)
- Both presets must meet **WCAG AA** contrast for body and muted text (verified manually). (spec: "Accessibility & quality floor")
- All sample data for `/kit` is **static fixtures** defined in the view/handler — no backend. (spec: "Component kit")
- Default preset is **`ThemeSlateIndigo`** (light); `ThemeDusk` is one line away in `config.New`. (spec: "Default preset")
- Font family `"Inter", ui-sans-serif, system-ui, sans-serif`; `FontWeightHeading` `800`; `RadiusBase` `0.625rem`; font sizes keep today's values (Sm 16px, Base 18px, Lg 20px, Xl 24px, 2xl 30px). (spec: "Exact palettes")
- Tests are `package views` / `package config` / `package main` internal tests using stdlib `testing`; render `dom.Node` via the existing `render(n)` helper in `views/views_test.go` and assert with `strings.Contains`. (codebase convention)
- After any change that adds new Tailwind class literals, CSS is regenerated with `make tailwind`. Render tests do **not** depend on CSS.

---

## File Structure

**Modified:**
- `config/config.go` — add `Theme` enum, `themeSlateIndigo()`/`themeDusk()`, `ThemeVar` type + `Theme.Vars()`, `Theme.Tokens()`; add `Theme` field to `Config` and `ThemeVars` to `PageData`; default-select the preset in `New`; add a `Kit` footer link.
- `views/layout.go` — emit the extended theme vars (`themeVarsStyle`) next to `pd.Tokens.StyleVars()`.
- `views/components.go` — de-hardcode the navbar background; make the nav responsive (`<details>` hamburger).
- `views/account.go` — de-hardcode the delete button's `text-white` → `text-[var(--color-on-danger)]`.
- `views/pages.go` — rebuild `Home` as a SaaS landing using the kit.
- `main.go` — register `/kit`.
- `README.md` — add a "Theming" section.
- `static/css/styles.css` — regenerated.

**Created:**
- `views/kit_foundations.go` — swatch grid, type scale, button set, status badges, avatar + avatar-group.
- `views/kit_data.go` — `metricCard`, `sparkline`, `dataTable`, `barChart`, `usageMeter`, `emptyState`.
- `views/kit_marketing.go` — `hero`, `featureHighlight`, `featureGrid`, `pricingTable`, `testimonial`, `logoCloud`, `ctaBand`, `faq`.
- `views/kit_feedback.go` — `formShowcase`, `toggle`, `tabStrip`.
- `views/kit.go` — `Kit(pd, meta)` assembling the showcase + fixtures.
- `handlers_kit.go` — `HandleKit`.
- Test files: `config/theme_test.go`, `views/kit_foundations_test.go`, `views/kit_data_test.go`, `views/kit_marketing_test.go`, `views/kit_feedback_test.go`, `views/kit_test.go`, `handlers_kit_test.go`.

---

### Task 1: Theme presets and extended vars in `config`

Pure functions only — the `Theme` enum, the two preset token builders, the extended-var type and accessor. No wiring yet, fully unit-testable.

**Files:**
- Create: `config/theme.go`
- Modify: `config/config.go` (remove `defaultTokens`, lines 253–280)
- Test: `config/theme_test.go`

**Interfaces:**
- Consumes: `ui.Tokens` (vendored struct — fields per spec palette).
- Produces:
  - `type Theme int` with `const ( ThemeSlateIndigo Theme = iota; ThemeDusk )`
  - `type ThemeVar struct { Name, Value string }`
  - `func (t Theme) Tokens() ui.Tokens`
  - `func (t Theme) Vars() []ThemeVar`

- [ ] **Step 1: Write the failing test**

Create `config/theme_test.go`:

```go
package config

import "testing"

func TestThemeTokens_BothPresetsPopulated(t *testing.T) {
	for _, th := range []Theme{ThemeSlateIndigo, ThemeDusk} {
		tk := th.Tokens()
		if tk.ColorPrimary == "" || tk.ColorBackground == "" || tk.ColorText == "" {
			t.Errorf("theme %d: tokens have empty core colors", th)
		}
		if tk.FontWeightHeading != "800" {
			t.Errorf("theme %d: FontWeightHeading = %q, want 800", th, tk.FontWeightHeading)
		}
		if tk.RadiusBase != "0.625rem" {
			t.Errorf("theme %d: RadiusBase = %q, want 0.625rem", th, tk.RadiusBase)
		}
	}
}

func TestThemeTokens_SelectionSwapsPalette(t *testing.T) {
	light := ThemeSlateIndigo.Tokens()
	dark := ThemeDusk.Tokens()
	if light.ColorPrimary == dark.ColorPrimary {
		t.Error("expected the two presets to differ in ColorPrimary")
	}
	if light.ColorPrimary != "#4f46e5" {
		t.Errorf("SlateIndigo ColorPrimary = %q, want #4f46e5", light.ColorPrimary)
	}
	if dark.ColorPrimary != "#22d3ee" {
		t.Errorf("Dusk ColorPrimary = %q, want #22d3ee", dark.ColorPrimary)
	}
}

func TestThemeVars_IncludeExtendedNames(t *testing.T) {
	for _, th := range []Theme{ThemeSlateIndigo, ThemeDusk} {
		got := map[string]string{}
		for _, v := range th.Vars() {
			got[v.Name] = v.Value
		}
		for _, name := range []string{
			"--color-surface-raised", "--color-ring", "--color-on-danger",
			"--chart-1", "--chart-2", "--chart-3", "--chart-4",
		} {
			if got[name] == "" {
				t.Errorf("theme %d: missing extended var %s", th, name)
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestTheme -v`
Expected: FAIL — `undefined: Theme`, `undefined: ThemeSlateIndigo`, etc.

- [ ] **Step 3: Create `config/theme.go` with the presets**

```go
package config

import "github.com/tunedmystic/rio/ui"

// Theme selects a compile-time visual preset. config.New sets one (default
// ThemeSlateIndigo); it flows through the token -> CSS-variable pipeline.
type Theme int

const (
	ThemeSlateIndigo Theme = iota // light, default
	ThemeDusk                     // dark
)

// ThemeVar is a single extended CSS custom property the data components need
// beyond the ui.Tokens set. The layout emits these into :root.
type ThemeVar struct {
	Name  string
	Value string
}

// Tokens returns the ui.Tokens for the preset.
func (t Theme) Tokens() ui.Tokens {
	switch t {
	case ThemeDusk:
		return themeDusk()
	default:
		return themeSlateIndigo()
	}
}

// Vars returns the extended CSS variables for the preset, in render order.
func (t Theme) Vars() []ThemeVar {
	switch t {
	case ThemeDusk:
		return []ThemeVar{
			{"--color-surface-raised", "#1e293b"},
			{"--color-ring", "#22d3ee"},
			{"--color-on-danger", "#0b1020"},
			{"--chart-1", "#155e75"},
			{"--chart-2", "#0891b2"},
			{"--chart-3", "#22d3ee"},
			{"--chart-4", "#67e8f9"},
		}
	default:
		return []ThemeVar{
			{"--color-surface-raised", "#f1f5f9"},
			{"--color-ring", "#6366f1"},
			{"--color-on-danger", "#ffffff"},
			{"--chart-1", "#c7d2fe"},
			{"--chart-2", "#a5b4fc"},
			{"--chart-3", "#6366f1"},
			{"--chart-4", "#4f46e5"},
		}
	}
}

// themeSlateIndigo is the light default: indigo accent on slate neutrals.
func themeSlateIndigo() ui.Tokens {
	return ui.Tokens{
		FontFamily:        `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:        "16px",
		FontSizeBase:      "18px",
		FontSizeLg:        "20px",
		FontSizeXl:        "24px",
		FontSize2xl:       "30px",
		ColorPrimary:      "#4f46e5",
		OnPrimary:         "#ffffff",
		ColorSecondary:    "#475569",
		OnSecondary:       "#ffffff",
		ColorBackground:   "#f8fafc",
		ColorSurface:      "#ffffff",
		ColorText:         "#0f172a",
		ColorTextMuted:    "#64748b",
		ColorBorder:       "#e2e8f0",
		ColorSuccess:      "#16a34a",
		ColorWarning:      "#b45309",
		ColorDanger:       "#dc2626",
		ColorInfo:         "#2563eb",
		RadiusBase:        "0.625rem",
		FontWeightHeading: "800",
	}
}

// themeDusk is the dark preset: cyan accent on deep navy-slate surfaces.
func themeDusk() ui.Tokens {
	return ui.Tokens{
		FontFamily:        `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:        "16px",
		FontSizeBase:      "18px",
		FontSizeLg:        "20px",
		FontSizeXl:        "24px",
		FontSize2xl:       "30px",
		ColorPrimary:      "#22d3ee",
		OnPrimary:         "#06283d",
		ColorSecondary:    "#818cf8",
		OnSecondary:       "#0b1020",
		ColorBackground:   "#0b1020",
		ColorSurface:      "#0f172a",
		ColorText:         "#f1f5f9",
		ColorTextMuted:    "#94a3b8",
		ColorBorder:       "#1e293b",
		ColorSuccess:      "#34d399",
		ColorWarning:      "#fbbf24",
		ColorDanger:       "#f87171",
		ColorInfo:         "#38bdf8",
		RadiusBase:        "0.625rem",
		FontWeightHeading: "800",
	}
}
```

- [ ] **Step 4: Remove the old `defaultTokens` from `config/config.go`**

Delete the entire `defaultTokens()` function (currently `config/config.go:253-280`). Task 2 replaces its single call site. Leave everything else untouched for now (the build is briefly broken until Step 6 of this task — that's fine; the commit happens after it compiles).

- [ ] **Step 5: Fix the call site so the package compiles**

In `config/config.go`, in `New(...)`, change the `Tokens` line in the `Config{...}` literal from:

```go
		Tokens:       defaultTokens(),
```

to:

```go
		Tokens:       ThemeSlateIndigo.Tokens(),
```

(Task 2 introduces the explicit `Theme` field and makes this derive from it; this interim line keeps the package building and tests green.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./config/ -run TestTheme -v && go build ./...`
Expected: PASS (all three theme tests), build clean.

- [ ] **Step 7: Commit**

```bash
git add config/theme.go config/config.go config/theme_test.go
git commit -m "feat(config): add ThemeSlateIndigo/ThemeDusk presets and extended vars"
```

---

### Task 2: Wire `Theme` into `Config` and `PageData`

Add the explicit `Theme` field, derive `Tokens` from it, and carry the extended vars on `PageData` so the layout can render them.

**Files:**
- Modify: `config/config.go` (`Config` struct ~lines 65–87; `New` ~lines 92–131; `PageData` struct ~lines 53–62; `PageData()` method ~lines 159–170)
- Test: `config/theme_test.go` (append)

**Interfaces:**
- Consumes: `Theme`, `Theme.Tokens()`, `Theme.Vars()`, `ThemeVar` (Task 1).
- Produces:
  - `Config.Theme Theme` field
  - `PageData.ThemeVars []ThemeVar` field
  - `Config.New` sets `c.Theme = ThemeSlateIndigo` and `c.Tokens = c.Theme.Tokens()`
  - `Config.PageData()` sets `ThemeVars: c.Theme.Vars()`

- [ ] **Step 1: Write the failing test**

Append to `config/theme_test.go`:

```go
func TestConfigDefaultTheme(t *testing.T) {
	c := New("debug", "v1test")
	if c.Theme != ThemeSlateIndigo {
		t.Errorf("default Theme = %d, want ThemeSlateIndigo", c.Theme)
	}
	if c.Tokens.ColorPrimary != "#4f46e5" {
		t.Errorf("default Tokens.ColorPrimary = %q, want #4f46e5", c.Tokens.ColorPrimary)
	}
}

func TestPageDataCarriesThemeVars(t *testing.T) {
	c := New("debug", "v1test")
	pd := c.PageData()
	if len(pd.ThemeVars) == 0 {
		t.Fatal("PageData.ThemeVars is empty")
	}
	var hasRing bool
	for _, v := range pd.ThemeVars {
		if v.Name == "--color-ring" && v.Value != "" {
			hasRing = true
		}
	}
	if !hasRing {
		t.Error("PageData.ThemeVars missing --color-ring")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run 'TestConfigDefaultTheme|TestPageDataCarriesThemeVars' -v`
Expected: FAIL — `c.Theme undefined`, `pd.ThemeVars undefined`.

- [ ] **Step 3: Add the `Theme` field to the `Config` struct**

In `config/config.go`, in the `Config` struct, add the field next to `Tokens`:

```go
	Tokens              ui.Tokens
	Theme               Theme
```

- [ ] **Step 4: Add `ThemeVars` to the `PageData` struct**

In the `PageData` struct, add after `Tokens`:

```go
	Tokens        ui.Tokens
	ThemeVars     []ThemeVar
```

- [ ] **Step 5: Set the theme in `New` and derive tokens from it**

In `New`, replace the interim line from Task 1:

```go
		Tokens:       ThemeSlateIndigo.Tokens(),
```

with the explicit field, and derive `Tokens` right after the struct literal is assigned to `c`. The literal line becomes:

```go
		Theme:        ThemeSlateIndigo, // <-- compile-time preset; set ThemeDusk for dark
```

Then immediately after the `c := Config{ ... }` block closes (before `c.DBPath = ...`), add:

```go
	c.Tokens = c.Theme.Tokens()
```

- [ ] **Step 6: Carry the extended vars in `PageData()`**

In the `PageData()` method, add the `ThemeVars` field to the returned literal:

```go
		Tokens:        c.Tokens,
		ThemeVars:     c.Theme.Vars(),
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./config/ -v && go build ./...`
Expected: PASS (all config tests), build clean.

- [ ] **Step 8: Commit**

```bash
git add config/config.go config/theme_test.go
git commit -m "feat(config): select Theme in New and carry extended vars on PageData"
```

---

### Task 3: Emit extended theme vars in the layout

Render the `PageData.ThemeVars` into a `:root` `<style>` block next to `pd.Tokens.StyleVars()`.

**Files:**
- Modify: `views/layout.go` (import block lines 1–7; `Page` head, after line 25)
- Test: `views/views_test.go` (append; or new `views/layout_extra_test.go`)

**Interfaces:**
- Consumes: `config.PageData.ThemeVars` (Task 2), `dom.StyleEl`, `dom.Raw`.
- Produces: `func themeVarsStyle(vars []config.ThemeVar) dom.Node` (unexported, in `views`).

- [ ] **Step 1: Write the failing test**

Append to `views/views_test.go`:

```go
func TestPage_EmitsExtendedThemeVars(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "t", Description: "d"}
	var b bytes.Buffer
	_ = Page(pd, meta, nil).Render(&b)
	html := b.String()

	for _, want := range []string{"--color-ring:", "--chart-1:", "--color-on-danger:"} {
		if !strings.Contains(html, want) {
			t.Errorf("Page output missing extended var %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestPage_EmitsExtendedThemeVars -v`
Expected: FAIL — output does not contain `--color-ring:`.

- [ ] **Step 3: Add the `strings` import and the helper**

In `views/layout.go`, update the import block to add `strings`:

```go
import (
	"strings"

	"app/config"

	"github.com/tunedmystic/rio/dom"
)
```

Add the helper at the end of `views/layout.go`:

```go
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
```

- [ ] **Step 4: Call the helper in the document head**

In `Page`, immediately after `pd.Tokens.StyleVars(),` (line 25), add:

```go
			pd.Tokens.StyleVars(),
			themeVarsStyle(pd.ThemeVars),
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./views/ -run TestPage -v`
Expected: PASS (both the existing `TestPage_RendersHeadAndChrome` and the new test).

- [ ] **Step 6: Commit**

```bash
git add views/layout.go views/views_test.go
git commit -m "feat(views): emit extended theme vars in document head"
```

---

### Task 4: De-hardcode colors and make the nav responsive

Remove the two literal colors and add a `<details>` hamburger so the header collapses on small screens.

**Files:**
- Modify: `views/components.go` (`navbar`, lines 14–33)
- Modify: `views/account.go` (line 307, the `text-white` delete button)
- Test: `views/components_test.go` (create)

**Interfaces:**
- Consumes: `config.PageData`, `pd.HeaderLinks`, `navLink`, `brand`, `accountAvatar`, `icon`, `--color-surface`, `--color-on-danger` (Task 3 makes the latter available).
- Produces: a `navbar(pd)` that renders both a desktop nav (`hidden sm:flex`) and a `<details>` mobile menu (`sm:hidden`).

- [ ] **Step 1: Write the failing test**

Create `views/components_test.go`:

```go
package views

import (
	"strings"
	"testing"
)

func TestNavbar_NoHardcodedCream(t *testing.T) {
	html := render(navbar(testPageData()))
	if strings.Contains(html, "#f8f5ee") {
		t.Error("navbar still contains hardcoded cream #f8f5ee")
	}
	if !strings.Contains(html, "bg-[var(--color-surface)]") {
		t.Error("navbar should use bg-[var(--color-surface)]")
	}
}

func TestNavbar_HasResponsiveDisclosure(t *testing.T) {
	html := render(navbar(testPageData()))
	if !strings.Contains(html, "<details") {
		t.Error("navbar missing <details> hamburger for small screens")
	}
	if !strings.Contains(html, "sm:hidden") || !strings.Contains(html, "hidden sm:flex") {
		t.Error("navbar missing responsive show/hide classes")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestNavbar -v`
Expected: FAIL — navbar contains `#f8f5ee`; no `<details>`.

- [ ] **Step 3: Rewrite `navbar` in `views/components.go`**

Replace the `navbar` function (lines 14–33) with:

```go
func navbar(pd config.PageData) dom.Node {
	// Build the shared link list once; reused by the desktop nav and the
	// mobile disclosure so there is a single source of truth.
	linkNodes := func() []dom.Node {
		nodes := make([]dom.Node, 0, len(pd.HeaderLinks)+1)
		for _, l := range pd.HeaderLinks {
			nodes = append(nodes, navLink(l))
		}
		if pd.Account.LoggedIn {
			nodes = append(nodes, accountAvatar(pd.Account))
		} else {
			nodes = append(nodes, navLink(config.Link{Text: "Log in", Href: "/login"}))
		}
		return nodes
	}

	desktop := dom.Nav(withClass("hidden items-center gap-6 sm:flex", linkNodes())...)

	mobile := dom.Details(
		dom.Class("relative sm:hidden"),
		dom.Summary(
			dom.Class("flex h-9 w-9 cursor-pointer list-none items-center justify-center rounded-[var(--radius-base)] text-[var(--color-text)] [&::-webkit-details-marker]:hidden"),
			dom.Aria("label", "Toggle navigation menu"),
			icon("menu", 22),
		),
		dom.Nav(withClass(
			"absolute right-0 z-20 mt-2 flex w-44 flex-col gap-1 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] p-2 shadow-lg",
			linkNodes())...),
	)

	return dom.Header(
		dom.Class("border-b border-[var(--color-border)] bg-[var(--color-surface)]"),
		dom.Div(
			dom.Class("mx-auto flex w-full max-w-5xl items-center justify-between px-5 py-4"),
			brand(pd),
			desktop,
			mobile,
		),
	)
}
```

- [ ] **Step 4: Add a `menu` icon case**

In `views/components.go`, in the `icon(name string, size int)` switch, add a case (the standard hamburger lines):

```go
	case "menu":
		body = `<line x1="4" y1="6" x2="20" y2="6"/><line x1="4" y1="12" x2="20" y2="12"/><line x1="4" y1="18" x2="20" y2="18"/>`
```

- [ ] **Step 5: De-hardcode the delete button in `views/account.go`**

At `views/account.go:307`, change `text-white` to `text-[var(--color-on-danger)]` in that button's class string. Verify no other literal `text-white`/`bg-white`/`#`-hex remain in `views/`:

Run: `grep -rnE 'text-white|bg-white|bg-\[#|text-\[#' views/`
Expected: no matches (empty output).

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestNavbar' -v && go build ./...`
Expected: PASS, build clean.

- [ ] **Step 7: Commit**

```bash
git add views/components.go views/account.go views/components_test.go
git commit -m "feat(views): token-drive nav background and add responsive hamburger menu"
```

---

### Task 5: Foundations kit (`views/kit_foundations.go`)

Token swatch grid, type-scale specimen, button set, status badges/pills, avatar + avatar-group. Reuses `ui.Button`/`ui.Badge` where they fit.

**Files:**
- Create: `views/kit_foundations.go`
- Test: `views/kit_foundations_test.go`

**Interfaces:**
- Consumes: `ui.Button`, `ui.ButtonPrimary/Secondary/Ghost/Danger`, `ui.Badge`, `ui.BadgeNeutral/Success/Warning/Danger`, `dom.*`, `initial` (existing helper in `components.go`).
- Produces:
  - `func colorSwatches() dom.Node`
  - `func typeScale() dom.Node`
  - `func buttonSet() dom.Node`
  - `func statusBadges() dom.Node`
  - `func avatar(name string) dom.Node`
  - `func avatarGroup(names []string) dom.Node`

- [ ] **Step 1: Write the failing tests**

Create `views/kit_foundations_test.go`:

```go
package views

import (
	"strings"
	"testing"
)

func TestColorSwatches_RendersTokenVars(t *testing.T) {
	html := render(colorSwatches())
	for _, want := range []string{"--color-primary", "--color-surface-raised", "--color-border"} {
		if !strings.Contains(html, want) {
			t.Errorf("colorSwatches missing swatch for %s", want)
		}
	}
}

func TestTypeScale_RendersSpecimens(t *testing.T) {
	html := render(typeScale())
	for _, want := range []string{"font-size-2xl", "font-size-base", "font-size-sm"} {
		if !strings.Contains(html, want) {
			t.Errorf("typeScale missing %s", want)
		}
	}
}

func TestButtonSet_RendersAllVariants(t *testing.T) {
	html := render(buttonSet())
	for _, want := range []string{"Primary", "Secondary", "Ghost", "Danger"} {
		if !strings.Contains(html, want) {
			t.Errorf("buttonSet missing %q button", want)
		}
	}
}

func TestStatusBadges_RendersVariants(t *testing.T) {
	html := render(statusBadges())
	for _, want := range []string{"Success", "Warning", "Danger", "Neutral", "Info"} {
		if !strings.Contains(html, want) {
			t.Errorf("statusBadges missing %q", want)
		}
	}
}

func TestAvatarGroup_RendersInitials(t *testing.T) {
	html := render(avatarGroup([]string{"Ada Lovelace", "Grace Hopper"}))
	if !strings.Contains(html, "A") || !strings.Contains(html, "G") {
		t.Error("avatarGroup missing member initials")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestColorSwatches|TestTypeScale|TestButtonSet|TestStatusBadges|TestAvatarGroup' -v`
Expected: FAIL — `undefined: colorSwatches`, etc.

- [ ] **Step 3: Implement `views/kit_foundations.go`**

```go
package views

import (
	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// colorSwatches renders a grid of the theme's color tokens as labeled chips.
func colorSwatches() dom.Node {
	swatches := []struct{ label, varName string }{
		{"Primary", "--color-primary"},
		{"Secondary", "--color-secondary"},
		{"Surface", "--color-surface"},
		{"Surface raised", "--color-surface-raised"},
		{"Border", "--color-border"},
		{"Success", "--color-success"},
		{"Warning", "--color-warning"},
		{"Danger", "--color-danger"},
		{"Info", "--color-info"},
		{"Ring", "--color-ring"},
	}
	cells := make([]dom.Node, 0, len(swatches)+1)
	cells = append(cells, dom.Class("grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-5"))
	for _, s := range swatches {
		cells = append(cells, dom.Div(
			dom.Class("rounded-[var(--radius-base)] border border-[var(--color-border)] overflow-hidden"),
			dom.Div(dom.Class("h-14 w-full"), dom.Style("background:var("+s.varName+")")),
			dom.Div(
				dom.Class("px-3 py-2 text-[length:var(--font-size-sm)]"),
				dom.Div(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(s.label)),
				dom.Div(dom.Class("text-[var(--color-text-muted)]"), dom.Text(s.varName)),
			),
		))
	}
	return dom.Div(cells...)
}

// typeScale renders one specimen line per font-size token.
func typeScale() dom.Node {
	steps := []struct{ size, label string }{
		{"--font-size-2xl", "font-size-2xl"},
		{"--font-size-xl", "font-size-xl"},
		{"--font-size-lg", "font-size-lg"},
		{"--font-size-base", "font-size-base"},
		{"--font-size-sm", "font-size-sm"},
	}
	rows := make([]dom.Node, 0, len(steps)+1)
	rows = append(rows, dom.Class("flex flex-col gap-3"))
	for _, s := range steps {
		rows = append(rows, dom.Div(
			dom.Class("flex items-baseline gap-4 border-b border-[var(--color-border)] pb-3"),
			dom.Span(
				dom.Class("tracking-tight text-[var(--color-text)]"),
				dom.Style("font-size:var("+s.size+")"),
				dom.Text("The quick brown fox"),
			),
			dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(s.label)),
		))
	}
	return dom.Div(rows...)
}

// buttonSet renders the four button variants from the ui kit.
func buttonSet() dom.Node {
	return dom.Div(
		dom.Class("flex flex-wrap items-center gap-3"),
		ui.Button(ui.ButtonPrimary, "Primary"),
		ui.Button(ui.ButtonSecondary, "Secondary"),
		ui.Button(ui.ButtonGhost, "Ghost"),
		ui.Button(ui.ButtonDanger, "Danger"),
	)
}

// pill renders a small rounded status label using a semantic color token.
func pill(label, colorVar string) dom.Node {
	return dom.Span(
		dom.Class("inline-flex items-center rounded-full px-2.5 py-0.5 text-[length:var(--font-size-sm)] font-medium"),
		dom.Style("background:color-mix(in srgb, var("+colorVar+") 14%, transparent);color:var("+colorVar+")"),
		dom.Text(label),
	)
}

// statusBadges renders the badge variants plus an Info pill (ui.Badge has no
// Info variant, so it is rendered as a token-driven pill).
func statusBadges() dom.Node {
	return dom.Div(
		dom.Class("flex flex-wrap items-center gap-3"),
		ui.Badge(ui.BadgeSuccess, "Success"),
		ui.Badge(ui.BadgeWarning, "Warning"),
		ui.Badge(ui.BadgeDanger, "Danger"),
		ui.Badge(ui.BadgeNeutral, "Neutral"),
		pill("Info", "--color-info"),
	)
}

// avatar renders a circular initial badge for a name.
func avatar(name string) dom.Node {
	return dom.Div(
		dom.Class("flex h-9 w-9 items-center justify-center rounded-full border-2 border-[var(--color-surface)] bg-[var(--color-primary)] text-[var(--color-on-primary)] text-[length:var(--font-size-sm)] font-semibold"),
		dom.Text(initial(name)),
	)
}

// avatarGroup renders overlapping avatars for a list of names.
func avatarGroup(names []string) dom.Node {
	items := make([]dom.Node, 0, len(names)+1)
	items = append(items, dom.Class("flex -space-x-2"))
	for _, n := range names {
		items = append(items, avatar(n))
	}
	return dom.Div(items...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestColorSwatches|TestTypeScale|TestButtonSet|TestStatusBadges|TestAvatarGroup' -v`
Expected: PASS (all five).

- [ ] **Step 5: Commit**

```bash
git add views/kit_foundations.go views/kit_foundations_test.go
git commit -m "feat(views): foundations kit — swatches, type scale, buttons, badges, avatars"
```

---

### Task 6: Data & dashboard kit (`views/kit_data.go`)

The signature data set: metric cards with SVG sparklines, a data table with status badges + a `<details>` row-action menu + pagination footer, an SVG bar chart, a usage meter, and an empty state.

**Files:**
- Create: `views/kit_data.go`
- Test: `views/kit_data_test.go`

**Interfaces:**
- Consumes: `ui.Badge`/badge variants, `icon`, `dom.*` (incl. `Table/Thead/Tbody/Tr/Th/Td`, `Details/Summary`), `fmt`, `strings`.
- Produces:
  - `type tableRow struct { Cells []string; Status string; Variant ui.BadgeVariant }`
  - `func metricCard(label, value string, deltaPct float64, spark []int) dom.Node`
  - `func sparkline(data []int) dom.Node`
  - `func dataTable(cols []string, rows []tableRow, rangeLabel string) dom.Node`
  - `func barChart(series []int) dom.Node`
  - `func usageMeter(label string, used, limit int) dom.Node`
  - `func emptyState(iconName, title, body string, cta dom.Node) dom.Node`

- [ ] **Step 1: Write the failing tests**

Create `views/kit_data_test.go`:

```go
package views

import (
	"strings"
	"testing"

	"github.com/tunedmystic/rio/ui"
)

func TestMetricCard_RendersValueAndSparkline(t *testing.T) {
	html := render(metricCard("Revenue", "$48.2k", 12.5, []int{3, 5, 4, 8, 7, 11}))
	if !strings.Contains(html, "$48.2k") {
		t.Error("metricCard missing value")
	}
	if !strings.Contains(html, "<svg") || !strings.Contains(html, "<polyline") {
		t.Error("metricCard missing SVG sparkline")
	}
	if !strings.Contains(html, "12.5") {
		t.Error("metricCard missing delta percentage")
	}
}

func TestMetricCard_NegativeDeltaUsesDownArrow(t *testing.T) {
	html := render(metricCard("Churn", "1.2%", -3.0, []int{9, 7, 6, 4}))
	if !strings.Contains(html, "▼") {
		t.Error("negative delta should render a down arrow")
	}
}

func TestDataTable_RendersBadgesAndPagination(t *testing.T) {
	rows := []tableRow{
		{Cells: []string{"INV-001", "Acme"}, Status: "Paid", Variant: ui.BadgeSuccess},
		{Cells: []string{"INV-002", "Globex"}, Status: "Overdue", Variant: ui.BadgeDanger},
	}
	html := render(dataTable([]string{"Invoice", "Customer", "Status", ""}, rows, "1–10 of 240"))
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
}

func TestBarChart_RendersRects(t *testing.T) {
	html := render(barChart([]int{4, 8, 6, 10, 3}))
	if !strings.Contains(html, "<svg") || !strings.Contains(html, "<rect") {
		t.Error("barChart missing SVG bars")
	}
}

func TestUsageMeter_RendersValueText(t *testing.T) {
	html := render(usageMeter("Storage", 18, 50))
	if !strings.Contains(html, "18") || !strings.Contains(html, "50") {
		t.Error("usageMeter missing used/limit text")
	}
}

func TestEmptyState_RendersTitleAndCTA(t *testing.T) {
	html := render(emptyState("layers", "No projects yet", "Create your first project to get started.", nil))
	if !strings.Contains(html, "No projects yet") {
		t.Error("emptyState missing title")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestMetricCard|TestDataTable|TestBarChart|TestUsageMeter|TestEmptyState' -v`
Expected: FAIL — `undefined: metricCard`, etc.

- [ ] **Step 3: Implement `views/kit_data.go`**

```go
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
			dom.Class("flex h-8 w-8 cursor-pointer list-none items-center justify-center rounded-[var(--radius-base)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-raised)] [&::-webkit-details-marker]:hidden"),
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
		dom.Class("rounded-[calc(var(--radius-base)-2px)] px-3 py-1.5 text-[var(--color-text)] hover:bg-[var(--color-surface-raised)]"),
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
```

- [ ] **Step 4: Add the `more` icon case**

In `views/components.go`, in the `icon` switch, add the three-dots case:

```go
	case "more":
		body = `<circle cx="12" cy="5" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="12" cy="19" r="1.5"/>`
```

(If `case "menu"` from Task 4 added `<line>`-based geometry and a `default` `<circle>` exists, this dotted icon reuses the standard `head` template; the filled dots render fine with `stroke`/`fill` defaults.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestMetricCard|TestDataTable|TestBarChart|TestUsageMeter|TestEmptyState' -v`
Expected: PASS (all six).

- [ ] **Step 6: Commit**

```bash
git add views/kit_data.go views/components.go views/kit_data_test.go
git commit -m "feat(views): data kit — metric cards, data table, bar chart, usage meter, empty state"
```

---

### Task 7: Marketing & layout kit (`views/kit_marketing.go`)

Hero, alternating feature highlights, feature grid, pricing table (with a highlighted plan), testimonial, logo cloud, CTA band, and a `<details>` FAQ accordion.

**Files:**
- Create: `views/kit_marketing.go`
- Test: `views/kit_marketing_test.go`

**Interfaces:**
- Consumes: `icon`, `eyebrow` (existing in `pages.go`), `ui.ButtonLink`/variants, `dom.*` (incl. `Details/Summary`, `Ul/Li`).
- Produces:
  - `type featureItem struct { Icon, Title, Blurb string }`
  - `type plan struct { Name, Price, Period string; Features []string; Highlighted bool; CTA dom.Node }`
  - `type faqItem struct { Q, A string }`
  - `func hero(eyebrowText, headline, sub string, primaryCTA, secondaryCTA, visual dom.Node) dom.Node`
  - `func featureHighlight(eyebrowText, title, body string, reverse bool) dom.Node`
  - `func featureGrid(items []featureItem) dom.Node`
  - `func pricingTable(plans []plan) dom.Node`
  - `func testimonial(quote, author, role string) dom.Node`
  - `func logoCloud(labels []string) dom.Node`
  - `func ctaBand(title, body string, cta dom.Node) dom.Node`
  - `func faq(items []faqItem) dom.Node`
  - `func svgPanel() dom.Node` (placeholder visual, no asset)

- [ ] **Step 1: Write the failing tests**

Create `views/kit_marketing_test.go`:

```go
package views

import (
	"strings"
	"testing"

	"github.com/tunedmystic/rio/dom"
)

func TestHero_RendersHeadlineAndCTAs(t *testing.T) {
	html := render(hero("New", "Ship faster", "A starter that scales.",
		dom.Text("Get started"), dom.Text("Learn more"), svgPanel()))
	for _, want := range []string{"Ship faster", "A starter that scales.", "Get started", "Learn more"} {
		if !strings.Contains(html, want) {
			t.Errorf("hero missing %q", want)
		}
	}
}

func TestFeatureGrid_RendersAllItems(t *testing.T) {
	items := []featureItem{
		{Icon: "layers", Title: "Composable", Blurb: "Build from parts."},
		{Icon: "message", Title: "Realtime", Blurb: "Always fresh."},
	}
	html := render(featureGrid(items))
	for _, want := range []string{"Composable", "Realtime", "Build from parts.", "Always fresh."} {
		if !strings.Contains(html, want) {
			t.Errorf("featureGrid missing %q", want)
		}
	}
}

func TestPricingTable_RendersPlansAndHighlight(t *testing.T) {
	plans := []plan{
		{Name: "Starter", Price: "$0", Period: "/mo", Features: []string{"1 project"}, CTA: dom.Text("Choose")},
		{Name: "Pro", Price: "$29", Period: "/mo", Features: []string{"Unlimited"}, Highlighted: true, CTA: dom.Text("Choose")},
	}
	html := render(pricingTable(plans))
	for _, want := range []string{"Starter", "Pro", "$0", "$29", "1 project", "Unlimited"} {
		if !strings.Contains(html, want) {
			t.Errorf("pricingTable missing %q", want)
		}
	}
	if !strings.Contains(html, "Popular") {
		t.Error("pricingTable highlighted plan missing 'Popular' marker")
	}
}

func TestFaq_RendersDetails(t *testing.T) {
	html := render(faq([]faqItem{{Q: "Is it free?", A: "Yes, to start."}}))
	if !strings.Contains(html, "<details") || !strings.Contains(html, "<summary") {
		t.Error("faq should render <details>/<summary>")
	}
	if !strings.Contains(html, "Is it free?") || !strings.Contains(html, "Yes, to start.") {
		t.Error("faq missing question/answer text")
	}
}

func TestLogoCloud_RendersLabels(t *testing.T) {
	html := render(logoCloud([]string{"Acme", "Globex"}))
	if !strings.Contains(html, "Acme") || !strings.Contains(html, "Globex") {
		t.Error("logoCloud missing labels")
	}
}

func TestFeatureHighlight_RendersTitle(t *testing.T) {
	html := render(featureHighlight("Speed", "Instant feedback", "See changes live.", true))
	if !strings.Contains(html, "Instant feedback") || !strings.Contains(html, "See changes live.") {
		t.Error("featureHighlight missing content")
	}
}

func TestCtaBand_RendersTitleAndCTA(t *testing.T) {
	html := render(ctaBand("Ready?", "Start in minutes.", dom.Text("Sign up")))
	for _, want := range []string{"Ready?", "Start in minutes.", "Sign up"} {
		if !strings.Contains(html, want) {
			t.Errorf("ctaBand missing %q", want)
		}
	}
}

func TestTestimonial_RendersQuoteAndAuthor(t *testing.T) {
	html := render(testimonial("It just works.", "Ada", "CTO, Acme"))
	for _, want := range []string{"It just works.", "Ada", "CTO, Acme"} {
		if !strings.Contains(html, want) {
			t.Errorf("testimonial missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestHero|TestFeatureGrid|TestPricingTable|TestFaq|TestLogoCloud|TestFeatureHighlight|TestCtaBand|TestTestimonial' -v`
Expected: FAIL — `undefined: hero`, etc.

- [ ] **Step 3: Implement `views/kit_marketing.go`**

```go
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
				dom.Class("flex cursor-pointer list-none items-center justify-between py-4 font-medium text-[var(--color-text)] [&::-webkit-details-marker]:hidden"),
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
```

- [ ] **Step 4: Add `check` and `chevron-down` icon cases if absent**

In `views/components.go`, in the `icon` switch, ensure these exist (add any that are missing):

```go
	case "check":
		body = `<path d="M20 6 9 17l-5-5"/>`
	case "chevron-down":
		body = `<path d="m6 9 6 6 6-6"/>`
```

(`chevron-right` and `arrow-right` are already used by existing helpers; reuse them.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestHero|TestFeatureGrid|TestPricingTable|TestFaq|TestLogoCloud|TestFeatureHighlight|TestCtaBand|TestTestimonial' -v`
Expected: PASS (all eight).

- [ ] **Step 6: Commit**

```bash
git add views/kit_marketing.go views/components.go views/kit_marketing_test.go
git commit -m "feat(views): marketing kit — hero, features, pricing, testimonial, FAQ, CTA"
```

---

### Task 8: Feedback & forms kit (`views/kit_feedback.go`)

A form layout (reusing `ui` fields), a no-JS toggle switch, and a tab strip. Alerts are reused directly from `ui.Alert` in the showcase (Task 9), so no new alert helper is needed.

**Files:**
- Create: `views/kit_feedback.go`
- Test: `views/kit_feedback_test.go`

**Interfaces:**
- Consumes: `ui.TextField`, `ui.Textarea`, `ui.Select`, `ui.Option`, `ui.Label`, `dom.*`.
- Produces:
  - `type tabItem struct { Key, Label string }`
  - `func formShowcase() dom.Node`
  - `func toggle(name, label string, on bool) dom.Node`
  - `func tabStrip(items []tabItem, active string) dom.Node`

- [ ] **Step 1: Write the failing tests**

Create `views/kit_feedback_test.go`:

```go
package views

import (
	"strings"
	"testing"
)

func TestFormShowcase_RendersFields(t *testing.T) {
	html := render(formShowcase())
	for _, want := range []string{"<form", "<input", "<textarea", "<select"} {
		if !strings.Contains(html, want) {
			t.Errorf("formShowcase missing %q", want)
		}
	}
}

func TestToggle_RendersCheckbox(t *testing.T) {
	html := render(toggle("notify", "Email notifications", true))
	if !strings.Contains(html, `type="checkbox"`) {
		t.Error("toggle should be a styled checkbox")
	}
	if !strings.Contains(html, "Email notifications") {
		t.Error("toggle missing label")
	}
	if !strings.Contains(html, "checked") {
		t.Error("toggle with on=true should be checked")
	}
}

func TestTabStrip_MarksActive(t *testing.T) {
	html := render(tabStrip([]tabItem{{"overview", "Overview"}, {"activity", "Activity"}}, "activity"))
	for _, want := range []string{"Overview", "Activity"} {
		if !strings.Contains(html, want) {
			t.Errorf("tabStrip missing %q", want)
		}
	}
	if !strings.Contains(html, "border-[var(--color-primary)]") {
		t.Error("tabStrip active tab should carry the primary underline")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestFormShowcase|TestToggle|TestTabStrip' -v`
Expected: FAIL — `undefined: formShowcase`, etc.

- [ ] **Step 3: Implement `views/kit_feedback.go`**

```go
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
		ui.Select("plan", "Plan", []ui.Option{
			{Value: "starter", Label: "Starter"},
			{Value: "pro", Label: "Pro"},
		}, "pro", ""),
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
		cls := "px-4 py-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
		if it.Key == active {
			cls = "px-4 py-2 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-primary)] border-b-2 border-[var(--color-primary)] -mb-px"
		}
		tabs = append(tabs, dom.A(dom.Class(cls), dom.Href("#"), dom.Text(it.Label)))
	}
	return dom.Div(tabs...)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestFormShowcase|TestToggle|TestTabStrip' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add views/kit_feedback.go views/kit_feedback_test.go
git commit -m "feat(views): feedback kit — form layout, toggle switch, tab strip"
```

---

### Task 9: `/kit` showcase page, handler, route, and footer link

Assemble every component under section headings with static fixtures; expose it at `/kit` and link it from the footer.

**Files:**
- Create: `views/kit.go`
- Create: `handlers_kit.go`
- Modify: `main.go` (route registration, after `s.Handle("/about", ...)`)
- Modify: `config/config.go` (`New`, the `FooterLinks` literal)
- Test: `views/kit_test.go`, `handlers_kit_test.go`

**Interfaces:**
- Consumes: every kit helper (Tasks 5–8), `Page`, `ruledHeading`, `shell`, `ui.ButtonLink`, `ui.Alert` + alert variants, `config.PageData`, `config.Meta`.
- Produces:
  - `func Kit(pd config.PageData, meta config.Meta) dom.Node`
  - `func HandleKit() http.Handler`
  - a `{Text: "Kit", Href: "/kit"}` entry in `Config.FooterLinks`

- [ ] **Step 1: Write the failing tests**

Create `views/kit_test.go`:

```go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestKit_RendersAllSections(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Kit", Description: "d"}
	var b bytes.Buffer
	_ = Kit(pd, meta).Render(&b)
	html := b.String()

	for _, want := range []string{
		"<!DOCTYPE html>", // full page chrome
		"Foundations",
		"Data",
		"Marketing",
		"Feedback",
		"<svg",       // a chart rendered
		"<details",   // an accordion / row menu rendered
		"1–10 of",    // data table pagination
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Kit output missing %q", want)
		}
	}
}
```

Create `handlers_kit_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleKit_OK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/kit", nil)
	rec := httptest.NewRecorder()
	HandleKit().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Foundations") {
		t.Error("response missing kit content")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run TestKit_RendersAllSections -v; go test . -run TestHandleKit_OK -v`
Expected: FAIL — `undefined: Kit`, `undefined: HandleKit`.

- [ ] **Step 3: Implement `views/kit.go` with fixtures and assembly**

```go
package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// kitSection wraps a group under a ruled heading with vertical rhythm.
func kitSection(title string, body ...dom.Node) dom.Node {
	inner := make([]dom.Node, 0, len(body)+2)
	inner = append(inner, dom.Class("flex flex-col gap-6"), ruledHeading(title))
	inner = append(inner, body...)
	return dom.Section(dom.Class("py-10"), shell(dom.Div(inner...)))
}

// Kit renders the public component showcase under the active theme.
func Kit(pd config.PageData, meta config.Meta) dom.Node {
	tableRows := []tableRow{
		{Cells: []string{"INV-1001", "Acme Inc."}, Status: "Paid", Variant: ui.BadgeSuccess},
		{Cells: []string{"INV-1002", "Globex"}, Status: "Pending", Variant: ui.BadgeWarning},
		{Cells: []string{"INV-1003", "Initech"}, Status: "Overdue", Variant: ui.BadgeDanger},
		{Cells: []string{"INV-1004", "Umbrella"}, Status: "Draft", Variant: ui.BadgeNeutral},
	}
	features := []featureItem{
		{Icon: "layers", Title: "Composable kit", Blurb: "Build pages from focused, token-driven parts."},
		{Icon: "message", Title: "Server-rendered", Blurb: "No JS framework; fast, semantic HTML."},
		{Icon: "check", Title: "Accessible", Blurb: "WCAG AA contrast and keyboard-reachable controls."},
	}
	plans := []plan{
		{Name: "Starter", Price: "$0", Period: "/mo", Features: []string{"1 project", "Community support"}, CTA: ui.ButtonLink(ui.ButtonSecondary, "#", "Choose Starter")},
		{Name: "Pro", Price: "$29", Period: "/mo", Features: []string{"Unlimited projects", "Priority support", "Analytics"}, Highlighted: true, CTA: ui.ButtonLink(ui.ButtonPrimary, "#", "Choose Pro")},
		{Name: "Team", Price: "$99", Period: "/mo", Features: []string{"Everything in Pro", "SSO", "Audit log"}, CTA: ui.ButtonLink(ui.ButtonSecondary, "#", "Choose Team")},
	}
	faqs := []faqItem{
		{Q: "How do I switch themes?", A: "Set Theme: ThemeDusk in config.New and rebuild."},
		{Q: "Do the charts need JavaScript?", A: "No — they are pure server-rendered inline SVG."},
	}

	return Page(pd, meta,
		pageHeader("Component Kit", "Every component in the design system, under the active theme."),

		kitSection("Foundations",
			colorSwatches(),
			typeScale(),
			buttonSet(),
			statusBadges(),
			avatarGroup([]string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Edsger Dijkstra"}),
		),

		kitSection("Data & dashboard",
			dom.Div(
				dom.Class("grid gap-4 sm:grid-cols-2 lg:grid-cols-3"),
				metricCard("Revenue", "$48.2k", 12.5, []int{12, 14, 13, 18, 22, 20, 26}),
				metricCard("Active users", "3,182", 4.1, []int{30, 28, 33, 31, 35, 40, 44}),
				metricCard("Churn", "1.2%", -0.6, []int{9, 8, 8, 7, 6, 5, 5}),
			),
			dataTable([]string{"Invoice", "Customer", "Status", ""}, tableRows, "1–10 of 240"),
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
			emptyState("layers", "No projects yet", "Create your first project to see it here.", ui.ButtonLink(ui.ButtonPrimary, "#", "New project")),
		),

		kitSection("Marketing & layout",
			hero("Design system",
				"A data-forward starter you can ship today",
				"Two themes, a full component kit, and a landing page — all server-rendered.",
				ui.ButtonLink(ui.ButtonPrimary, "#", "Get started"),
				ghostLink("#", "View on GitHub"),
				svgPanel()),
			logoCloud([]string{"Acme", "Globex", "Initech", "Umbrella", "Hooli"}),
			featureHighlight("Fast", "Instant feedback", "Edit a token and the whole app re-skins.", false),
			featureGrid(features),
			pricingTable(plans),
			testimonial("This template saved us a week of setup.", "Ada Lovelace", "CTO, Acme"),
			faq(faqs),
			ctaBand("Ready to build?", "Clone the template and ship your idea.", ui.ButtonLink(ui.ButtonPrimary, "#", "Start now")),
		),

		kitSection("Feedback & forms",
			dom.Div(
				dom.Class("flex flex-col gap-3"),
				ui.Alert(ui.AlertInfo, dom.Text("Heads up — this is an informational alert.")),
				ui.Alert(ui.AlertSuccess, dom.Text("Saved — your changes were applied.")),
				ui.Alert(ui.AlertWarning, dom.Text("Careful — your trial ends soon.")),
				ui.Alert(ui.AlertError, dom.Text("Error — something went wrong.")),
			),
			tabStrip([]tabItem{{"overview", "Overview"}, {"activity", "Activity"}, {"settings", "Settings"}}, "overview"),
			formShowcase(),
		),
	)
}
```

- [ ] **Step 4: Implement `handlers_kit.go`**

```go
package main

import (
	"net/http"

	"app/views"

	"github.com/tunedmystic/rio"
)

// HandleKit renders the public component showcase / style guide.
func HandleKit() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Component Kit")
		return render(w, http.StatusOK, views.Kit(Conf.PageDataFor(account(r)), meta))
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 5: Register the route in `main.go`**

After `s.Handle("/about", HandleAbout())`, add:

```go
	s.Handle("/kit", HandleKit())
```

- [ ] **Step 6: Add the footer link in `config/config.go`**

In `New`, append to the `FooterLinks` literal a Kit entry:

```go
		FooterLinks: []Link{
			{Text: "Home", Href: "/"},
			{Text: "About", Href: "/about"},
			{Text: "Kit", Href: "/kit"},
			{Text: "Privacy Policy", Href: "/privacy-policy"},
		},
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./views/ -run TestKit_RendersAllSections -v && go test . -run TestHandleKit_OK -v && go build ./...`
Expected: PASS (both), build clean.

- [ ] **Step 8: Commit**

```bash
git add views/kit.go handlers_kit.go main.go config/config.go views/kit_test.go handlers_kit_test.go
git commit -m "feat: add /kit showcase page, handler, route, and footer link"
```

---

### Task 10: Rebuild the `Home` landing page with the kit

Replace the current four-feature-row home with a real SaaS landing: hero → logo cloud → feature highlights → feature grid → metric/stat band → pricing → FAQ → CTA band.

**Files:**
- Modify: `views/pages.go` (`Home`, lines 13–69)
- Test: `views/pages_test.go` (create, or append to `views/views_test.go`)

**Interfaces:**
- Consumes: all marketing/data kit helpers (Tasks 6–7), `ui.ButtonLink`, `ghostLink`, `config.PageData`, `config.Meta`.
- Produces: a rebuilt `func Home(pd config.PageData, meta config.Meta) dom.Node` (same signature as today).

- [ ] **Step 1: Write the failing test**

Create `views/pages_test.go`:

```go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestHome_RendersLandingSections(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Home", Description: "d"}
	var b bytes.Buffer
	_ = Home(pd, meta).Render(&b)
	html := b.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		pd.SiteName, // hero references the product
		"Pricing",   // pricing section heading
		"<details",  // FAQ accordion
		"<svg",      // a chart or hero visual
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Home output missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestHome_RendersLandingSections -v`
Expected: FAIL — current Home has no "Pricing" / `<details>`.

- [ ] **Step 3: Rewrite `Home` in `views/pages.go`**

Replace the `Home` function body (keep the signature) with:

```go
func Home(pd config.PageData, meta config.Meta) dom.Node {
	features := []featureItem{
		{Icon: "layers", Title: "Component kit", Blurb: "A full set of token-driven, server-rendered components."},
		{Icon: "message", Title: "Auth & billing", Blurb: "Email login, Google OAuth, and Stripe — already wired."},
		{Icon: "check", Title: "Production floor", Blurb: "Tests, hardening, and accessibility from day one."},
	}
	plans := []plan{
		{Name: "Starter", Price: "$0", Period: "/mo", Features: []string{"1 project", "Community support"}, CTA: ui.ButtonLink(ui.ButtonSecondary, "/login", "Get started")},
		{Name: "Pro", Price: "$29", Period: "/mo", Features: []string{"Unlimited projects", "Priority support", "Analytics"}, Highlighted: true, CTA: ui.ButtonLink(ui.ButtonPrimary, "/login", "Start Pro")},
		{Name: "Team", Price: "$99", Period: "/mo", Features: []string{"Everything in Pro", "SSO", "Audit log"}, CTA: ui.ButtonLink(ui.ButtonSecondary, "/login", "Contact us")},
	}
	faqs := []faqItem{
		{Q: "Is it really no-JS?", A: "Yes — pages are server-rendered HTML; charts are inline SVG."},
		{Q: "Can I change the theme?", A: "Set Theme: ThemeDusk in config.New and rebuild the CSS."},
		{Q: "What's included?", A: "Auth, Google sign-in, Stripe billing, and this component kit."},
	}

	return Page(pd, meta,
		hero("Project template",
			"Clone it, set ProjectName, ship",
			"A "+pd.SiteName+" starter with auth, billing, and a world-class component kit — server-rendered, no JS framework.",
			ui.ButtonLink(ui.ButtonPrimary, "/messages", "Explore the demo"),
			ghostLink("/kit", "Browse the kit"),
			svgPanel()),

		logoCloud([]string{"Acme", "Globex", "Initech", "Umbrella", "Hooli"}),

		featureHighlight("Dashboard-ready",
			"Data components that look the part",
			"Metric cards, tables, and charts render on the server with tabular numerals and a tuned chart palette.",
			false),

		dom.Section(
			dom.Class("py-12"),
			shell(dom.Div(
				dom.Class("grid gap-4 sm:grid-cols-3"),
				metricCard("Revenue", "$48.2k", 12.5, []int{12, 14, 13, 18, 22, 20, 26}),
				metricCard("Active users", "3,182", 4.1, []int{30, 28, 33, 31, 35, 40, 44}),
				metricCard("Uptime", "99.98%", 0.1, []int{40, 41, 40, 42, 41, 43, 44}),
			)),
		),

		featureGrid(features),

		pricingTable(plans),

		faq(faqs),

		ctaBand("Ready to build?", "Clone the template and ship your idea this weekend.",
			ui.ButtonLink(ui.ButtonPrimary, "/login", "Get started")),
	)
}
```

- [ ] **Step 4: Remove now-unused helpers if the compiler flags them**

Run `go build ./...`. If it reports that helpers previously used only by the old `Home` (e.g. `featureRow`) are now unused, that is acceptable for unexported funcs in Go only if they are referenced somewhere; Go does **not** error on unused functions, so no action is required. If `go vet` or an import becomes unused (e.g. an import only the old Home used), remove that import. Verify:

Run: `go vet ./views/`
Expected: clean (no unused-import errors).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./views/ -run TestHome -v && go build ./...`
Expected: PASS, build clean.

- [ ] **Step 6: Commit**

```bash
git add views/pages.go views/pages_test.go
git commit -m "feat(views): rebuild Home as a SaaS landing using the component kit"
```

---

### Task 11: Regenerate CSS, document theming, and final verification

Rebuild Tailwind so the new class literals are emitted, document the `Theme` seam, and run the full suite. Eyeball both presets (the one step that needs human eyes).

**Files:**
- Modify: `README.md`
- Regenerate: `static/css/styles.css`

**Interfaces:**
- Consumes: everything above. No new symbols.

- [ ] **Step 1: Regenerate the CSS**

Run: `make tailwind`
Expected: writes `static/css/styles.css` with no error (Tailwind scans `./**/*.go`, picking up the new kit class literals). If `bin/tailwind` is missing, run `make bin/tailwind` first (the Makefile target downloads it).

- [ ] **Step 2: Run the full test suite and vet**

Run: `go vet ./... && go test ./...`
Expected: all packages PASS, vet clean.

- [ ] **Step 3: Add a "Theming" section to `README.md`**

Add this section (place it near the existing configuration/customization docs):

```markdown
## Theming

The template ships with two compile-time theme presets, selected by one value in
`config.New`:

- **`ThemeSlateIndigo`** (default) — a light, indigo-on-slate look.
- **`ThemeDusk`** — a dark, cyan-on-navy look.

To switch, set the `Theme` field in `config.New`:

```go
Theme: ThemeDusk, // in config/config.go, inside New(...)
```

The selected preset builds `Config.Tokens` (the `:root` CSS variables every
component reads) plus a small set of extended variables (`--color-ring`,
`--color-on-danger`, `--color-surface-raised`, `--chart-1..--chart-4`) emitted by
the layout. After changing the theme, rebuild the CSS with `make tailwind`.

Every component in the system is shown under the active theme at **`/kit`** — use
it as the live component reference. Components are token-driven only, so both
presets render correctly without per-theme code.
```

- [ ] **Step 4: Manually eyeball both presets**

Run the app and visit both pages, then flip the theme and repeat:

```bash
make run   # or: go run . (see the Makefile run target)
```

Visit `http://localhost:<addr>/` and `http://localhost:<addr>/kit`. Confirm the light preset looks correct (contrast, charts, responsive nav collapses below `sm`). Then set `Theme: ThemeDusk` in `config/config.go`, re-run `make tailwind` is **not** needed (class literals unchanged), restart, and re-check both pages under the dark preset. Restore `Theme: ThemeSlateIndigo` as the committed default.

- [ ] **Step 5: Commit**

```bash
git add README.md static/css/styles.css config/config.go
git commit -m "docs: document Theme presets and /kit; regenerate CSS for the design system"
```

---

## Self-Review Notes

- **Spec coverage:** theming architecture (Tasks 1–3), de-hardcoded colors + responsive nav (Task 4), foundations (5), data set (6), marketing (7), feedback/forms (8), `/kit` page + handler + route + footer link (9), rebuilt Home (10), CSS regen + README + manual eyeball (11). Out-of-scope items (runtime toggle, per-theme code, live data, new deps) are not implemented, per the spec.
- **Per-component render tests** are present for every helper; **config tests** assert both presets populate tokens + extended vars and that `Theme` selection swaps them; **page tests** assert `Kit` and `Home` render with representative markers; the **responsive nav** has its own test.
- **Type consistency:** `tableRow`, `featureItem`, `plan`, `faqItem`, `tabItem` are each defined once and consumed with matching field names/signatures across tasks; `sparkline`/`barChart`/`metricCard`/`dataTable` signatures match their call sites in `Kit` and `Home`.
- **API correctness:** uses real vendored signatures — `ui.Button(variant,label,...)`, `ui.Badge(variant,label)`, `ui.Alert(variant,...)`, `ui.ButtonLink(variant,href,label,...)`, `ui.TextField/Textarea/Select/Label`, and typed `dom.Table/Details/Summary`; SVG via `dom.Raw` (no typed `Svg`). `ui.Badge` has no Info variant, so Info is a token-driven `pill`.
```
