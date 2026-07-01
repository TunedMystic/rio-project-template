# Pages Index (`/pages`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dev-only `/pages` route that renders a grouped, annotated index of every page in the template.

**Architecture:** A `views.Pages` view renders grouped link lists from `views.PageGroup`/`views.PageLink`. The `main` package builds a static catalog (`pageCatalog()`) of those groups and a `HandlePages()` handler, registered only under `Conf.Debug`.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio/dom`, `rio.MakeHandler`, existing `views` helpers. No new dependencies.

## Global Constraints

- **Zero new Go module dependencies.**
- The route is **dev-only**: registered only when `Conf.Debug` is true; must not exist in a production build.
- Route name is `/pages` (not `/dev/...`).
- List **every** page even when its feature is disabled, each with a short gating note.
- Single type definition: `views.PageGroup` / `views.PageLink`; the `main` catalog builds `[]views.PageGroup` directly (no duplicate types, no mapping).
- Follow existing idioms: `dom.*` builders, `rio.MakeHandler`, existing view helpers (`Page`, `pageHeader`, `shell`, `card`, `ruledHeading`). Run tests with `go test ./...`.

---

### Task 1: `Pages` view + types

**Files:**
- Modify: `views/pages.go` (add `PageLink`, `PageGroup`, `Pages`)
- Test: `views/pages_test.go`

**Interfaces:**
- Consumes: `config.PageData`, `config.Meta`; existing `Page`, `pageHeader`, `shell`, `card`, `ruledHeading`.
- Produces:
  - `type PageLink struct{ Label, Href, Note string }`
  - `type PageGroup struct{ Title string; Links []PageLink }`
  - `func Pages(pd config.PageData, meta config.Meta, groups []PageGroup) dom.Node`

- [ ] **Step 1: Write the failing test**

Add to `views/pages_test.go`:

```go
func TestPages_RendersGroupsAndLinks(t *testing.T) {
	groups := []PageGroup{
		{Title: "Public", Links: []PageLink{
			{Label: "About", Href: "/about"},
			{Label: "Component Kit", Href: "/kit"},
		}},
		{Title: "Account", Links: []PageLink{
			{Label: "Account", Href: "/account", Note: "requires login"},
		}},
	}
	html := render(Pages(testPageData(), config.Meta{Title: "Pages"}, groups))
	for _, want := range []string{"Public", "Account", `href="/about"`, `href="/kit"`, `href="/account"`, "requires login"} {
		if !strings.Contains(html, want) {
			t.Errorf("Pages output missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run TestPages_RendersGroupsAndLinks -v`
Expected: FAIL — `undefined: PageGroup` / `Pages`.

- [ ] **Step 3: Write the implementation**

Add to `views/pages.go` (near the other page views, e.g. after `PrivacyPolicy`):

```go
// PageLink is one entry in the pages index.
type PageLink struct{ Label, Href, Note string }

// PageGroup is a titled group of links in the pages index.
type PageGroup struct {
	Title string
	Links []PageLink
}

// Pages renders a grouped index of the template's pages (a dev-only reference).
func Pages(pd config.PageData, meta config.Meta, groups []PageGroup) dom.Node {
	cards := make([]dom.Node, 0, len(groups))
	for _, g := range groups {
		rows := make([]dom.Node, 0, len(g.Links)+1)
		rows = append(rows, ruledHeading(g.Title))
		list := make([]dom.Node, 0, len(g.Links))
		for _, l := range g.Links {
			row := []dom.Node{
				dom.Class("flex flex-wrap items-baseline gap-x-3 gap-y-1 py-2"),
				dom.A(
					dom.Class("font-medium text-[var(--color-primary)] hover:underline"),
					dom.Href(l.Href),
					dom.Text(l.Label),
				),
				dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text(l.Href)),
			}
			if l.Note != "" {
				row = append(row, dom.Span(
					dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
					dom.Text("· "+l.Note),
				))
			}
			list = append(list, dom.Div(row...))
		}
		rows = append(rows, dom.Div(withClass("mt-2 divide-y divide-[var(--color-border)]", list)...))
		cards = append(cards, card(rows...))
	}
	return Page(pd, meta,
		pageHeader("Pages", "Every page in this template — a dev-only index."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("max-w-2xl space-y-6", cards)...))),
	)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./views/ -run TestPages_RendersGroupsAndLinks -v && go test ./views/`
Expected: PASS (new test + full views suite).

- [ ] **Step 5: Commit**

```bash
git add views/pages.go views/pages_test.go
git commit -m "feat: Pages view — grouped index of template pages"
```

---

### Task 2: catalog, handler, and dev-only route

**Files:**
- Modify: `handlers.go` (add `pageCatalog()` + `HandlePages()`)
- Modify: `main.go` (register `/pages` under `Conf.Debug`)
- Modify: `README.md` (one-line note)
- Test: `handlers_pages_test.go` (Create)

**Interfaces:**
- Consumes: `views.Pages`, `views.PageGroup`, `views.PageLink` (Task 1); `account`, `render`, `Conf.PageDataFor`, `Conf.NewMeta`, `rio.MakeHandler`.
- Produces: `func pageCatalog() []views.PageGroup`, `func HandlePages() http.Handler`.

- [ ] **Step 1: Write the failing test**

Create `handlers_pages_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlePages_ListsRoutes(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pages", nil)
	rec := httptest.NewRecorder()
	HandlePages().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="/about"`, `href="/kit"`, `href="/privacy-policy"`, `href="/terms"`,
		`href="/account"`, `href="/admin"`, `href="/dev/emails"`, `href="/pages"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pages index missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./ -run TestHandlePages_ListsRoutes -v`
Expected: FAIL — `undefined: HandlePages`.

- [ ] **Step 3: Add the catalog + handler**

Add to `handlers.go` (after `HandlePrivacyPolicy`, and note `handlers.go` already imports `net/http`, `app/views`, `github.com/tunedmystic/rio`):

```go
// pageCatalog is the hand-maintained list of the template's pages for the
// dev-only /pages index. Update it when adding a page.
func pageCatalog() []views.PageGroup {
	return []views.PageGroup{
		{Title: "Public", Links: []views.PageLink{
			{Label: "Home", Href: "/"},
			{Label: "About", Href: "/about"},
			{Label: "Messages", Href: "/messages", Note: "SQLite-backed demo"},
			{Label: "Component Kit", Href: "/kit"},
			{Label: "Privacy Policy", Href: "/privacy-policy"},
			{Label: "Terms of Service", Href: "/terms"},
			{Label: "Log in", Href: "/login"},
		}},
		{Title: "Account", Links: []views.PageLink{
			{Label: "Account (Profile)", Href: "/account", Note: "requires login"},
			{Label: "Security", Href: "/account/security", Note: "requires login"},
			{Label: "Billing", Href: "/account/billing", Note: "requires login"},
			{Label: "Delete account", Href: "/account/delete", Note: "requires login"},
		}},
		{Title: "Admin", Links: []views.PageLink{
			{Label: "Admin (users)", Href: "/admin", Note: "requires ADMIN_EMAILS; non-admins get 404"},
		}},
		{Title: "Billing", Links: []views.PageLink{
			{Label: "Premium", Href: "/premium", Note: "requires Stripe + active subscription"},
			{Label: "Guide", Href: "/guide", Note: "requires Stripe + ebook entitlement"},
		}},
		{Title: "Reference", Links: []views.PageLink{
			{Label: "Email previews", Href: "/dev/emails", Note: "dev only"},
			{Label: "This page", Href: "/pages", Note: "dev only"},
		}},
		{Title: "Utility endpoints", Links: []views.PageLink{
			{Label: "Version (JSON)", Href: "/version"},
			{Label: "Health", Href: "/healthz"},
			{Label: "robots.txt", Href: "/robots.txt"},
		}},
	}
}

// HandlePages renders the dev-only index of the template's pages.
func HandlePages() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Pages")
		return render(w, http.StatusOK, views.Pages(Conf.PageDataFor(account(r)), meta, pageCatalog()))
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Register the route (dev only)**

In `main.go` `run()`, find the existing dev block that registers `/dev/emails`:

```go
	if Conf.Debug {
		s.Handle("/dev/emails", HandleDevEmails())
		s.Handle("GET /dev/emails/{name}", HandleDevEmailPreview())
	}
```

and add the `/pages` line at the top of that block:

```go
	if Conf.Debug {
		s.Handle("/pages", HandlePages())
		s.Handle("/dev/emails", HandleDevEmails())
		s.Handle("GET /dev/emails/{name}", HandleDevEmailPreview())
	}
```

- [ ] **Step 5: Run tests and build**

Run: `go build ./... && go test ./ -run TestHandlePages_ListsRoutes -v && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 5b: Rebuild the CSS**

The `Pages` view introduces Tailwind utilities that may not yet be in the compiled stylesheet (`divide-y`, `divide-[var(--color-border)]`, `flex-wrap`, `items-baseline`, `gap-x-3`, `gap-y-1`). Regenerate so the page renders correctly:

Run: `make tailwind`
Then `git status --short` should show `static/css/styles.css` modified (include it in the final commit). Tests don't depend on CSS, but the live smoke check and the real page do.

- [ ] **Step 6: Add a README note**

In `README.md`, add a short line (e.g. under the theming/dev section or near the "kit" mention):

```markdown
In dev (`make run`), visit `/pages` for an index of every page in the template,
and `/dev/emails` to preview the email templates. Both are dev-only.
```

- [ ] **Step 7: Live smoke test**

The route registers only under a debug build:
```bash
DB_DIR=/tmp go run -ldflags="-X 'main.BuildEnv=debug'" . >/tmp/pages.log 2>&1 &
sleep 3
curl -s -o /dev/null -w "pages=%{http_code}\n" http://localhost:3000/pages
echo "has kit link: $(curl -s http://localhost:3000/pages | grep -c 'href=\"/kit\"')"
kill %1 2>/dev/null; rm -f /tmp/pages.log
```
Expected: `pages=200` and `has kit link: 1`. (A production build has `Conf.Debug=false`, so `/pages` is correctly absent there.)

- [ ] **Step 8: Commit**

```bash
git add handlers.go handlers_pages_test.go main.go README.md static/css/styles.css
git commit -m "feat: dev-only /pages index route"
```

---

## Self-Review

**Spec coverage:**
- `GET /pages` dev-only route → Task 2 (registered inside `if Conf.Debug`). ✓
- Grouped, annotated catalog (Public/Account/Admin/Billing/Reference/Utility) listing every page incl. disabled ones with notes → Task 2 `pageCatalog()`. ✓
- Single type definition `views.PageGroup`/`views.PageLink`; catalog builds them directly → Tasks 1 & 2. ✓
- View renders groups + links + notes → Task 1 `Pages`. ✓
- Tests (handler lists representative links; view renders groups/links/notes) → Task 2 & Task 1. ✓
- README note → Task 2 Step 6. ✓
- Global constraints (zero deps; dev-gated; `/pages` name; existing helpers) → enforced. ✓

**Placeholder scan:** No TBD/TODO; every step has complete code. The smoke test uses the debug `go run` because the route is dev-gated (a prod build omits it — that's the intended behavior, noted).

**Type consistency:** `views.PageLink{Label,Href,Note}`, `views.PageGroup{Title,Links}`, `Pages(pd, meta, groups)`, `pageCatalog() []views.PageGroup`, `HandlePages()` are referenced identically across Tasks 1–2. The `views` types are the single definition; `main` builds them directly. `card`, `ruledHeading`, `shell`, `pageHeader`, `withClass`, `Page` are existing `views` helpers used by the render code.
