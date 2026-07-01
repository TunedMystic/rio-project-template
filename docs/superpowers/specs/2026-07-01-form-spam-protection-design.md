# Public-Form Spam Protection (honeypot + rate limiting) — Design

**Date:** 2026-07-01
**Status:** Approved (pending spec review)

## Goal

Give the template a reusable, no-JavaScript way to protect public (unauthenticated)
form endpoints from naive bots and abusive submission volume, and apply it to the
two public forms that ship today (login, messages) as worked reference examples —
so any future public form (contact, signup, lead capture) can be protected in a
few lines.

## Scope

In scope:
- A reusable **honeypot** field + check (silently drops bot submissions).
- A reusable **per-IP rate limit** for public forms, reusing the existing
  `auth.Limiter`.
- Wiring both into the existing **login** and **messages** forms.
- A short README note on protecting a new form.

Out of scope (deferred; do NOT build):
- Timing-based honeypot (signed render-timestamp / min-fill-time check).
- CAPTCHA or any third-party bot-detection.
- A distributed / persistent rate limiter (the in-memory `auth.Limiter` is
  single-instance only, which matches this template's single-instance,
  single-file-SQLite deployment).
- Rate-limiting authenticated/admin actions (this is about *public* forms).

## Global constraints

- **Zero new Go module dependencies.**
- **No client JavaScript** — honeypot is pure HTML/CSS, consistent with the
  template's server-rendered philosophy.
- Reuse existing primitives: `auth.Limiter` (`auth/ratelimit.go`), `clientIP(r,
  Conf.TrustProxy)`, `rio/forms`, `rio/dom`, `rio/ui`, `rio.MakeHandler`.
- **Single-instance assumption:** the in-memory limiter is reused as-is; limits
  reset on restart and are not shared across replicas. This is acceptable and
  documented.
- Follow existing idioms and keep the existing test suite green.

## Behavior summary

| Trigger | Login (`/login`) | Public forms (`/messages`, future) |
|---|---|---|
| Honeypot filled | Silent — redirect to `/login/sent?...` as if success; no token issued | Silent — `303` redirect as if success; nothing written |
| Rate limit exceeded | **Unchanged** — existing email+IP limiter; always shows the same "check your email" page (anti-enumeration) | Re-render the form with a form-level notice + **HTTP 429** |

The honeypot is **always silent** (never reveals it was caught). Login keeps its
deliberate silent, anti-enumeration behavior; generic public forms get an honest,
friendly 429 because they have no enumeration concern.

## Component 1: Honeypot primitive (`views`)

A single source of truth for the decoy field name, plus a renderer and a checker.

```go
// HoneypotName is the decoy field name shared by the renderer and the handler
// check. A non-empty value on submit means a bot filled a field a human can't see.
const HoneypotName = "website"

// Honeypot renders an off-screen decoy input that humans and screen readers
// never interact with. Bots that fill every field will trip it.
func Honeypot() dom.Node
```

- Renders a wrapper hidden with an **inline** style `position:absolute;left:-9999px`
  (off-screen, not `display:none`; inline so it needs no Tailwind rebuild and
  can't be purged by the CSS build), containing:
  `<input type="text" name="website" tabindex="-1" autocomplete="off" aria-hidden="true">`.
- Lives in `views` (both the login and messages forms, which are in `views`, embed
  it; the `main` package references `views.HoneypotName` for the check).

Handler-side check, in the `main` package (near other handler helpers):

```go
// honeypotTripped reports whether the honeypot decoy field was filled (a bot).
func honeypotTripped(r *http.Request) bool {
	return strings.TrimSpace(r.FormValue(views.HoneypotName)) != ""
}
```

## Component 2: Rate-limit primitive (reuse `auth.Limiter`)

No new type. Reuse `auth.NewLimiter(max, window)` and `clientIP`, keyed by **IP
only** (public forms have no email/user identity).

- `main.run()` constructs one shared public-form limiter and injects it, mirroring
  the existing `loginLimiter`:

```go
publicFormLimiter := auth.NewLimiter(5, 10*time.Minute) // 5 submissions / 10 min / IP
```

- Gate pattern in a handler: `if !limiter.Allow(clientIP(r, Conf.TrustProxy)) { /* 429 */ }`.

## Component 3: Wiring the reference forms

### Login (`handlers_auth.go` + `views/auth.go`)

- `views.Login` form gains `views.Honeypot()`.
- At the **top of the POST branch** of `HandleLogin`, before validation:
  `if honeypotTripped(r) { http.Redirect(w, r, "/login/sent?email="+url.QueryEscape(emailAddr), http.StatusSeeOther); return nil }`.
- Existing email+IP rate limiter and anti-enumeration behavior are **unchanged**.

### Messages (`handlers.go` + `views/pages.go` + `main.go`)

- `HandleMessages` signature gains a limiter: `HandleMessages(store *database.Store, limiter *auth.Limiter)`.
  `main.go` injects `publicFormLimiter`.
- `views.Messages` form gains `views.Honeypot()`, and the view gains a **form-level
  notice** parameter rendered as a `rio/ui` alert above the form when non-empty:
  `Messages(pd, meta, msgs, bodyValue, bodyErr, notice string)`.
- POST order of operations in `HandleMessages`:
  1. **Honeypot:** `if honeypotTripped(r)` → `303` redirect to `/messages`
     (silent success, nothing written).
  2. **Rate limit:** else `if !limiter.Allow(clientIP(r, Conf.TrustProxy))` →
     re-render `views.Messages(..., notice: "Too many submissions, please try again shortly.")`
     with **HTTP 429** (`http.StatusTooManyRequests`).
  3. **Normal:** else validate (`rio/forms`) and `CreateMessage` as today. The
     normal render calls pass an empty `notice`.

## Component 4: Reusability (README)

Add a short note documenting that any new public form is protected in three steps:
1. Drop `views.Honeypot()` into the form body.
2. Call `honeypotTripped(r)` at the top of the POST handler and drop silently if true.
3. Inject an `auth.Limiter` and gate with `limiter.Allow(clientIP(r, Conf.TrustProxy))`.

## Testing

- **Honeypot (handlers):**
  - `POST /messages` with `website=<nonempty>` → message **not** stored, response
    is the success-shaped `303` redirect.
  - `POST /login` with `website=<nonempty>` → no login token created, response is
    the `/login/sent` redirect.
- **Rate limit (handler):** `POST /messages` 6× from the same IP within the window
  → the 6th returns **429** and the body contains the notice text; a valid message
  is not stored on the limited request.
- **View render:** `views.Messages(...)` and `views.Login(...)` output contains a
  hidden `name="website"` input; `views.Messages(..., notice)` renders the notice
  when non-empty and omits it when empty.
- Update existing handler/view tests for the new `HandleMessages` and
  `views.Messages` signatures; keep the full suite green.

## Files touched

- `views/` — new `Honeypot()` + `HoneypotName` (small new file, e.g.
  `views/forms.go`); `views/auth.go` (Login form); `views/pages.go` (Messages form
  + `notice` param).
- `handlers.go` — `HandleMessages` (honeypot + rate limit + notice), `honeypotTripped` helper.
- `handlers_auth.go` — `HandleLogin` honeypot check.
- `main.go` — construct `publicFormLimiter`, inject into `HandleMessages`.
- `README.md` — "protecting a new form" note.
- Tests — `handlers`/`views` test files.

## Non-goals / YAGNI

- No timing check, no CAPTCHA, no third-party services.
- No distributed/persistent limiter — in-memory, single-instance only.
- No new generic UI primitives beyond the honeypot field and reuse of the existing
  `rio/ui` alert for the notice.
