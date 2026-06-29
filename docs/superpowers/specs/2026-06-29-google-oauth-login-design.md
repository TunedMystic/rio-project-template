# Accounts Platform — Sub-project #3: Google OAuth Login — Design

**Date:** 2026-06-29
**Status:** Approved, ready for implementation planning
**Branch:** `google-oauth` (off `accounts`)
**Target repo:** `github.com/TunedMystic/rio-project-template`
**Depends on:** Sub-project #1 (Identity Foundation + Email Magic-Link)

## Context

Sub-project #1 added the identity foundation: a `users` table keyed by
case-insensitive email, server-side DB-backed sessions, passwordless email
magic-link login, and a tabbed account area. This sub-project adds **"Continue
with Google"** as a second login method alongside magic-link, plus
connect/disconnect management in the account's Security tab.

Google login reuses the entire session machinery from #1 — it only adds a new
*way to authenticate*; once authenticated, a user gets the same `sessions` row,
the same cookie, and the same "active sessions / revoke / logout" behaviour.

## Decisions (locked during brainstorming)

| Decision | Choice | Rationale |
|---|---|---|
| Identity model | **`google_id` column on `users`** | Store Google's stable `sub`. Match by `google_id` → else by verified email (link) → else create. One table; survives the user changing their Google email; fits B2C "one user = one account". |
| Account linking | **Link by verified email** | On first Google login, if a user already exists for that verified email, attach `google_id` to it (no duplicate account). Only ever link when Google reports `email_verified == true`. |
| Linking scope | **Login + connect/disconnect** | The Security tab gains a "Login methods" section to connect or disconnect Google for the signed-in account. |
| Disconnect guard | **None needed** | Email magic-link is always available to every account, so removing Google can never lock anyone out. |
| Profile data | **Name backfill only** | Set `name` from Google only when ours is empty; never overwrite. Keep the monogram nav avatar — no avatar column, no external image dependency. |
| OAuth library | **`golang.org/x/oauth2` (+ `/google`)** | Official, well-vendored. The one new dependency family (already approved in the #1 spec's dependency philosophy). |
| Userinfo retrieval | **Userinfo endpoint via the access token** | After the server-to-server code exchange (TLS direct to Google), `GET` the OpenID userinfo endpoint for `sub`/`email`/`email_verified`/`name`. Avoids pulling in a JWT/JWKS verification dependency; trustworthy because the token was obtained directly from Google. |
| OAuth CSRF | **Signed `state` cookie + PKCE** | A random `state` round-tripped through a short-lived signed cookie, plus PKCE (S256) via x/oauth2 helpers. Defense-in-depth even as a confidential client. |
| Unverified email | **Reject** | If `email_verified` is false, refuse the login (prevents email-based account takeover). |
| Dev experience | **Graceful degradation** | When `GOOGLE_CLIENT_ID`/`GOOGLE_CLIENT_SECRET` are unset, `Conf.GoogleEnabled()` is false: the login page hides the Google button and the Google routes 404/redirect. Mirrors #1's Console-email "no setup in dev" pattern. |

## Out of scope (this sub-project)

- Other providers (GitHub, Apple, Microsoft) — the `google_id` column is
  single-provider by design; a future sub-project can generalize to an
  `oauth_identities` table if a second provider is ever needed.
- Avatar/profile-picture storage and display.
- Token refresh / calling Google APIs after login — we need identity only, so
  the access token is used once (userinfo) and discarded.
- Organizations/teams linking; Stripe (sub-project #4).

## Architecture & package layout

Dependency direction is unchanged: `handlers → auth → database`. Google login
adds one new file in `auth/`, extends `database/users.go`, and adds handlers +
view sections. No cycles.

- **`database/`**
  - `migrations/0003_google.sql` — add `google_id` + partial unique index.
  - `users.go` (extend) — `UserByGoogleID`, `SetUserGoogleID`,
    `ClearUserGoogleID`; `User` struct gains `GoogleID string` (empty when
    unlinked).
- **`auth/`**
  - `google.go` (new) — OAuth2 config builder from env + `BaseURL`;
    `GoogleEnabled()`; state-cookie issue/verify (carrying `state`, `next`,
    `mode`); userinfo fetch + `GoogleUser{Sub, Email, EmailVerified, Name}`;
    PKCE helpers.
- **Root handlers**
  - `handlers_auth.go` (extend) — `HandleGoogleLogin` (GET → redirect to
    Google), `HandleGoogleCallback` (GET → exchange/fetch/link/session).
  - `handlers_account.go` (extend) — `HandleDisconnectGoogle` (POST, CSRF).
- **`views/`**
  - `auth.go` (extend) — "Continue with Google" button on the login card
    (rendered only when `GoogleEnabled`), with an "or" divider above the
    email form.
  - `account.go` (extend) — a "Login methods" section in the Security tab:
    email magic-link (always active) and Google (Connect / Disconnect).
- **`config/`** — add `GoogleClientID`, `GoogleClientSecret` (from env) and a
  `GoogleEnabled()` method (true when both are set). The `Login` view receives
  this as an explicit `bool`; handlers gate on it directly.
- **`main.go`** — register `/auth/google/login`, `/auth/google/callback`
  (public, but callback acts on the current user when `mode=link`); register
  `/account/google/disconnect` under `RequireUser`. `go mod tidy && go mod
  vendor` for x/oauth2.

## Data model — `database/migrations/0003_google.sql`

```sql
ALTER TABLE users ADD COLUMN google_id TEXT;

-- Partial unique index: at most one user per Google account, while still
-- allowing many users with no Google link (NULL).
CREATE UNIQUE INDEX idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
```

`User` gains `GoogleID string`. Reads scan `google_id` (nullable) into a
`sql.NullString` and flatten to `""` when absent, keeping the struct a plain
string consistent with the rest of the model.

New store methods:

- `UserByGoogleID(ctx, googleID string) (User, error)` — `sql.ErrNoRows` when absent.
- `SetUserGoogleID(ctx, userID int64, googleID string) error` — link.
- `ClearUserGoogleID(ctx, userID int64) error` — disconnect (`SET google_id = NULL`).

## OAuth flow

### Login (`mode=login`)

1. `GET /auth/google/login` — if `!GoogleEnabled`, redirect to `/login`.
   Generate a random `state` and PKCE `verifier`. Set a short-lived (10-min)
   signed, `HttpOnly` cookie carrying `state`, the validated `next`
   (`auth.SafeNext`), `mode=login`, and the PKCE `verifier`. Redirect to
   Google's consent URL (`AuthCodeURL(state, S256ChallengeOption)`), scopes
   `openid email profile`.
2. `GET /auth/google/callback` — read the state cookie; reject if missing or
   if `r.URL.Query().Get("state")` ≠ the cookie's state. Exchange the `code`
   (with the PKCE verifier) for a token. `GET` the userinfo endpoint with the
   access token → `GoogleUser`. **If `!EmailVerified`, render an auth error
   page.** Then resolve the account:
   1. `UserByGoogleID(sub)` → that user.
   2. else `UserByEmail(email)` → link (`SetUserGoogleID`) → that user.
   3. else `CreateUser(email, name)` then `SetUserGoogleID(sub)`.

   Backfill: if the resolved user's `name == ""` and Google supplied a name,
   `UpdateUserName`. Mint a session (same `GenerateToken` → `CreateSession` →
   `SetSessionCookie` path as magic-link verify) and redirect to the cookie's
   `next` (default `/account`). Clear the state cookie.

### Connect (`mode=link`)

Same `/auth/google/login` entry, but invoked from the Security tab while
signed in; the state cookie records `mode=link`. On callback, instead of
find-or-create:

- If `UserByGoogleID(sub)` returns a *different* user → flash error "That
  Google account is already linked to another user," redirect to
  `/account/security`.
- Else `SetUserGoogleID(currentUser.ID, sub)` (and name backfill), flash
  "Google connected," redirect to `/account/security`.

`mode=link` requires a current user in context; if absent (e.g. session
expired mid-flow), fall back to treating the callback as a normal login.

### Disconnect

`POST /account/google/disconnect` (CSRF-checked, under `RequireUser`):
`ClearUserGoogleID(currentUser.ID)`, flash "Google disconnected," redirect to
`/account/security`. Always permitted (email magic-link remains).

## Config & graceful degradation

`config` reads `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` from env. The
OAuth redirect URL is derived as `BaseURL + "/auth/google/callback"`.
`Conf.GoogleEnabled()` is true only when both creds are set:

- Login page renders the Google button only when enabled.
- `/auth/google/login` and `/auth/google/callback` redirect to `/login` when
  disabled.
- The Security "Login methods" section shows Google's Connect action only when
  enabled (a linked-but-now-disabled account can still Disconnect).

No production secret fail-fast is needed (unlike `APP_SECRET`): Google login is
optional, so "disabled" is a valid production state.

## Security notes

- **State + PKCE** guard the authorization-code flow against CSRF and code
  interception.
- **`email_verified` is mandatory** before any email-based linking or account
  creation.
- The state cookie is `HttpOnly`, `Secure` when `!Debug`, `SameSite=Lax`,
  short TTL, and signed with `APP_SECRET` (reuse the existing HMAC/`ValidCSRF`
  primitives or an equivalent `auth` helper) so its contents can't be forged.
- The Google access token is used once (userinfo) and never stored; no refresh
  token is requested.
- Linking to an already-linked Google account is rejected, preventing one
  Google identity from attaching to two local users.

## Testing

Automated (Go tests, no network):

- `database/users_test.go` — `google_id` round-trip: set, look up by
  `google_id`, clear; partial-unique constraint rejects a second user with the
  same `google_id`.
- `auth/google_test.go` — state cookie issue/verify (accept matching, reject
  missing/mismatched/expired); `GoogleEnabled` gating; `GoogleUser` JSON
  decoding from a sample userinfo payload via `httptest`.
- Handler-level — the find/link/create decision and `email_verified` rejection,
  driven through a seam that lets the test supply a fake `GoogleUser` (so the
  callback logic is tested without hitting Google). Disconnect clears
  `google_id`; connect rejects an already-linked Google account.

Manual (documented in README, not automated): the live end-to-end round-trip
through Google's real consent screen with `GOOGLE_CLIENT_ID`/`SECRET` set and
the redirect URI registered in Google Cloud Console.

## Documentation

README "Accounts & auth" section gains a row for `GOOGLE_CLIENT_ID` /
`GOOGLE_CLIENT_SECRET` (purpose; "unset → Google login hidden"), plus a short
note on registering the redirect URI (`<BASE_URL>/auth/google/callback`) in
Google Cloud Console. Dockerfile env comment updated.
