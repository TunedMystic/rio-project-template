# Admin / Back-Office — Design

**Date:** 2026-07-01
**Status:** Approved (pending spec review)

## Goal

Add a minimal, auth-gated admin back-office so an operator can look up users,
inspect a subscription, and comp/adjust an account without opening the SQLite
file — the single biggest gap between this template and running a real SaaS on
it. Scope is deliberately MVP: **read + a few safe actions**, no destructive ops.

## Scope

In scope:
- Access control via an `ADMIN_EMAILS` allowlist + a `RequireAdmin` guard.
- A searchable/paginated user list (the admin landing page) and a user detail page.
- Safe mutations: grant/revoke an entitlement (comp), revoke a user's sessions.

Out of scope (deferred; do NOT build here):
- An overview/metrics/dashboard page (total users, active subs, signup trends).
  Deliberately excluded — a metrics landing page doesn't match how this is
  operated; the user list is the landing page.
- Destructive ops: deleting users, cancelling Stripe subscriptions via API.
- Editing arbitrary user fields, impersonation, audit-log persistence, RBAC/roles
  beyond the single admin allowlist, CSV export.

## Global constraints

- **Zero new Go module dependencies.**
- New feature is **optional and env-gated**: empty `ADMIN_EMAILS` → no admins →
  the guard denies everyone (panel effectively disabled), matching the
  Google/Stripe pattern.
- Follow existing idioms: `config.New` env helpers, `database.Store` methods with
  `context`, `auth` middleware signatures (`func(http.Handler) http.Handler`),
  `rio.MakeHandler` handlers, `dom.*` views, the existing `card()` / `ui.Badge` /
  `breadcrumbs` / `pagination` / `csrfInput` building blocks, and table-driven
  tests with the existing helpers.
- All admin mutations require CSRF and are admin-guarded; admin actions are logged
  (actor email + target) via the app's `slog` logger.

## Access control

- `config`: add `AdminEmails []string`, parsed from `ADMIN_EMAILS` (comma-
  separated) — each entry trimmed and lowercased, empties dropped. A helper
  `csvEnv(key string) []string` performs the split/normalize.
- `auth.IsAdmin(email string, admins []string) bool` — normalized (lowercased,
  trimmed) membership test. Empty `admins` always returns false.
- `auth.RequireAdmin(admins []string) func(http.Handler) http.Handler` — composed
  AFTER `auth.RequireUser`:
  - no authenticated user → redirect to `/login?next=<path>` (same as RequireUser
    would; safe even if used alone),
  - authenticated non-admin → **HTTP 404** via `rio.Http404(w, ...)` so the admin
    surface is not advertised,
  - admin → pass through.

## Store additions (`database/admin.go`)

```go
// ListUsers returns users whose email contains query (case-insensitive; empty
// query matches all), newest first, paginated by limit/offset.
func (s *Store) ListUsers(ctx context.Context, query string, limit, offset int) ([]User, error)

// CountUsers returns the number of users matching query (empty = all).
// Used for pagination totals.
func (s *Store) CountUsers(ctx context.Context, query string) (int, error)

// RevokeEntitlement removes a product entitlement from a user (no error if
// absent). Counterpart to the existing GrantEntitlement.
func (s *Store) RevokeEntitlement(ctx context.Context, userID int64, productKey string) error
```

- `ListUsers`/`CountUsers` use `WHERE email LIKE '%'||?||'%'` (email column already
  has case-insensitive collation) and select via the existing `userColumns` /
  `scanUser`. `ListUsers` orders `created_at DESC, id DESC` with `LIMIT ? OFFSET ?`.
- `RevokeEntitlement`: `DELETE FROM entitlements WHERE user_id = ? AND product_key = ?`.
- Reuses existing `UserByID`, `ListEntitlements`, `ListUserSessions`,
  `DeleteUserSessions`, and the `User` subscription fields.

## Handlers (`handlers_admin.go`) + routes

All registered under `auth.RequireUser` → `auth.RequireAdmin(Conf.AdminEmails)`.
Handlers use `rio.MakeHandler` like the rest of the app. Page size constant
`adminPageSize = 25`.

- `GET /admin` — `HandleAdminIndex()`: redirects (303) to `/admin/users`. The
  admin area has a single landing page — the user list — so `/admin` is just the
  canonical entry point.
- `GET /admin/users` — `HandleAdminUsers(store)`: reads `q` (search) and `page`
  (1-based) from the query string; renders the paginated list. This is the admin
  landing page.
- `GET /admin/users/{id}` — `HandleAdminUserDetail(store)`: loads the user,
  entitlements, and sessions. 404 if the id is unknown/non-numeric.
