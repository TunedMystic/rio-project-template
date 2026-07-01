# Pages Index (`/pages`) — Design

**Date:** 2026-07-01
**Status:** Approved (pending spec review)

## Goal

Give a freshly-cloned template a single dev-only page that lists every page/route
in the app, grouped and annotated, so a developer can discover and jump to all
the built-in pages from one place.

## Scope

In scope:
- A `GET /pages` route, registered only under `Conf.Debug` (dev builds).
- A grouped, annotated list of the template's pages + utility endpoints.

Out of scope (deferred; do NOT build):
- Any production/public exposure of the index.
- Auto-discovering routes from the mux (the catalog is a hand-maintained list).
- Listing pure form-POST/action endpoints (`/logout`, `/messages` POST, session
  revokes, entitlement grant/revoke, Stripe webhook) — those are not pages.
- Search, auth, or per-user filtering of the list.

## Global constraints

- **Zero new Go module dependencies.**
- The route is **dev-only**: registered only when `Conf.Debug` is true (i.e.
  `make run` / `BuildEnv=debug`); it must not exist in a production build.
- The name is `/pages` (not `/dev/...`).
- List **every** page even when its feature is disabled (e.g. Premium/Guide when
  Stripe is off, Admin when `ADMIN_EMAILS` is unset), each with a short note
  explaining the gating — so the page is a complete map of the template.
- Follow existing idioms: `dom.*` builders, `rio.MakeHandler` handlers, existing
  view helpers (`Page`, `pageHeader`, `shell`, `card`, `ruledHeading`), and
  table-driven tests with existing helpers.

## Component 1: Link catalog (`handlers.go`)

A static catalog defined once, in the `main` package, built from the **`views`
types** (`views.PageGroup` / `views.PageLink`, defined in Component 3) so there
is a single type definition and no `main`↔`views` mapping:

```go
// pageCatalog returns the grouped list of the template's pages + endpoints.
// Hand-maintained — update it when adding a page.
func pageCatalog() []views.PageGroup
```

Contents (exact):

- **Public**
  - Home — `/`
  - About — `/about`
  - Messages — `/messages` — note: "SQLite-backed demo"
  - Component Kit — `/kit`
  - Privacy Policy — `/privacy-policy`
  - Terms of Service — `/terms`
  - Log in — `/login`
- **Account** (note on group items: "requires login")
  - Account (Profile) — `/account`
  - Security — `/account/security`
  - Billing — `/account/billing`
  - Delete account — `/account/delete`
- **Admin**
  - Admin (users) — `/admin` — note: "requires ADMIN_EMAILS; non-admins get 404"
- **Billing**
  - Premium — `/premium` — note: "requires Stripe + active subscription"
  - Guide — `/guide` — note: "requires Stripe + ebook entitlement"
- **Reference**
  - Email previews — `/dev/emails` — note: "dev only"
  - This page — `/pages` — note: "dev only"
- **Utility endpoints**
  - Version (JSON) — `/version`
  - Health — `/healthz`
  - robots.txt — `/robots.txt`

Notes are attached per link (the "Account" group notes go on each account link's
`Note`, e.g. "requires login"). The catalog is hand-maintained; a comment in
`pageCatalog` reminds maintainers to update it when adding pages.

## Component 2: Handler (`handlers.go`)

```go
// HandlePages renders the dev-only index of the template's pages.
func HandlePages() http.Handler
```

Uses `rio.MakeHandler`; renders `views.Pages(Conf.PageDataFor(account(r)), meta,
pageCatalog())` with `meta := Conf.NewMeta(r.URL.RequestURI(), "Pages")`.

## Component 3: View (`views/pages.go`)

Add to the existing `views/pages.go` (where `PrivacyPolicy`, `About`, etc. live):

```go
// PageGroup / PageLink are the view-facing shapes for the pages index.
type PageLink struct{ Label, Href, Note string }
type PageGroup struct {
	Title string
	Links []PageLink
}

// Pages renders the grouped index of the template's pages.
func Pages(pd config.PageData, meta config.Meta, groups []PageGroup) dom.Node
```

- Renders `Page(pd, meta, pageHeader("Pages", "Every page in this template — a
  dev-only index."), <section with the groups>)`.
- Each group is a `card(ruledHeading(group.Title), <list of link rows>)`.
- Each link row is an `<a href=Href>` showing the `Label` (primary link style)
  and, when `Note != ""`, a muted note span/line beside or under it.
- Reuses existing helpers; no new generic primitives.

`views.PageGroup` / `views.PageLink` are the single type definition; the `main`
package's `pageCatalog()` builds `[]views.PageGroup` directly (no duplicate types,
no mapping).

## Wiring (`main.go` `run()`)

Extend the existing dev-only block (the one that registers `/dev/emails`) to also
register `/pages`:

```go
	if Conf.Debug {
		s.Handle("/pages", HandlePages())
		s.Handle("/dev/emails", HandleDevEmails())
		s.Handle("GET /dev/emails/{name}", HandleDevEmailPreview())
	}
```

## Testing

- **handler** (`handlers_test.go` or a small new test file): `HandlePages()` served
  with `GET /pages` returns 200 and the body contains representative links —
  `href="/about"`, `href="/kit"`, `href="/admin"`, `href="/terms"`,
  `href="/dev/emails"`, `href="/pages"`.
- **view** (`views/pages_test.go`): `Pages(testPageData(), meta, sampleGroups)`
  renders each group title and each link `href`, and renders a `Note` when
  present.

## Non-goals / YAGNI

- No route auto-discovery; the catalog is an explicit, hand-maintained list.
- No production exposure; dev-gated only.
- No new view primitives; compose existing `card`/`ruledHeading`/link styles.
