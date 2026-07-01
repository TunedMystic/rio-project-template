# Admin / Back-Office Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an auth-gated admin back-office (allowlisted admins) to look up users, inspect subscriptions, and comp/adjust accounts — a searchable/paginated user list plus a user-detail page with safe actions.

**Architecture:** An `ADMIN_EMAILS` allowlist in config gates a new `auth.RequireAdmin` middleware (non-admins get 404). New `database` query methods back a set of `main`-package handlers that render new `views` composed from existing helpers (`card`, `ui.Badge`, `pagination`, `breadcrumbs`, `csrfInput`). No overview/metrics page — the user list is the landing page.

**Tech Stack:** Go 1.26, `net/http` (stdlib `ServeMux` `{id}` wildcards + method patterns), `log/slog`, `github.com/tunedmystic/rio`, existing `app/{auth,database,config,views}` packages.

## Global Constraints

- **Zero new Go module dependencies.**
- Optional/env-gated: empty `ADMIN_EMAILS` → no admins → guard denies everyone (panel disabled).
- Non-admins (logged-in but not allowlisted) receive **HTTP 404** (surface not advertised); logged-out users are redirected to `/login`.
- All admin mutations are POST, require CSRF (reuse `requireCSRF`), use PRG (redirect-after-post), carry a flash via `?flash=<url-escaped>`, and are logged (actor email + target) via `rio.LogInfo`.
- Follow existing idioms: `config.New` env helpers; `database.Store` methods take `context`; `auth` middleware is `func(http.Handler) http.Handler`; handlers use `rio.MakeHandler`; views use `dom.*`; tests are table-driven using existing harnesses.
- Run tests with `go test ./...`.

---

### Task 1: config — `ADMIN_EMAILS` allowlist

**Files:**
- Modify: `config/config.go` (add `AdminEmails []string` field; parse in `New`; add `csvEnv` helper)
- Test: `config/admin_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `Config.AdminEmails []string`; `csvEnv(key string) []string`.

- [ ] **Step 1: Write the failing test**

Create `config/admin_test.go`:

```go
package config

import (
	"reflect"
	"testing"
)

