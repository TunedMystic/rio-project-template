# Email Templates + Dev Preview — Design

**Date:** 2026-07-01
**Status:** Approved (pending spec review)

## Goal

Give the template branded, reusable HTML email templates (with plain-text
fallbacks) and a dev-only preview route so a developer can iterate on email
branding/copy in the browser without sending real mail. Today emails are
plain-text only and the single email (login magic link) is a hardcoded string.

## Scope

In scope:
- Extend the email transport to carry HTML + plain text.
- A reusable base email layout + helpers, rendered email-safe (inline styles,
  table layout, brand colors from `config.Tokens`).
- Convert the existing login magic-link email to the template.
- One generic `NotificationEmail` transactional template (title, body, optional
  CTA) — the copy-me starting point.
- A dev-only preview route rendering every email with sample data.

Out of scope (deferred; do NOT build):
- Additional emails (welcome, receipts, lifecycle) — consumers add these using
  `NotificationEmail` as the pattern.
- Live-reload of email templates, MJML/third-party email frameworks, attachments,
  localization, per-recipient personalization beyond passed-in data.
- Sending anything from the preview route.

## Global constraints

- **Zero new Go module dependencies** (uses `dom`, `net/http`, `config.Tokens`).
- Email HTML must be **email-client-safe**: a single centered table container,
  **all styling inline** (no `<style>`, no external CSS, no CSS variables, no
  Tailwind classes), colors as **hex** pulled from `config.Tokens` (the theme
  tokens are hex, e.g. `#4f46e5`, so no `oklch()` that Outlook can't render).
- Layering: `email` stays transport-only; `views` renders HTML/text and is
  transport-agnostic (returns strings, does not import `email`); handlers wire
  the two together. No import cycles.
- The preview route is **dev-only** — registered only when `Conf.Debug` is true
  (i.e. `make run` / `BuildEnv=debug`); it must not exist in a production build.
- Follow existing idioms: `dom.*` builders, `rio.MakeHandler` handlers, table-
  driven tests with existing helpers.

---

## Component 1: Email transport carries HTML + text (`email` package)

Replace the text-only `Send` with a message struct.

```go
// Message is a single outbound email. HTML is the rich body; Text is the
// plain-text fallback sent alongside it (multipart).
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}
```

- `Console.Send(ctx, msg)` — logs `to`, `subject`, and the `Text` body (and notes
  that an HTML body is present), keeping local dev zero-config.
- `Postmark.Send(ctx, msg)` — posts `HtmlBody: msg.HTML` and `TextBody: msg.Text`
  (plus `From`, `To`, `Subject`, `MessageStream: "outbound"`). The Postmark
  payload map becomes `map[string]string` with both bodies.
- Update `email/email_test.go` for the new signature (assert the Postmark request
  JSON contains both `HtmlBody` and `TextBody`; assert Console logs subject/text).

## Component 2: Email templates (`views/emails.go`, new file)

Rendered with `dom` but **email-safe** (inline styles, table layout). A helper
renders a `dom.Node` tree to a string.

```go
// EmailContext is the branding an email needs, built from Conf.
type EmailContext struct {
	SiteName string
	Tokens   ui.Tokens // hex colors: ColorPrimary, OnPrimary, ColorText, ColorTextMuted, ColorSurface, ColorBorder, ...
}

// renderString renders a dom.Node to an HTML string (bytes.Buffer + node.Render).
func renderString(n dom.Node) string
```

**Base layout + helpers** (email-safe, all inline styles):
- `emailLayout(ec EmailContext, inner ...dom.Node) dom.Node` — a complete HTML
  document: `<!doctype html>`, a body with a muted background, a centered ~600px
  table "card" (surface background, 1px border, radius), a brand header (the
  site name as a text wordmark, in `Tokens.ColorPrimary`), the `inner` content,
  and a muted footer (e.g. "Sent by <SiteName>"). No images.
- `emailHeading(ec, text) dom.Node` — an `<h1>`-ish heading, inline-styled with
  `Tokens.ColorText`.
- `emailParagraph(ec, text) dom.Node` — a `<p>` in `Tokens.ColorText`.
- `emailButton(ec, href, label) dom.Node` — a bulletproof-ish CTA: an `<a>`
  styled as a button (background `Tokens.ColorPrimary`, text `Tokens.OnPrimary`,
  padding, radius, `display:inline-block`), wrapped so it renders acceptably in
  major clients.

**Templates** — each returns `(subject, html, text string)`:

```go
// LoginEmail is the magic-link sign-in email.
func LoginEmail(ec EmailContext, link string) (subject, html, text string)

// NotificationEmail is the generic transactional template: a heading, one or
// more body paragraphs, and an optional CTA button (empty ctaLabel -> no button).
// This is the copy-me starting point for new emails.
func NotificationEmail(ec EmailContext, heading, body, ctaLabel, ctaHref string) (subject, html, text string)
```

- `LoginEmail`: subject "Your login link"; HTML = layout with a heading, a short
  paragraph, an `emailButton(link, "Log in")`, and a note that it expires in 15
  minutes + the raw link as fallback text; text = the current plain-text style
  ("Click to log in (expires in 15 minutes):\n\n<link>").
- `NotificationEmail`: subject = `heading`; HTML = layout with `emailHeading` +
  `emailParagraph(body)` + optional `emailButton`; text = `heading` + blank line
  + `body` + (if CTA) blank line + `ctaLabel + ": " + ctaHref`.

The plain-text bodies are hand-written (not derived from HTML) so they read well.

## Component 3: Login email wiring (`handlers_auth.go`)

Build the message from the view instead of the hardcoded string. Replace:

```go
body := "Click to log in (expires in 15 minutes):\n\n" + link
if err := sender.Send(r.Context(), emailAddr, "Your login link", body); err != nil {
```

with:

```go
subject, html, text := views.LoginEmail(emailContext(), link)
if err := sender.Send(r.Context(), email.Message{To: emailAddr, Subject: subject, HTML: html, Text: text}); err != nil {
```

Add a small `emailContext()` helper in the `main` package that builds
`views.EmailContext{SiteName: Conf.SiteName, Tokens: Conf.Tokens}` (reused by the
preview handlers). `handlers_auth.go` already imports `app/email` and `app/views`.

## Component 4: Dev preview route (`handlers_dev.go`, new file)

Registered only under `Conf.Debug` (see wiring). A single source of truth for the
preview catalog:

```go
// devEmail is one previewable email: a display name and a render func that
// produces (subject, html, text) from sample data + the email context.
type devEmail struct {
	Name   string // url slug, e.g. "login", "notification"
	Title  string // display title
	Render func(ec views.EmailContext) (subject, html, text string)
}

func devEmails() []devEmail // fixed catalog with sample data baked in
```

Sample data (baked into the catalog): login → a fake magic link
(`<BaseURL>/auth/verify?token=SAMPLE_TOKEN`); notification → sample heading/body
and a sample CTA.

Handlers:
- `HandleDevEmails()` — `GET /dev/emails`: an index HTML page listing each email
  with a link to its HTML preview and its `?format=text` preview, plus the
  subject line. Rendered with the app's normal `views`/`render` (it's a dev
  chrome page, not an email).
