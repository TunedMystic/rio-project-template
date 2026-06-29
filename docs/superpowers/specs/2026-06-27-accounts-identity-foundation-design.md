# Accounts Platform — Sub-project #1: Identity Foundation + Email Magic-Link — Design

**Date:** 2026-06-27
**Status:** Approved, ready for implementation planning
**Branch:** `accounts` (off `rewrite`)
**Target repo:** `github.com/TunedMystic/rio-project-template`

## Context: the bigger picture

The template is being extended from a minimal one-dependency starter into a
reusable **product platform** that every future product can clone. The user
sells subscription products and wants account/settings pages, Stripe billing,
and multiple login methods (email + Google) baked in once.

This is **four sub-systems**, designed and built one at a time, each with its
own spec → plan → build cycle:

| # | Sub-project | Depends on |
|---|---|---|
| **1** | **Identity foundation + email magic-link** (this spec) | — |
| 2 | (folded into #1: email magic-link is the first auth method) | 1 |
| 3 | Google OAuth login | 1 |
| 4 | Stripe subscriptions | 1 |

This spec covers **sub-project #1 only**: the users/sessions foundation, the
auth flow via email magic links, and the account/settings area shell.

## Decisions (locked during brainstorming)

| Decision | Choice | Rationale |
|---|---|---|
| Dependency philosophy | **Curated trusted libs where it matters** | Stdlib for the simple/safe parts; official libs (`x/oauth2`, `stripe-go`) only in #3/#4 where hand-rolling crypto is risky. **#1 adds zero new dependencies.** |
| Session model | **Server-side sessions** (DB-backed) | Revocable: "log out", "log out everywhere", session listing, instant invalidation. One cheap indexed lookup per request. |
| Account model | **Individual user accounts (B2C)** | One user = one login = one (future) Stripe customer. Orgs/teams deferred to a possible future sub-project. |
| Sign-up vs sign-in | **Unified** | Unknown email → create user; known email → sign in. Access is gated later by subscription (#4). Response is always "check your inbox" (no enumeration). |
| Email delivery | **Postmark REST in prod; Console log in dev** | When `POSTMARK_TOKEN` is unset, log the magic-link URL to the console — zero email setup to develop locally. |
| Token policy | **Single-use, 15-min, hashed at rest** | 32-byte `crypto/rand` token; DB stores only `sha256`. |
| CSRF posture | **`SameSite=Lax` primary + HMAC synchronizer token on authenticated forms** | Standard, sound posture; login form needs no token. |

## Out of scope (this sub-project)

- Google OAuth (#3), Stripe billing (#4 — Billing tab is a stub here).
- Organizations/teams, roles, invites.
- Email **change** flow (Profile shows email read-only in v1).
- Avatar upload.
- Password auth (passwordless only).

## Architecture & package layout

Dependency direction: `handlers → auth → database`, `handlers → email`. No cycles.

- **`database/`** (persistence; raw SQL, consistent with existing pattern)
  - `users.go` — `User` + `CreateUser`, `UserByEmail`, `UserByID`, `UpdateName`, `DeleteUser`
  - `sessions.go` — `Session` + `CreateSession`, `SessionByID`, `ListUserSessions`, `DeleteSession`, `DeleteUserSessions`, `DeleteExpiredSessions`
  - `tokens.go` — `LoginToken` + `CreateToken`, `ConsumeToken` (atomic single-use), `DeleteExpiredTokens`
  - `migrations/0002_accounts.sql`
- **`auth/`** (policy + crypto + http; → `database`)
  - `session.go` — signed-cookie read/write, `Create`/`Destroy`, current-user-from-request
  - `token.go` — magic-link issue/verify (hash, expiry, single-use), `next`-path validation
  - `middleware.go` — `LoadUser` (server-wide, → request context), `RequireUser` (redirect to `/login?next=`)
  - `csrf.go` — per-session token `HMAC(APP_SECRET, session_id)` issue + verify
  - `ratelimit.go` — small in-memory limiter for `/login`
- **`email/`** (no deps)
  - `email.go` — `Sender` interface; `Postmark` (REST via `net/http`) and `Console` (dev log) impls; chosen at startup
- **`views/`** (extends)
  - `auth.go` — `Login`, `LoginSent`, `VerifyError`
  - `account.go` — tabbed account layout + `Profile`, `Security`, `Billing` (stub), `Danger`
  - nav/flash additions to existing chrome
- **Root** — split the growing handler file:
  - `handlers_auth.go` — `HandleLogin` (GET/POST), `HandleVerify`, `HandleLogout`
  - `handlers_account.go` — `HandleProfile` (GET/POST), `HandleSecurity` (+ sign-out actions), `HandleDeleteAccount`
  - `handlers.go` keeps the public site
- **`config/`** — add `BaseURL`, `AppSecret`, `PostmarkToken`, `EmailFrom` (from env)

## Data model — `database/migrations/0002_accounts.sql`

```sql
CREATE TABLE users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT NOT NULL UNIQUE COLLATE NOCASE,   -- case-insensitive identity
    name       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    -- stripe_customer_id + subscription fields land in sub-project #4
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,                      -- sha256(cookie token); raw token never stored
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE login_tokens (
    token_hash TEXT PRIMARY KEY,                      -- sha256(magic-link token)
    email      TEXT NOT NULL COLLATE NOCASE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- Cookie holds a random token; DB stores only its `sha256` (a DB leak can't be replayed).
- `ON DELETE CASCADE` + the already-enabled `foreign_keys` pragma → deleting a user wipes their sessions.
- CSRF tokens are derived (`HMAC(secret, session_id)`), so no extra table.

## Auth flow (data flow)

1. **`GET /login`** → email field only (anonymous pre-auth form; no CSRF token, per the CSRF stance below).
2. **`POST /login`** → validate email (`forms.StrEmail`); rate-limit per email+IP; create token (`sha256` stored, 15-min expiry); build `BASE_URL/auth/verify?token=…`; hand to `email.Sender`. **Always** redirect to `/login/sent` (no enumeration; send failures only log).
3. **`GET /login/sent`** → "Check your email."
4. **`GET /auth/verify?token=…`** → hash; `ConsumeToken` (select+delete in one tx, reject if expired); **find-or-create** user by email; create session; set cookie; redirect to `/account` (or validated local `next`). On failure → "link expired or already used" page + re-request.
5. **`POST /logout`** → delete session row, clear cookie, redirect home.

## Session & cookie security

- Cookie: random token, `HttpOnly`, `Secure` (when not `Debug`), `SameSite=Lax`, `Path=/`, 30-day expiry. Server stores only `sha256(token)`.
- **`LoadUser`** middleware (server-wide via `s.Use`): cookie → hash → session lookup (not expired) → load user into request context, else nil.
- **`RequireUser`** wraps protected handlers: no user → `302 /login?next=<path>`.

### CSRF, rate-limiting, secret

- **CSRF:** primary defense `SameSite=Lax` (no session cookie on cross-site POSTs); defense-in-depth synchronizer token `HMAC(APP_SECRET, session_id)` in a hidden field on authenticated state-changing forms (profile, delete, sign-out), verified on POST. The pre-auth login form needs none.
- **Rate-limit:** in-memory limiter on `/login` (~5 links per email per 15 min). Adequate for a single-instance SQLite app.
- **`APP_SECRET`:** HMAC key for CSRF and signing. Dev → built-in default **with a logged warning**; prod → **required, fail-fast at startup if unset**.

## Account area UI

Reuses the warm teal design, `card()`, `ruledHeading`, rio/ui components.

- **Nav:** logged-out → "Log in"; logged-in → monogram avatar (initials) linking `/account` + logout. Public links stay.
- **Auth pages** (centered card): `/login`, `/login/sent`, verify-error.
- **Account area** — tabbed sub-layout (Profile · Security · Billing · Danger):
  - **Profile:** editable name, email read-only, Save → success flash.
  - **Security:** active sessions list ("this device" badge, UA / IP / created / expires), per-row Sign out + Sign out everywhere.
  - **Billing:** stub ("free plan", disabled "Manage billing") — filled in #4.
  - **Danger:** Delete account → confirm (type email) → cascade-delete → redirect home.
- **Flash:** minimal one-time signed cookie, rendered at the top of the next page.

## Config & env

Added to existing `DB_DIR`/`PORT`/`ADDR`:

| Env | Purpose | Dev default |
|---|---|---|
| `BASE_URL` | Absolute base for magic-link URLs | `http://localhost:<port>` |
| `APP_SECRET` | HMAC key (CSRF + signing) | built-in default + **warning**; prod **required** |
| `POSTMARK_TOKEN` | Postmark server token | unset → Console sender |
| `EMAIL_FROM` | From address | e.g. `noreply@localhost` |

README + Dockerfile env docs updated.

## Error handling & edge cases

- Invalid/expired/used token → friendly re-request page (not 500).
- Email send failure → logged; user still sees "check your inbox."
- Rate-limit exceeded → still generic "check your inbox."
- Unauthenticated `/account*` → `302 /login?next=`.
- Deleted/expired session → treated as logged out; cookie cleared lazily.
- `ConsumeToken` atomic (delete-in-tx) → truly single-use under concurrency.
- `next` param → **local paths only** (no open redirect).
- Missing `APP_SECRET` in prod → refuse to start.

## Testing

- **`database`:** case-insensitive unique email; session create/lookup/expiry; token single-use consume; cascade delete on user delete.
- **`auth`:** token issue/verify (expiry, reuse, bad token); session cookie round-trip; `LoadUser`/`RequireUser`; CSRF HMAC verify; `next`-path validation.
- **`email`:** Console logs; Postmark via `httptest` server asserting request shape (base URL injected).
- **handlers:** login POST (issues token, calls a fake `Sender`, redirects); verify (creates user+session+cookie); logout; protected-route redirects; profile update; delete; rate-limit — with a fake `Sender`.
- **views:** render assertions for auth + account pages.

## Build order (high level)

1. `database/`: migration `0002_accounts.sql`; `users.go`/`sessions.go`/`tokens.go` stores (+ round-trip tests).
2. `email/`: `Sender` interface, `Console`, `Postmark` (+ `httptest` test).
3. `config/`: `BaseURL`/`AppSecret`/`PostmarkToken`/`EmailFrom` from env (fail-fast on prod secret).
4. `auth/`: `token.go`, `session.go`, `middleware.go`, `csrf.go`, `ratelimit.go` (+ tests).
5. `views/`: `auth.go`, `account.go`, nav/flash (+ render tests).
6. handlers: `handlers_auth.go`, `handlers_account.go`; wire routes + `LoadUser` middleware in `main.go` (+ httptest).
7. README/Dockerfile env docs; full verify (`make check`, live magic-link round-trip via Console sender).

## Success criteria

- A new email logs in end-to-end via a console-logged magic link (dev), creating a user + session; the account area renders, profile name saves, sessions list shows the current device, "sign out everywhere" invalidates others, and account deletion cascades.
- Tokens are single-use, 15-min, hashed at rest; sessions are server-side and revocable; cookies are `HttpOnly`/`Secure`/`SameSite=Lax`.
- **No new Go dependency** is added in this sub-project.
- Prod refuses to start without `APP_SECRET`; dev runs with zero email/secret setup.
- The Billing tab is a stub ready for sub-project #4.