func TestCsvEnv(t *testing.T) {
	t.Run("unset is empty", func(t *testing.T) {
		if got := csvEnv("RIO_TEST_CSV"); len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
	t.Run("splits, trims, lowercases, drops empties", func(t *testing.T) {
		t.Setenv("RIO_TEST_CSV", " Admin@Example.com , ,bob@x.io ")
		got := csvEnv("RIO_TEST_CSV")
		want := []string{"admin@example.com", "bob@x.io"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestNew_ParsesAdminEmails(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "root@example.com")
	c := New("debug", "hash")
	if len(c.AdminEmails) != 1 || c.AdminEmails[0] != "root@example.com" {
		t.Errorf("AdminEmails = %v, want [root@example.com]", c.AdminEmails)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run 'TestCsvEnv|TestNew_ParsesAdminEmails' -v`
Expected: FAIL — `undefined: csvEnv` and unknown field `AdminEmails`.

- [ ] **Step 3: Add the field, parsing, and helper**

In `config/config.go`, add to the `Config` struct (after the `ErrorWebhookURL` field in the Operational group):

```go
	AdminEmails []string
```

In `New`, immediately before `return c`, add:

```go
	c.AdminEmails = csvEnv("ADMIN_EMAILS")
```

Add this helper near the other env helpers (e.g. after `durationFromEnv`):

```go
// csvEnv reads key as a comma-separated list, trimming and lowercasing each
// item and dropping empties. Returns an empty slice when unset.
func csvEnv(key string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.ToLower(strings.TrimSpace(p)); v != "" {
			out = append(out, v)
		}
	}
	return out
}
```

(`os` and `strings` are already imported.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -run 'TestCsvEnv|TestNew_ParsesAdminEmails' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/admin_test.go
git commit -m "feat: parse ADMIN_EMAILS allowlist in config"
```

---

### Task 2: auth — `IsAdmin` + `RequireAdmin` guard

**Files:**
- Create: `auth/admin.go`
- Test: `auth/admin_test.go`

**Interfaces:**
- Consumes: `UserFrom(ctx)` and the unexported `userKey` (same package).
- Produces: `func IsAdmin(email string, admins []string) bool`; `func RequireAdmin(admins []string) func(http.Handler) http.Handler`.

- [ ] **Step 1: Write the failing tests**

Create `auth/admin_test.go`:

```go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/database"
)

func TestIsAdmin(t *testing.T) {
	admins := []string{"root@example.com", "ops@example.com"}
	cases := []struct {
		email string
		want  bool
	}{
		{"root@example.com", true},
		{"  ROOT@Example.com ", true}, // normalized
		{"nope@example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsAdmin(c.email, admins); got != c.want {
			t.Errorf("IsAdmin(%q) = %v, want %v", c.email, got, c.want)
		}
	}
	if IsAdmin("root@example.com", nil) {
		t.Error("empty allowlist must deny")
	}
}

// withUser returns a request whose context carries u, as LoadUser would.
func withUser(u database.User) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	return r.WithContext(context.WithValue(r.Context(), userKey, u))
}

func TestRequireAdmin(t *testing.T) {
	admins := []string{"root@example.com"}
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Run("admin passes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		RequireAdmin(admins)(probe).ServeHTTP(rec, withUser(database.User{Email: "root@example.com"}))
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("non-admin gets 404", func(t *testing.T) {
		rec := httptest.NewRecorder()
		RequireAdmin(admins)(probe).ServeHTTP(rec, withUser(database.User{Email: "user@example.com"}))
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("logged-out redirects to login", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/admin/users", nil) // no user in context
		RequireAdmin(admins)(probe).ServeHTTP(rec, r)
		if rec.Code != http.StatusSeeOther {
			t.Errorf("status = %d, want 303", rec.Code)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./auth/ -run 'TestIsAdmin|TestRequireAdmin' -v`
Expected: FAIL — `undefined: IsAdmin` / `RequireAdmin`.

- [ ] **Step 3: Write the implementation**

Create `auth/admin.go`:

```go
package auth

import (
	"net/http"
	"net/url"
	"strings"
)

// IsAdmin reports whether email is in the admins allowlist (case- and
// whitespace-insensitive). An empty allowlist always denies.
func IsAdmin(email string, admins []string) bool {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return false
	}
	for _, a := range admins {
		if strings.ToLower(strings.TrimSpace(a)) == e {
			return true
		}
	}
	return false
}

// RequireAdmin gates a handler to allowlisted admins. Logged-out users are
// redirected to /login; authenticated non-admins get a 404 so the admin surface
// is not advertised. Intended to wrap a handler already behind RequireUser, but
// is safe on its own.
func RequireAdmin(admins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFrom(r.Context())
			if !ok {
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
				return
			}
			if !IsAdmin(u.Email, admins) {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./auth/ -run 'TestIsAdmin|TestRequireAdmin' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add auth/admin.go auth/admin_test.go
git commit -m "feat: RequireAdmin guard + IsAdmin allowlist check"
```

---

### Task 3: database — user listing/search + revoke entitlement

**Files:**
- Create: `database/admin.go`
- Test: `database/admin_test.go`

**Interfaces:**
- Consumes: existing `userColumns`, `scanUser(rowScanner)`, `User`, `GrantEntitlement`, `HasEntitlement`.
- Produces:
  - `func (s *Store) ListUsers(ctx context.Context, query string, limit, offset int) ([]User, error)`
  - `func (s *Store) CountUsers(ctx context.Context, query string) (int, error)`
  - `func (s *Store) RevokeEntitlement(ctx context.Context, userID int64, productKey string) error`

- [ ] **Step 1: Write the failing tests**

Create `database/admin_test.go`:

```go
package database

import (
	"context"
	"path/filepath"
	"testing"
)

func newAdminTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return NewStore(db)
}

func TestListAndCountUsers(t *testing.T) {
	s := newAdminTestStore(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, "alice@example.com", "Alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "bob@other.com", "Bob"); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListUsers(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d users, want 2", len(all))
	}
	// Newest first: bob created after alice.
	if all[0].Email != "bob@other.com" {
		t.Errorf("all[0] = %q, want bob@other.com", all[0].Email)
	}

	filtered, err := s.ListUsers(ctx, "example", 10, 0)
	if err != nil {
		t.Fatalf("ListUsers(query): %v", err)
	}
	if len(filtered) != 1 || filtered[0].Email != "alice@example.com" {
		t.Errorf("search 'example' = %v, want [alice@example.com]", filtered)
	}

	n, err := s.CountUsers(ctx, "example")
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if n != 1 {
		t.Errorf("CountUsers('example') = %d, want 1", n)
	}

	// Paging: limit 1 offset 1 returns the second-newest (alice).
	page2, err := s.ListUsers(ctx, "", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].Email != "alice@example.com" {
		t.Errorf("page2 = %v, want [alice@example.com]", page2)
	}
}

func TestRevokeEntitlement(t *testing.T) {
	s := newAdminTestStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, "c@example.com", "C")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.GrantEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Fatalf("RevokeEntitlement: %v", err)
	}
	has, err := s.HasEntitlement(ctx, u.ID, "ebook")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("entitlement should be revoked")
	}
	// Revoking an absent entitlement is a no-op, not an error.
	if err := s.RevokeEntitlement(ctx, u.ID, "ebook"); err != nil {
		t.Errorf("revoking absent entitlement should be no-op, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./database/ -run 'TestListAndCountUsers|TestRevokeEntitlement' -v`
Expected: FAIL — `undefined: ListUsers` etc.

- [ ] **Step 3: Write the implementation**

Create `database/admin.go`:

```go
package database

import "context"

// ListUsers returns users whose email contains query (case-insensitive; empty
// query matches all), newest first, paginated by limit/offset.
func (s *Store) ListUsers(ctx context.Context, query string, limit, offset int) ([]User, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE email LIKE '%'||?||'%' "+
			"ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := s.scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountUsers returns the number of users whose email contains query (empty = all).
func (s *Store) CountUsers(ctx context.Context, query string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE email LIKE '%'||?||'%'", query).Scan(&n)
	return n, err
}

// RevokeEntitlement removes a product entitlement from a user. Removing an
// entitlement the user does not have is a no-op (no error).
func (s *Store) RevokeEntitlement(ctx context.Context, userID int64, productKey string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM entitlements WHERE user_id = ? AND product_key = ?", userID, productKey)
	return err
}
```

Note: `s.scanUser` accepts a `rowScanner` (anything with `Scan(...)`); `*sql.Rows` satisfies it, so it works per-row here.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./database/ -run 'TestListAndCountUsers|TestRevokeEntitlement' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/admin.go database/admin_test.go
git commit -m "feat: ListUsers/CountUsers/RevokeEntitlement store methods"
```

---

### Task 4: views — admin list & detail pages

**Files:**
- Create: `views/admin.go`
- Test: `views/admin_test.go`

**Interfaces:**
- Consumes: `config.PageData`, `config.Meta`, `config.Link`, `config.Product`, `database.User`, `database.Session`; existing view helpers `Page`, `pageHeader`, `shell`, `withClass`, `card`, `ruledHeading`, `breadcrumbs`, `pagination`, `emptyState`, `csrfInput`, `deviceLabel`, `submitButton`; `ui.Badge`, `ui.BadgeVariant`, `ui.Select`, `ui.Option`, `ui.Alert`, `ui.AlertSuccess`. Submit buttons use `submitButton()` (NOT `ui.Button(..., dom.Type("submit"))`, which renders `type="button"` first and never submits).
- Produces:
  - `func subStatusBadge(status string) dom.Node`
  - `func AdminUsers(pd config.PageData, meta config.Meta, query string, users []database.User, page, numPages int) dom.Node`
  - `type AdminUserView struct { User database.User; Entitlements []string; Sessions []database.Session; Products []config.Product; CSRF, Flash string }`
  - `func AdminUserDetail(pd config.PageData, meta config.Meta, v AdminUserView) dom.Node`

- [ ] **Step 1: Write the failing tests**

Create `views/admin_test.go`:

```go
package views

import (
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func TestAdminUsers_RendersRowsAndSearch(t *testing.T) {
	users := []database.User{
		{ID: 7, Email: "alice@example.com", Name: "Alice", SubscriptionStatus: "active"},
		{ID: 8, Email: "bob@example.com", Name: "Bob"},
	}
	html := render(AdminUsers(testPageData(), config.Meta{Title: "Users"}, "ali", users, 1, 3))
	if !strings.Contains(html, `href="/admin/users/7"`) {
		t.Error("missing link to user detail")
	}
	if !strings.Contains(html, "alice@example.com") || !strings.Contains(html, "bob@example.com") {
		t.Error("missing user rows")
	}
	if !strings.Contains(html, `name="q"`) || !strings.Contains(html, `value="ali"`) {
		t.Error("missing search box preserving query")
	}
	if !strings.Contains(html, `aria-label="Pagination"`) {
		t.Error("missing pagination")
	}
}

func TestAdminUserDetail_RendersActions(t *testing.T) {
	v := AdminUserView{
		User:         database.User{ID: 7, Email: "alice@example.com", Name: "Alice", SubscriptionStatus: "active"},
		Entitlements: []string{"ebook"},
		Sessions:     []database.Session{{ID: "s1", UserAgent: "ua", IP: "1.2.3.4"}},
		Products:     []config.Product{{Key: "ebook", Name: "E-book"}, {Key: "pro", Name: "Pro"}},
		CSRF:         "tok",
		Flash:        "Granted ebook",
	}
	html := render(AdminUserDetail(testPageData(), config.Meta{Title: "User"}, v))
	for _, want := range []string{
		"alice@example.com",
		`value="tok"`,                                        // csrf input
		`action="/admin/users/7/entitlements/grant"`,         // grant form
		`action="/admin/users/7/entitlements/revoke"`,        // revoke form
		`action="/admin/users/7/sessions/revoke"`,            // revoke sessions
		"Granted ebook",                                       // flash
	} {
		if !strings.Contains(html, want) {
			t.Errorf("detail page missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestAdminUsers|TestAdminUserDetail' -v`
Expected: FAIL — `undefined: AdminUsers` / `AdminUserView`.

- [ ] **Step 3: Write the implementation**

Create `views/admin.go`:

```go
package views

import (
	"fmt"
	"net/url"

	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// adminShell wraps admin page content with the header and a breadcrumb trail.
func adminShell(pd config.PageData, meta config.Meta, section string, body ...dom.Node) dom.Node {
	trail := []config.Link{
		{Text: "Home", Href: "/"},
		{Text: "Admin", Href: "/admin/users"},
		{Text: section, Href: "#"},
	}
	content := append([]dom.Node{breadcrumbs(trail)}, body...)
	return Page(pd, meta,
		pageHeader("Admin", "Operator tools — users and subscriptions."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("space-y-6", content)...))),
	)
}

// subStatusBadge maps a subscription status to a colored badge.
func subStatusBadge(status string) dom.Node {
	variant := ui.BadgeNeutral
	label := status
	switch status {
	case "active", "trialing":
		variant = ui.BadgeSuccess
	case "past_due":
		variant = ui.BadgeWarning
	case "canceled":
		variant = ui.BadgeDanger
	}
	if label == "" {
		label = "none"
	}
	return ui.Badge(variant, label)
}

// AdminUsers renders the searchable, paginated user list (the admin landing page).
func AdminUsers(pd config.PageData, meta config.Meta, query string, users []database.User, page, numPages int) dom.Node {
	search := dom.Form(
		dom.Method("get"), dom.Action("/admin/users"),
		dom.Class("flex gap-2"),
		dom.Input(
			dom.Type("search"), dom.Name("q"), dom.Value(query),
			dom.Placeholder("Search by email"),
			dom.Class("w-full max-w-sm rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-[length:var(--font-size-sm)] text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]"),
		),
		submitButton("Search"),
	)

	head := dom.Tr(
		dom.Class("border-b border-[var(--color-border)]"),
		adminTh("Email"), adminTh("Name"), adminTh("Joined"), adminTh("Subscription"),
	)
	rows := make([]dom.Node, 0, len(users)+1)
	rows = append(rows, head)
	for _, u := range users {
		rows = append(rows, dom.Tr(
			dom.Class("border-b border-[var(--color-border)] last:border-0"),
			dom.Td(dom.Class("px-4 py-3 text-[length:var(--font-size-sm)]"),
				dom.A(
					dom.Class("font-medium text-[var(--color-primary)] hover:underline"),
					dom.Href(fmt.Sprintf("/admin/users/%d", u.ID)),
					dom.Text(u.Email),
				),
			),
			adminTd(u.Name),
			adminTd(u.CreatedAt.Format("2006-01-02")),
			dom.Td(dom.Class("px-4 py-3"), subStatusBadge(u.SubscriptionStatus)),
		))
	}

	table := dom.Div(
		dom.Class("overflow-hidden rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)]"),
		dom.Div(dom.Class("overflow-x-auto"),
			dom.Table(dom.Class("w-full border-collapse"),
				dom.Thead(head0(head)), dom.Tbody(rows[1:]...))),
	)

	base := "/admin/users"
	if query != "" {
		base = "/admin/users?q=" + url.QueryEscape(query) + "&"
	}

	body := []dom.Node{search, table}
	if numPages > 1 {
		body = append(body, pagination(page, numPages, base))
	}
	if len(users) == 0 {
		body = []dom.Node{search, emptyState("layers", "No users", "No users match this search.", nil)}
	}
	return adminShell(pd, meta, "Users", body...)
}

// head0 wraps the header row for Thead (kept separate so Tbody excludes it).
func head0(tr dom.Node) dom.Node { return tr }

func adminTh(label string) dom.Node {
	return dom.Th(
		dom.Class("px-4 py-3 text-left text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text-muted)]"),
		dom.Text(label),
	)
}

func adminTd(text string) dom.Node {
	return dom.Td(
		dom.Class("px-4 py-3 text-[length:var(--font-size-sm)] text-[var(--color-text)]"),
		dom.Text(text),
	)
}

// AdminUserView is the data the user-detail page needs.
type AdminUserView struct {
	User         database.User
	Entitlements []string
	Sessions     []database.Session
	Products     []config.Product
	CSRF         string
	Flash        string
}

// AdminUserDetail renders one user's profile, entitlements, and sessions with
// safe admin actions.
func AdminUserDetail(pd config.PageData, meta config.Meta, v AdminUserView) dom.Node {
	u := v.User
	body := []dom.Node{}
	if v.Flash != "" {
		body = append(body, ui.Alert(ui.AlertSuccess, dom.Text(v.Flash)))
	}

	// Profile key-values.
	google := "no"
	if u.GoogleID != "" {
		google = "yes"
	}
	periodEnd := "—"
	if !u.CurrentPeriodEnd.IsZero() {
		periodEnd = u.CurrentPeriodEnd.Format("2006-01-02")
	}
	profile := card(
		ruledHeading("Profile"),
		dom.Div(dom.Class("mt-4"),
			kv("ID", fmt.Sprintf("%d", u.ID)),
			kv("Email", u.Email),
			kv("Name", u.Name),
			kv("Joined", u.CreatedAt.Format("2006-01-02 15:04")),
			kv("Google linked", google),
			kv("Stripe customer", orDash(u.StripeCustomerID)),
			kv("Subscription", orDash(u.SubscriptionStatus)),
			kv("Current period end", periodEnd),
		),
	)

	// Entitlements: current list with per-item revoke, plus a grant form.
	entItems := make([]dom.Node, 0, len(v.Entitlements))
	for _, key := range v.Entitlements {
		entItems = append(entItems, dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-2"),
			dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text)]"), dom.Text(key)),
			adminActionForm(fmt.Sprintf("/admin/users/%d/entitlements/revoke", u.ID), v.CSRF,
				dom.Input(dom.Type("hidden"), dom.Name("product_key"), dom.Value(key)),
				submitButton("Revoke"),
			),
		))
	}
	if len(entItems) == 0 {
		entItems = append(entItems, dom.P(dom.Class("py-2 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("No entitlements.")))
	}
	opts := make([]ui.Option, 0, len(v.Products))
	for _, p := range v.Products {
		opts = append(opts, ui.Option{Value: p.Key, Label: p.Name})
	}
	grant := adminActionForm(fmt.Sprintf("/admin/users/%d/entitlements/grant", u.ID), v.CSRF,
		dom.Div(dom.Class("flex items-end gap-2"),
			ui.Select("product_key", "Grant product", opts, "", ""),
			submitButton("Grant"),
		),
	)
	entitlements := card(ruledHeading("Entitlements"), dom.Div(dom.Class("mt-4 space-y-3"), dom.Div(entItems...), grant))

	// Sessions with a revoke-all action.
	sessItems := make([]dom.Node, 0, len(v.Sessions))
	for _, sess := range v.Sessions {
		sessItems = append(sessItems, dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-2 text-[length:var(--font-size-sm)]"),
			dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(deviceLabel(sess.UserAgent))),
			dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(sess.IP)),
		))
	}
	if len(sessItems) == 0 {
		sessItems = append(sessItems, dom.P(dom.Class("py-2 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("No active sessions.")))
	}
	sessions := card(
		ruledHeading("Sessions"),
		dom.Div(dom.Class("mt-4 space-y-3"),
			dom.Div(sessItems...),
			adminActionForm(fmt.Sprintf("/admin/users/%d/sessions/revoke", u.ID), v.CSRF,
				submitButton("Revoke all sessions"),
			),
		),
	)

	body = append(body, profile, entitlements, sessions)
	return adminShell(pd, meta, u.Email, body...)
}

// kv renders a label/value row.
func kv(label, value string) dom.Node {
	return dom.Div(
		dom.Class("flex justify-between gap-4 border-b border-[var(--color-border)] py-2 text-[length:var(--font-size-sm)] last:border-0"),
		dom.Span(dom.Class("text-[var(--color-text-muted)]"), dom.Text(label)),
		dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(value)),
	)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// adminActionForm builds a POST form with a CSRF field and the given controls.
func adminActionForm(action, csrf string, controls ...dom.Node) dom.Node {
	children := []dom.Node{dom.Method("post"), dom.Action(action), csrfInput(csrf)}
	children = append(children, controls...)
	return dom.Form(children...)
}
```

Note on the table: `head` is built once; `dom.Thead(head0(head))` puts the header row in the thead and `dom.Tbody(rows[1:]...)` renders the body rows (rows[0] is the header, excluded). Keep `head0` as the trivial passthrough shown.

All `dom.*`/`ui.*` helpers used here are confirmed present in `vendor/`:
`dom.Form/Method/Action/Input/Type/Name/Value/Placeholder/Table/Thead/Tbody/Tr/Th/Td/Div/Span/P/A/Href/Class/Text`, `ui.Badge/Select/Option/Alert/AlertSuccess`, and view helpers `Page/pageHeader/shell/withClass/card/ruledHeading/breadcrumbs/pagination/emptyState/csrfInput/deviceLabel/submitButton`. Use `dom.Placeholder(...)` for the search field (there is no `dom.Attr`). Use `submitButton(label)` for every form submit — do NOT use `ui.Button(..., dom.Type("submit"))`: `ui.Button` prepends `dom.Type("button")`, so a trailing `dom.Type("submit")` is ignored (first attribute wins) and the form would never submit.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestAdminUsers|TestAdminUserDetail' -v`
Expected: PASS. If a `dom.*`/`ui.*` helper name is wrong, fix per the note and re-run.

- [ ] **Step 5: Run the full views suite (no regressions)**

Run: `go test ./views/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add views/admin.go views/admin_test.go
git commit -m "feat: admin user list and detail views"
```

---

### Task 5: handlers + route wiring

**Files:**
- Create: `handlers_admin.go`
- Modify: `main.go` (register admin routes)
- Test: `handlers_admin_test.go`

**Interfaces:**
- Consumes: `auth.RequireUser`, `auth.RequireAdmin`, `auth.SessionFrom`, `auth.UserFrom`, `auth.CSRFToken`; `requireCSRF` (in `handlers_account.go`); `account(r)`, `render(w, status, node)`; `Conf` (`AdminEmails`, `AppSecret`, `Products`, `ProductByKey`, `PageDataFor`, `NewMeta`); store methods `CountUsers`, `ListUsers`, `UserByID`, `ListEntitlements`, `ListUserSessions`, `GrantEntitlement`, `RevokeEntitlement`, `DeleteUserSessions`; views `AdminUsers`, `AdminUserDetail`, `AdminUserView`; `rio.MakeHandler`, `rio.LogInfo`.
- Produces: `HandleAdminIndex`, `HandleAdminUsers`, `HandleAdminUserDetail`, `HandleAdminGrantEntitlement`, `HandleAdminRevokeEntitlement`, `HandleAdminRevokeSessions`; `const adminPageSize = 25`.

- [ ] **Step 1: Write the failing tests**

Create `handlers_admin_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"app/auth"
	"app/database"
)

func newHandlerTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/s.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return database.NewStore(db)
}

func TestAdminUsers_AdminSees200_NonAdmin404(t *testing.T) {
	store := newHandlerTestStore(t)
	sess, u := loggedInRequestSession(t, store)

	// Admin allowlist contains the user → 200.
	r, _ := loggedInWith(t, store, u, sess, http.MethodGet, "/admin/users", "")
	rec := httptest.NewRecorder()
	auth.RequireUser(auth.RequireAdmin([]string{u.Email})(HandleAdminUsers(store))).ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Errorf("admin status = %d, want 200", rec.Code)
	}

	// Empty allowlist → non-admin → 404.
	r2, _ := loggedInWith(t, store, u, sess, http.MethodGet, "/admin/users", "")
	rec2 := httptest.NewRecorder()
	auth.RequireUser(auth.RequireAdmin(nil)(HandleAdminUsers(store))).ServeHTTP(rec2, r2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("non-admin status = %d, want 404", rec2.Code)
	}
}

func TestAdminGrantEntitlement_AddsAndRedirects(t *testing.T) {
	store := newHandlerTestStore(t)
	sess, u := loggedInRequestSession(t, store)

	id := strconv.FormatInt(u.ID, 10)
	form := url.Values{}
	form.Set("_csrf", auth.CSRFToken(Conf.AppSecret, sess.ID))
	form.Set("product_key", "ebook")
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/admin/users/"+id+"/entitlements/grant", form.Encode())
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", id) // handler reads r.PathValue("id"); set it explicitly since we call the handler directly, not via the mux

	rec := httptest.NewRecorder()
	HandleAdminGrantEntitlement(store).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	has, err := store.HasEntitlement(r.Context(), u.ID, "ebook")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("entitlement was not granted")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run 'TestAdminUsers_AdminSees200|TestAdminGrantEntitlement' -v`
Expected: FAIL — `undefined: HandleAdminUsers` / `HandleAdminGrantEntitlement`.

- [ ] **Step 3: Write the handlers**

Create `handlers_admin.go`:

```go
package main

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"app/auth"
	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
)

const adminPageSize = 25

// HandleAdminIndex redirects the admin root to the user list (the landing page).
func HandleAdminIndex() http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return nil
	})
}

// HandleAdminUsers renders the searchable, paginated user list.
func HandleAdminUsers(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		page := 1
		if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
			page = p
		}
		total, err := store.CountUsers(r.Context(), q)
		if err != nil {
			return err
		}
		numPages := (total + adminPageSize - 1) / adminPageSize
		if numPages < 1 {
			numPages = 1
		}
		if page > numPages {
			page = numPages
		}
		users, err := store.ListUsers(r.Context(), q, adminPageSize, (page-1)*adminPageSize)
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Admin · Users")
		return render(w, http.StatusOK, views.AdminUsers(Conf.PageDataFor(account(r)), meta, q, users, page, numPages))
	})
}

// HandleAdminUserDetail renders one user's detail page.
func HandleAdminUserDetail(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		u, err := store.UserByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		ents, err := store.ListEntitlements(r.Context(), id)
		if err != nil {
			return err
		}
		sessions, err := store.ListUserSessions(r.Context(), id)
		if err != nil {
			return err
		}
		sess, _ := auth.SessionFrom(r.Context())
		v := views.AdminUserView{
			User:         u,
			Entitlements: ents,
			Sessions:     sessions,
			Products:     Conf.Products,
			CSRF:         auth.CSRFToken(Conf.AppSecret, sess.ID),
			Flash:        r.URL.Query().Get("flash"),
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Admin · User")
		return render(w, http.StatusOK, views.AdminUserDetail(Conf.PageDataFor(account(r)), meta, v))
	})
}

// HandleAdminGrantEntitlement grants a catalog product to a user.
func HandleAdminGrantEntitlement(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		key := r.FormValue("product_key")
		if _, ok := Conf.ProductByKey(key); !ok {
			http.Redirect(w, r, adminUserURL(id, "Unknown product"), http.StatusSeeOther)
			return nil
		}
		if err := store.GrantEntitlement(r.Context(), id, key); err != nil {
			return err
		}
		logAdminAction(r, "grant_entitlement", id, key)
		http.Redirect(w, r, adminUserURL(id, "Granted "+key), http.StatusSeeOther)
		return nil
	})
}

// HandleAdminRevokeEntitlement removes an entitlement from a user.
func HandleAdminRevokeEntitlement(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		key := r.FormValue("product_key")
		if err := store.RevokeEntitlement(r.Context(), id, key); err != nil {
			return err
		}
		logAdminAction(r, "revoke_entitlement", id, key)
		http.Redirect(w, r, adminUserURL(id, "Revoked "+key), http.StatusSeeOther)
		return nil
	})
}

// HandleAdminRevokeSessions signs a user out of all sessions.
func HandleAdminRevokeSessions(store *database.Store) http.Handler {
	return rio.MakeHandler(func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return nil
		}
		if err := store.DeleteUserSessions(r.Context(), id, ""); err != nil {
			return err
		}
		logAdminAction(r, "revoke_sessions", id, "")
		http.Redirect(w, r, adminUserURL(id, "Sessions revoked"), http.StatusSeeOther)
		return nil
	})
}

// adminUserURL builds the detail URL with a flash message.
func adminUserURL(id int64, flash string) string {
	return "/admin/users/" + strconv.FormatInt(id, 10) + "?flash=" + url.QueryEscape(flash)
}

// logAdminAction records an admin mutation (actor + target) to the app logger.
func logAdminAction(r *http.Request, action string, targetID int64, detail string) {
	actor, _ := auth.UserFrom(r.Context())
	rio.LogInfo("admin action",
		slog.String("action", action),
		slog.String("actor", actor.Email),
		slog.Int64("target_user", targetID),
		slog.String("detail", detail),
	)
}
```

- [ ] **Step 4: Register the routes in `main.go`**

In `main.go` `run()`, after the account routes block (before the `if Conf.StripeEnabled()` block), add:

```go
	// Admin (env-gated by ADMIN_EMAILS; non-admins get 404).
	admin := func(h http.Handler) http.Handler {
		return auth.RequireUser(auth.RequireAdmin(Conf.AdminEmails)(h))
	}
	s.Handle("/admin", admin(HandleAdminIndex()))
	s.Handle("/admin/users", admin(HandleAdminUsers(store)))
	s.Handle("GET /admin/users/{id}", admin(HandleAdminUserDetail(store)))
	s.Handle("POST /admin/users/{id}/entitlements/grant", admin(HandleAdminGrantEntitlement(store)))
	s.Handle("POST /admin/users/{id}/entitlements/revoke", admin(HandleAdminRevokeEntitlement(store)))
	s.Handle("POST /admin/users/{id}/sessions/revoke", admin(HandleAdminRevokeSessions(store)))
```

(`auth` is already imported in `main.go`.)

- [ ] **Step 5: Run the tests and the full suite + build**

Run: `go build ./... && go test . -run 'TestAdminUsers_AdminSees200|TestAdminGrantEntitlement' -v && go test ./...`
Expected: PASS across all packages, build clean.

- [ ] **Step 6: Live smoke test**

Run:
```bash
go build -o /tmp/rio-admin . && DB_DIR=/tmp PORT=3014 APP_SECRET=dev ADMIN_EMAILS=nobody@example.com /tmp/rio-admin &
sleep 2
curl -s -o /dev/null -w "admin(anon)=%{http_code}\n" http://localhost:3014/admin/users   # expect 303 -> /login
kill %1; rm -f /tmp/rio-admin /tmp/riostarter.db*
```
Expected: `admin(anon)=303` (unauthenticated → redirect to login), proving the routes are wired and guarded.

- [ ] **Step 7: Commit**

```bash
git add handlers_admin.go main.go handlers_admin_test.go
git commit -m "feat: admin handlers and routes (user list/detail + safe actions)"
```

---

### Task 6: `.env.example` + README docs

**Files:**
- Modify: `.env.example` (add `ADMIN_EMAILS`)
- Modify: `README.md` (env row + short Admin section)
- Test: extend `env_example_test.go` (guard that `ADMIN_EMAILS` is listed)

**Interfaces:**
- Consumes: nothing.
- Produces: docs; a strengthened env guard test.

- [ ] **Step 1: Extend the failing test**

In `env_example_test.go`, add `"ADMIN_EMAILS"` to the `keys` slice asserted by `TestEnvExample_ListsAllKeys` (the test that reads `.env.example`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestEnvExample -v`
Expected: FAIL — `.env.example missing ADMIN_EMAILS`.

- [ ] **Step 3: Add `ADMIN_EMAILS` to `.env.example`**

In `.env.example`, under the `# ----- Operations -----` section, add:

```bash
# Comma-separated admin emails granted access to /admin. Empty disables the panel.
ADMIN_EMAILS=
```

- [ ] **Step 4: Document in the README**

In `README.md`, add a row to the environment table (near the auth/ops vars):

```markdown
| `ADMIN_EMAILS` | Comma-separated admin emails for `/admin` (empty disables) | unset |
```

And add a short section after the Accounts/Billing material:

```markdown
## Admin

An auth-gated back-office at `/admin` (redirects to `/admin/users`). Set
`ADMIN_EMAILS` to a comma-separated allowlist; those users get a searchable,
paginated user list and a per-user detail page where you can grant/revoke an
entitlement (comp an account) and revoke a user's sessions. Non-admins receive a
404 (the surface is not advertised); leaving `ADMIN_EMAILS` empty disables it.
```

- [ ] **Step 5: Run the test and full suite**

Run: `go test . -run TestEnvExample -v && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add .env.example README.md env_example_test.go
git commit -m "docs: document ADMIN_EMAILS and the /admin back-office"
```

---

## Self-Review

**Spec coverage:**
- Access control (`ADMIN_EMAILS` + `RequireAdmin`, 404 for non-admins, empty disables) → Tasks 1, 2; wired in Task 5. ✓
- Store methods (`ListUsers`, `CountUsers`, `RevokeEntitlement`) → Task 3. ✓
- Handlers/routes (`/admin` redirect, `/admin/users`, `/admin/users/{id}`, grant/revoke/revoke-sessions), CSRF, PRG, `?flash=`, logging → Task 5. ✓
- Views (`adminShell`, `adminUsers`, `adminUserDetail`, `subStatusBadge`; no overview/metrics; reuse `pagination`/`ui.Badge`/`breadcrumbs`/`csrfInput`; purpose-built linkable table, not `dataTable`) → Task 4. ✓
- Config/docs (`.env.example`, README) → Task 6. ✓
- Global constraints (zero new deps; env-gated; CSRF on all POSTs; admin-only) enforced across Tasks 2 & 5. ✓

**Placeholder scan:** No TBD/TODO. All `dom.*`/`ui.*`/view helper names were verified against `vendor/` (notably `dom.Placeholder`, and the `ui.Button`-can't-submit gotcha → `submitButton`). No deferred work.

**Type consistency:** `AdminUserView{User, Entitlements, Sessions, Products, CSRF, Flash}`, `AdminUsers(pd, meta, query, users, page, numPages)`, `subStatusBadge(status)`, `RequireAdmin(admins)`, `IsAdmin(email, admins)`, `ListUsers(ctx, query, limit, offset)`, `CountUsers(ctx, query)`, `RevokeEntitlement(ctx, userID, productKey)`, and the six `HandleAdmin*` names are referenced identically across Tasks 2–6. Store method signatures match the real `func(context.Context, …) error` shapes; `store.DeleteUserSessions(ctx, id, "")` matches the existing signature `(ctx, userID, exceptID)`.