- `HandleDevEmailPreview()` — `GET /dev/emails/{name}`: looks up the catalog entry
  by `r.PathValue("name")`; unknown → 404. Default renders the email's **raw
  HTML** directly to the response (`Content-Type: text/html`) so it displays
  exactly as sent. `?format=text` writes the plain-text body as
  `text/plain; charset=utf-8`.

## Wiring (`main.go` `run()`)

After the existing routes, register the preview routes only in dev:

```go
	// Email previews (dev only): see /dev/emails to iterate on email branding.
	if Conf.Debug {
		s.Handle("/dev/emails", HandleDevEmails())
		s.Handle("GET /dev/emails/{name}", HandleDevEmailPreview())
	}
```

## Testing

- **email:** `Postmark.Send` posts a JSON body containing both `HtmlBody` and
  `TextBody` (httptest server asserting the payload); `Console.Send` logs the
  subject and text; both via the `Message` struct.
- **views/emails:** `LoginEmail` returns non-empty subject/html/text; the HTML
  contains the link, the site name, and the brand color (`Tokens.ColorPrimary`);
  the text contains the link. `NotificationEmail` includes the heading + body in
  both; renders a button when `ctaLabel != ""` and omits it (no `ctaHref`) when
  `ctaLabel == ""`. HTML is a full document (contains `<!doctype` and no Tailwind
  `class="` utility soup — inline `style=` only; assert it contains `style=` and
  the hex color).
- **preview handlers:** `/dev/emails` lists each catalog entry (contains "login"
  and "notification" and their subjects); `/dev/emails/login` → 200 `text/html`
  containing the sample link; `/dev/emails/login?format=text` → `text/plain`
  containing the link; unknown name → 404.
- **auth login:** update the existing login test for the new `Send(Message)`
  signature (assert the sent `Message` carries the verify link in HTML and/or
  text).

## Non-goals / YAGNI

- No extra email types beyond login + the generic notification.
- No email framework/MJML; hand-rolled inline-styled HTML is sufficient and
  dependency-free.
- No preview auth beyond the `Conf.Debug` gate (dev-only surface).
- Text fallbacks are hand-written, not auto-derived from HTML.