- `POST /admin/users/{id}/entitlements/grant` — `HandleAdminGrantEntitlement(store)`:
  form field `product_key` must be a known catalog product (`Conf.ProductByKey`);
  grants it, flashes success, redirects to the detail page.
- `POST /admin/users/{id}/entitlements/revoke` — `HandleAdminRevokeEntitlement(store)`:
  form field `product_key`; revokes, flashes, redirects.
- `POST /admin/users/{id}/sessions/revoke` — `HandleAdminRevokeSessions(store)`:
  `DeleteUserSessions(userID, "")` (all sessions), flashes, redirects.

Route registration uses the standard-library `{id}` path pattern already used by
the router (Go 1.22+ `http.ServeMux`). Each POST validates CSRF (the app's
existing mechanism), logs the action, and uses PRG (redirect after post). Flash
messages are carried via a `?flash=<url-escaped msg>` query param on the redirect
target and read back with `r.URL.Query().Get("flash")` — the exact mechanism the
account area uses (e.g. `handlers_account.go` `/account/security?flash=...`). The
detail handler renders that flash.

## Views (`views/admin.go`)

- `adminShell(pd, meta, body...)` — page chrome: `pageHeader("Admin", …)` and a
  breadcrumb trail (`Home › Admin › <section>`). Mirrors `accountShell`'s
  structure, minus a subnav (the admin area is just the user list + detail, so a
  subnav would be noise).
- `adminUsers(...)` — a search form (`GET`, field `q`), a users table (email,
  name, created, subscription `ui.Badge`) where each row links to
  `/admin/users/{id}`, and the reused `pagination(current, total, "/admin/users")`
  (the pager must preserve `q`; see note). Purpose-built table, NOT the kit's
  `dataTable` (which hardcodes a demo View/Edit/Delete menu and no row links).
- `adminUserDetail(...)` — a key-value block (id, email, name, created, Google
  linked yes/no, Stripe customer id, subscription status + current period end),
  the entitlements list with a per-entitlement revoke button and a grant form
  (a `ui.Select` of catalog products + submit), and the sessions list (device via
  the existing `deviceLabel`, ip, created) with a "Revoke all sessions" button.
  All action forms include `csrfInput`.

Pagination note: `pagination` builds `href = baseHref?page=N`. To preserve the
search term, the admin users list passes `baseHref` of `/admin/users?q=<q>&` when
`q` is non-empty, so the appended `page=N` composes correctly; when `q` is empty
it passes `/admin/users`. This keeps `pagination` unchanged.

## Subscription-status → badge mapping

A small helper maps `User.SubscriptionStatus` to a `ui.BadgeVariant`:
`active`/`trialing` → `BadgeSuccess`; `past_due` → `BadgeWarning`;
`canceled` → `BadgeDanger`; `""`/other → `BadgeNeutral`. Used in both the list and
detail views.

## Testing

- **auth:** `IsAdmin` (member, non-member, case/whitespace-insensitive, empty
  allowlist → false); `RequireAdmin` (admin passes; non-admin → 404; logged-out →
  redirect; empty allowlist → 404 for a logged-in user).
- **config:** `ADMIN_EMAILS` parsing (`csvEnv`): unset → empty; spaces/case
  normalized; empties dropped.
- **store:** `ListUsers` (search substring match, newest-first order, limit/offset
  paging); `CountUsers` (with and without query); `RevokeEntitlement` (grant→revoke
  round-trip; revoking an absent entitlement is a no-op error-free). Use the
  existing DB test harness.
- **handlers:** `/admin/users` renders 200 for an admin and 404 for a logged-in
  non-admin; `/admin` redirects (303) to `/admin/users`; unknown user id → 404;
  grant POST adds the entitlement and redirects (303); revoke-sessions POST
  removes sessions and redirects.
- **views:** render smoke tests — a user row links to `/admin/users/<id>`; the
  detail page renders the grant form and a `csrfInput`.

## Config / docs

- `.env.example`: add `ADMIN_EMAILS=` under the ops section with a comment
  (comma-separated admin emails; empty disables the panel).
- README: add `ADMIN_EMAILS` to the env table and a short "Admin" subsection
  (what `/admin` is, how to enable it, that non-admins get a 404).

## Non-goals / YAGNI

- No roles/permissions beyond the single allowlist.
- No persisted audit log (actions are logged to stdout only).
- No destructive actions (delete user, cancel subscription).
- No overview/metrics/dashboard page — the user list is the landing page.
- No new generic view primitives; the admin views compose existing helpers
  (`card`, `ui.Badge`, `pagination`, `breadcrumbs`, `csrfInput`).
