# Email Templates + Dev Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add branded HTML email templates (with plain-text fallbacks), convert the login email to them, add a generic notification template, and add a dev-only route to preview every email in the browser.

**Architecture:** Extend the `email` transport to carry an HTML+text `Message`. Render email HTML in `views` with `dom` but email-safe (inline styles, table layout, hex brand colors from `config.Tokens`); templates return `(subject, html, text)` strings so `views` stays transport-agnostic. Handlers wire views→transport and expose a dev-gated preview route.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio/dom` (incl. `dom.CreateAttr` for email attributes), `net/http`, `config.Tokens` (hex colors). No new dependencies.

## Global Constraints

- **Zero new Go module dependencies.**
- Email HTML must be email-client-safe: one centered table, **all styling inline** (no `<style>`, no external CSS, no CSS vars, no Tailwind classes), colors as **hex** from `config.Tokens` (e.g. `ColorPrimary` = `#4f46e5`) — never `oklch()`.
- Layering: `email` is transport-only; `views` renders and returns strings (must NOT import `email`); handlers wire them. No import cycles.
- Preview route is **dev-only** — registered only when `Conf.Debug` is true.
- Interpolated content (site name, body, links) goes through `dom.Text` (auto-escaped); email-specific attributes use `dom.CreateAttr(name, value)`.
- Follow existing idioms: `dom.*` builders, `rio.MakeHandler` handlers, table-driven tests. Run tests with `go test ./...`.

---

### Task 1: `email` transport carries HTML + text (`Message`)

**Files:**
- Modify: `email/email.go` (add `Message`, change `Sender`/`Console`/`Postmark`)
- Modify: `email/email_test.go` (new signature)
- Modify: `handlers_auth.go` (build a `Message` — plain text for now)
- Modify: `handlers_auth_test.go` (`fakeSender` to new signature)

**Interfaces:**
- Produces: `email.Message{To, Subject, HTML, Text string}`; `email.Sender.Send(ctx context.Context, msg Message) error`.

- [ ] **Step 1: Update the email tests to the new signature (failing)**

In `email/email_test.go`, replace the two `Send(...)` calls and add body assertions:

```go
func TestConsole_LogsMessage(t *testing.T) {
	var buf bytes.Buffer
	c := Console{Log: log.New(&buf, "", 0)}
	if err := c.Send(context.Background(), Message{To: "to@example.com", Subject: "Subject", Text: "Body link https://x/y"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "to@example.com") || !strings.Contains(out, "https://x/y") {
		t.Errorf("console output missing recipient or body: %q", out)
	}
}

func TestPostmark_PostsToAPI(t *testing.T) {
	var gotToken string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Postmark-Server-Token")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ErrorCode":0}`)
	}))
	defer srv.Close()

	p := Postmark{Token: "tok", From: "from@example.com", BaseURL: srv.URL, Client: srv.Client()}
	if err := p.Send(context.Background(), Message{To: "to@example.com", Subject: "Subj", HTML: "<b>hi</b>", Text: "the body"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotToken != "tok" {
		t.Errorf("token header = %q", gotToken)
	}
	if gotBody["To"] != "to@example.com" || gotBody["From"] != "from@example.com" {
		t.Errorf("body = %+v", gotBody)
	}
	if gotBody["HtmlBody"] != "<b>hi</b>" || gotBody["TextBody"] != "the body" {
		t.Errorf("body missing HtmlBody/TextBody: %+v", gotBody)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./email/ -v`
Expected: FAIL — `too many arguments`/`undefined: Message` (compile error).

- [ ] **Step 3: Implement `Message` + new signatures**

In `email/email.go`, replace the `Sender` interface, `Console.Send`, and `Postmark.Send`:

```go
// Message is a single outbound email. HTML is the rich body; Text is the
// plain-text fallback sent alongside it.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Sender delivers an email.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
```

```go
func (c Console) Send(ctx context.Context, msg Message) error {
	c.Log.Printf("[email] to=%s subject=%q\n%s", msg.To, msg.Subject, msg.Text)
	return nil
}
```

```go
func (p Postmark) Send(ctx context.Context, msg Message) error {
	payload, _ := json.Marshal(map[string]string{
		"From":          p.From,
		"To":            msg.To,
		"Subject":       msg.Subject,
		"HtmlBody":      msg.HTML,
		"TextBody":      msg.Text,
		"MessageStream": "outbound",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/email", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", p.Token)

	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("postmark: status %d", resp.StatusCode)
	}
	return nil
}
```

The `Sender` interface doc comment update and struct are the only other changes; `New` is unchanged.

- [ ] **Step 4: Update the login caller (plain text for now)**

In `handlers_auth.go`, inside `HandleLogin`, replace the `body := ...` line and the `sender.Send(...)` call with:

```go
					msg := email.Message{
						To:      emailAddr,
						Subject: "Your login link",
						Text:    "Click to log in (expires in 15 minutes):\n\n" + link,
					}
					if err := sender.Send(r.Context(), msg); err != nil {
						rio.LogError(err)
					}
```

(`handlers_auth.go` already imports `app/email`.)

- [ ] **Step 5: Update the fakeSender in the auth test**

In `handlers_auth_test.go`, replace the `fakeSender` type, its `Send`, and the assertion:

```go
type fakeSender struct{ lastMsg email.Message }

func (f *fakeSender) Send(ctx context.Context, msg email.Message) error {
	f.lastMsg = msg
	return nil
}
```

And update the assertion in `TestHandleLogin_POST_IssuesAndSends`:

```go
	if !strings.Contains(sender.lastMsg.Text, "/auth/verify?token=") {
		t.Errorf("sent message missing verify link: %q", sender.lastMsg.Text)
	}
```

Keep the `var _ email.Sender = (*fakeSender)(nil)` line.

- [ ] **Step 6: Build and run tests**

Run: `go build ./... && go test ./email/ ./ -v 2>&1 | tail -20`
Expected: PASS (email tests + the auth login test), build clean.

- [ ] **Step 7: Commit**

```bash
git add email/email.go email/email_test.go handlers_auth.go handlers_auth_test.go
git commit -m "feat: email transport carries HTML + text via Message"
```

---

### Task 2: email templates (`views/emails.go`)

**Files:**
- Create: `views/emails.go`
- Test: `views/emails_test.go`

**Interfaces:**
- Consumes: `config.PageData`, `config.Meta`, `ui.Tokens`, `dom.*`.
- Produces:
  - `type EmailContext struct { SiteName string; Tokens ui.Tokens }`
  - `func LoginEmail(ec EmailContext, link string) (subject, html, text string)`
  - `func NotificationEmail(ec EmailContext, heading, body, ctaLabel, ctaHref string) (subject, html, text string)`
  - `type EmailPreviewLink struct { Name, Title, Subject string }`
  - `func DevEmailsIndex(pd config.PageData, meta config.Meta, items []EmailPreviewLink) dom.Node`

- [ ] **Step 1: Write the failing tests**

Create `views/emails_test.go`:

```go
package views

import (
	"strings"
	"testing"

	"app/config"

	"github.com/tunedmystic/rio/ui"
)

func testEmailContext() EmailContext {
	return EmailContext{
		SiteName: "Testco",
		Tokens: ui.Tokens{
			ColorPrimary:    "#4f46e5",
			OnPrimary:       "#ffffff",
			ColorText:       "#0f172a",
			ColorTextMuted:  "#64748b",
			ColorSurface:    "#ffffff",
			ColorBackground: "#f4f5f7",
			ColorBorder:     "#e2e8f0",
			RadiusBase:      "0.5rem",
		},
	}
}

func TestLoginEmail(t *testing.T) {
	subject, html, text := LoginEmail(testEmailContext(), "https://x/auth/verify?token=ABC")
	if subject == "" {
		t.Error("empty subject")
	}
	for _, want := range []string{"<!doctype html>", "Testco", "https://x/auth/verify?token=ABC", "#4f46e5", "style="} {
		if !strings.Contains(html, want) {
			t.Errorf("login html missing %q", want)
		}
	}
	if strings.Contains(html, `class="`) {
		t.Error("email html must not use CSS classes (inline styles only)")
	}
	if !strings.Contains(text, "https://x/auth/verify?token=ABC") {
		t.Error("login text missing link")
	}
}

func TestNotificationEmail_WithCTA(t *testing.T) {
	subject, html, text := NotificationEmail(testEmailContext(), "Heads up", "Something happened.", "View", "https://x/view")
	if subject != "Heads up" {
		t.Errorf("subject = %q, want Heads up", subject)
	}
	for _, want := range []string{"Heads up", "Something happened.", "https://x/view"} {
		if !strings.Contains(html, want) {
			t.Errorf("notification html missing %q", want)
		}
	}
	if !strings.Contains(text, "View: https://x/view") {
		t.Error("notification text missing CTA")
	}
}

func TestNotificationEmail_NoCTA(t *testing.T) {
	_, html, _ := NotificationEmail(testEmailContext(), "Hi", "No button here.", "", "")
	if strings.Contains(html, "text-decoration:none") {
		t.Error("should not render a CTA button when ctaLabel is empty")
	}
}

func TestDevEmailsIndex(t *testing.T) {
	items := []EmailPreviewLink{{Name: "login", Title: "Login", Subject: "Your login link"}}
	html := render(DevEmailsIndex(testPageData(), config.Meta{Title: "Emails"}, items))
	if !strings.Contains(html, `/dev/emails/login`) || !strings.Contains(html, "Login") {
		t.Error("index missing preview link")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestLoginEmail|TestNotificationEmail|TestDevEmailsIndex' -v`
Expected: FAIL — `undefined: LoginEmail` etc.

- [ ] **Step 3: Write the implementation**

Create `views/emails.go`:

```go
package views

import (
	"bytes"

	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// EmailContext is the branding an email needs, built from the app config.
type EmailContext struct {
	SiteName string
	Tokens   ui.Tokens
}

// renderString renders a dom node tree to a string.
func renderString(n dom.Node) string {
	var b bytes.Buffer
	_ = n.Render(&b)
	return b.String()
}

// emailDoc renders the layout node and prepends an HTML5 doctype.
func emailDoc(n dom.Node) string {
	return "<!doctype html>\n" + renderString(n)
}

// emailLayout builds the email HTML document: a centered card with a brand
// header, the inner content, and a muted footer. All styling is inline and
// table-based for email-client compatibility.
func emailLayout(ec EmailContext, inner ...dom.Node) dom.Node {
	t := ec.Tokens

	header := dom.Tr(dom.Td(
		dom.Style("padding:24px 28px;border-bottom:1px solid "+t.ColorBorder),
		dom.Span(dom.Style("font-weight:700;font-size:18px;color:"+t.ColorPrimary), dom.Text(ec.SiteName)),
	))

	bodyTd := dom.Td(append([]dom.Node{dom.Style("padding:28px")}, inner...)...)

	footer := dom.Tr(dom.Td(
		dom.Style("padding:20px 28px;border-top:1px solid "+t.ColorBorder+";color:"+t.ColorTextMuted+";font-size:12px"),
		dom.Text("Sent by "+ec.SiteName),
	))

	card := dom.Table(
		dom.Role("presentation"), dom.Width("600"),
		dom.CreateAttr("cellpadding", "0"), dom.CreateAttr("cellspacing", "0"), dom.CreateAttr("border", "0"),
		dom.Style("max-width:600px;width:100%;background:"+t.ColorSurface+";border:1px solid "+t.ColorBorder+";border-radius:"+t.RadiusBase),
		header, dom.Tr(bodyTd), footer,
	)

	outer := dom.Table(
		dom.Role("presentation"), dom.Width("100%"),
		dom.CreateAttr("cellpadding", "0"), dom.CreateAttr("cellspacing", "0"), dom.CreateAttr("border", "0"),
		dom.Style("background:"+t.ColorBackground),
		dom.Tr(dom.Td(dom.CreateAttr("align", "center"), dom.Style("padding:24px"), card)),
	)

	return dom.Html(
		dom.Head(
			dom.Meta(dom.Charset("utf-8")),
			dom.Meta(dom.Name("viewport"), dom.Content("width=device-width,initial-scale=1")),
			dom.TitleEl(dom.Text(ec.SiteName)),
		),
		dom.Body(dom.Style("margin:0;padding:0;background:"+t.ColorBackground), outer),
	)
}

func emailHeading(ec EmailContext, text string) dom.Node {
	return dom.H1(
		dom.Style("margin:0 0 16px;font-size:22px;font-weight:700;color:"+ec.Tokens.ColorText),
		dom.Text(text),
	)
}

func emailParagraph(ec EmailContext, text string) dom.Node {
	return dom.P(
		dom.Style("margin:0 0 16px;font-size:15px;line-height:1.6;color:"+ec.Tokens.ColorText),
		dom.Text(text),
	)
}

func emailButton(ec EmailContext, href, label string) dom.Node {
	t := ec.Tokens
	return dom.P(
		dom.Style("margin:24px 0"),
		dom.A(
			dom.Href(href),
			dom.Style("display:inline-block;padding:12px 20px;background:"+t.ColorPrimary+";color:"+t.OnPrimary+";text-decoration:none;font-weight:600;border-radius:"+t.RadiusBase),
			dom.Text(label),
		),
	)
}

// LoginEmail is the magic-link sign-in email.
func LoginEmail(ec EmailContext, link string) (subject, html, text string) {
	subject = "Your login link"
	doc := emailLayout(ec,
		emailHeading(ec, "Sign in to "+ec.SiteName),
		emailParagraph(ec, "Click the button below to sign in. This link expires in 15 minutes."),
		emailButton(ec, link, "Log in"),
		emailParagraph(ec, "If the button doesn't work, copy and paste this link:"),
		dom.P(dom.Style("margin:0;font-size:13px;word-break:break-all;color:"+ec.Tokens.ColorTextMuted), dom.Text(link)),
	)
	html = emailDoc(doc)
	text = "Sign in to " + ec.SiteName + "\n\nClick to log in (expires in 15 minutes):\n\n" + link + "\n"
	return subject, html, text
}

// NotificationEmail is the generic transactional template: a heading, a body
// paragraph, and an optional CTA button (empty ctaLabel -> no button). Copy this
// as the starting point for new transactional emails.
func NotificationEmail(ec EmailContext, heading, body, ctaLabel, ctaHref string) (subject, html, text string) {
	subject = heading
	inner := []dom.Node{
		emailHeading(ec, heading),
		emailParagraph(ec, body),
	}
	if ctaLabel != "" {
		inner = append(inner, emailButton(ec, ctaHref, ctaLabel))
	}
	html = emailDoc(emailLayout(ec, inner...))
	text = heading + "\n\n" + body + "\n"
	if ctaLabel != "" {
		text += "\n" + ctaLabel + ": " + ctaHref + "\n"
	}
	return subject, html, text
}

// EmailPreviewLink is one row in the dev email index.
type EmailPreviewLink struct {
	Name    string
	Title   string
	Subject string
}

// DevEmailsIndex renders the dev-only list of previewable emails (app chrome).
func DevEmailsIndex(pd config.PageData, meta config.Meta, items []EmailPreviewLink) dom.Node {
	rows := make([]dom.Node, 0, len(items))
	for _, it := range items {
		rows = append(rows, card(
			dom.Div(dom.Class("font-semibold text-[var(--color-text)]"), dom.Text(it.Title)),
			dom.Div(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Subject: "+it.Subject)),
			dom.Div(
				dom.Class("mt-3 flex gap-4 text-[length:var(--font-size-sm)]"),
				dom.A(dom.Class("text-[var(--color-primary)] hover:underline"), dom.Href("/dev/emails/"+it.Name), dom.Text("HTML preview")),
				dom.A(dom.Class("text-[var(--color-primary)] hover:underline"), dom.Href("/dev/emails/"+it.Name+"?format=text"), dom.Text("Text version")),
			),
		))
	}
	return Page(pd, meta,
		pageHeader("Email previews", "Dev-only: how each email renders with sample data."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("max-w-2xl space-y-4", rows)...))),
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -run 'TestLoginEmail|TestNotificationEmail|TestDevEmailsIndex' -v && go test ./views/`
Expected: PASS (new tests + full views suite).

- [ ] **Step 5: Commit**

```bash
git add views/emails.go views/emails_test.go
git commit -m "feat: HTML email templates (login, notification) + preview index view"
```

---

### Task 3: wire the login email to the template

**Files:**
- Modify: `handlers.go` (add `emailContext()` helper)
- Modify: `handlers_auth.go` (use `views.LoginEmail`)
- Modify: `handlers_auth_test.go` (assert HTML carries the link)

**Interfaces:**
- Consumes: `views.LoginEmail`, `views.EmailContext` (Task 2); `email.Message` (Task 1); `Conf.SiteName`, `Conf.Tokens`.
- Produces: `func emailContext() views.EmailContext` (package `main`).

- [ ] **Step 1: Strengthen the login test (failing)**

In `handlers_auth_test.go` `TestHandleLogin_POST_IssuesAndSends`, add an HTML assertion after the existing text assertion:

```go
	if !strings.Contains(sender.lastMsg.HTML, "/auth/verify?token=") {
		t.Errorf("sent message HTML missing verify link: %q", sender.lastMsg.HTML)
	}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ -run TestHandleLogin_POST_IssuesAndSends -v`
Expected: FAIL — `lastMsg.HTML` is empty (login still sends text-only from Task 1).

- [ ] **Step 3: Add the `emailContext()` helper**

In `handlers.go`, add near the top (after the `render` helper):

```go
// emailContext builds the branding passed to email templates.
func emailContext() views.EmailContext {
	return views.EmailContext{SiteName: Conf.SiteName, Tokens: Conf.Tokens}
}
```

- [ ] **Step 4: Use the template in the login handler**

In `handlers_auth.go` `HandleLogin`, replace the `msg := email.Message{...}` block from Task 1 with:

```go
					subject, html, text := views.LoginEmail(emailContext(), link)
					msg := email.Message{To: emailAddr, Subject: subject, HTML: html, Text: text}
					if err := sender.Send(r.Context(), msg); err != nil {
						rio.LogError(err)
					}
```

(`handlers_auth.go` already imports `app/views` and `app/email`.)

- [ ] **Step 5: Run tests**

Run: `go build ./... && go test ./ -run TestHandleLogin -v && go test ./...`
Expected: PASS (login test now sees the link in both HTML and Text), full suite green.

- [ ] **Step 6: Commit**

```bash
git add handlers.go handlers_auth.go handlers_auth_test.go
git commit -m "feat: send the login email using the branded template"
```

---

### Task 4: dev-only email preview route

**Files:**
- Create: `handlers_dev.go`
- Create: `handlers_dev_test.go`
- Modify: `main.go` (register dev routes under `Conf.Debug`)

**Interfaces:**
- Consumes: `emailContext()` (Task 3); `views.LoginEmail`, `views.NotificationEmail`, `views.DevEmailsIndex`, `views.EmailPreviewLink` (Task 2); `Conf.BaseURL`, `Conf.PageDataFor`, `Conf.NewMeta`, `account`, `render`.
- Produces: `HandleDevEmails()`, `HandleDevEmailPreview()`, `devEmails() []devEmail`.

- [ ] **Step 1: Write the failing tests**

Create `handlers_dev_test.go`:

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleDevEmails_Index(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails", nil)
	rec := httptest.NewRecorder()
	HandleDevEmails().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/dev/emails/login") || !strings.Contains(body, "/dev/emails/notification") {
		t.Error("index missing preview links")
	}
}

func TestHandleDevEmailPreview_HTML(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails/login", nil)
	r.SetPathValue("name", "login")
	rec := httptest.NewRecorder()
	HandleDevEmailPreview().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "SAMPLE_TOKEN") {
		t.Error("preview missing sample link")
	}
}

func TestHandleDevEmailPreview_Text(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails/login?format=text", nil)
	r.SetPathValue("name", "login")
	rec := httptest.NewRecorder()
	HandleDevEmailPreview().ServeHTTP(rec, r)
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("content-type = %q, want text/plain", ct)
	}
	if !strings.Contains(rec.Body.String(), "SAMPLE_TOKEN") {
		t.Error("text preview missing sample link")
	}
}

func TestHandleDevEmailPreview_Unknown(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/dev/emails/nope", nil)
	r.SetPathValue("name", "nope")
	rec := httptest.NewRecorder()
	HandleDevEmailPreview().ServeHTTP(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./ -run TestHandleDevEmail -v`
Expected: FAIL — `undefined: HandleDevEmails` / `HandleDevEmailPreview`.

- [ ] **Step 3: Write the implementation**

Create `handlers_dev.go`:

```go
package main

import (
	"io"
	"net/http"

	"app/views"

	"github.com/tunedmystic/rio"
)

// devEmail is one previewable email: a slug, a display title, and a render func
// that produces (subject, html, text) from the email context + baked sample data.
type devEmail struct {
	Name   string
	Title  string
	Render func(ec views.EmailContext) (subject, html, text string)
}

// devEmails is the catalog of previewable emails, with sample data baked in.
func devEmails() []devEmail {
	sampleLink := Conf.BaseURL + "/auth/verify?token=SAMPLE_TOKEN"
	return []devEmail{
		{Name: "login", Title: "Login magic link", Render: func(ec views.EmailContext) (string, string, string) {
			return views.LoginEmail(ec, sampleLink)
		}},
		{Name: "notification", Title: "Generic notification", Render: func(ec views.EmailContext) (string, string, string) {
			return views.NotificationEmail(ec, "Your report is ready",
				"Your weekly report has finished generating and is ready to view.",
				"View report", Conf.BaseURL+"/account")
		}},
	}
}

// HandleDevEmails lists the previewable emails (dev only).
func HandleDevEmails() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		ec := emailContext()
		items := make([]views.EmailPreviewLink, 0)
		for _, de := range devEmails() {
			subject, _, _ := de.Render(ec)
			items = append(items, views.EmailPreviewLink{Name: de.Name, Title: de.Title, Subject: subject})
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Email previews")
		return render(w, http.StatusOK, views.DevEmailsIndex(Conf.PageDataFor(account(r)), meta, items))
	}
	return rio.MakeHandler(fn)
}

// HandleDevEmailPreview renders one email's HTML (or ?format=text) with sample
// data (dev only).
func HandleDevEmailPreview() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		name := r.PathValue("name")
		for _, de := range devEmails() {
			if de.Name != name {
				continue
			}
			_, html, text := de.Render(emailContext())
			if r.URL.Query().Get("format") == "text" {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = io.WriteString(w, text)
				return nil
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, html)
			return nil
		}
		http.NotFound(w, r)
		return nil
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Register the dev routes in `main.go`**

In `main.go` `run()`, immediately before `s.Handle("/static/", HandleStatic())`, add:

```go
	// Email previews (dev only): visit /dev/emails to iterate on email branding.
	if Conf.Debug {
		s.Handle("/dev/emails", HandleDevEmails())
		s.Handle("GET /dev/emails/{name}", HandleDevEmailPreview())
	}
```

- [ ] **Step 5: Run tests and full suite + build**

Run: `go build ./... && go test ./ -run TestHandleDevEmail -v && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 6: Live smoke test**

The dev routes register only under a debug build (`Conf.Debug`), so run with the debug ldflag (this is what `make run` does):
```bash
DB_DIR=/tmp go run -ldflags="-X 'main.BuildEnv=debug'" . >/tmp/mail.log 2>&1 &
sleep 3
curl -s -o /dev/null -w "index=%{http_code}\n" http://localhost:3000/dev/emails
echo "login preview has sample link: $(curl -s http://localhost:3000/dev/emails/login | grep -c SAMPLE_TOKEN)"
kill %1 2>/dev/null; rm -f /tmp/mail.log
```
Expected: `index=200` and `login preview has sample link: 1`. (A non-debug production build has `Conf.Debug=false`, so `/dev/emails` is correctly absent there.)

- [ ] **Step 7: Commit**

```bash
git add handlers_dev.go handlers_dev_test.go main.go
git commit -m "feat: dev-only /dev/emails preview route"
```

---

## Self-Review

**Spec coverage:**
- Transport carries HTML+text (`Message`, Postmark HtmlBody+TextBody, Console logs) → Task 1. ✓
- Base layout + helpers + `LoginEmail` + `NotificationEmail`, email-safe inline styles from `config.Tokens`, returns `(subject,html,text)`, views transport-agnostic → Task 2. ✓
- Login converted to the template → Task 3. ✓
- Dev-only preview: index + `{name}` HTML/`?format=text`, unknown→404, gated on `Conf.Debug` → Task 4. ✓
- Testing per spec (Postmark both bodies; login/notification html/text; CTA presence/absence; preview index/html/text/404; login handler) → covered across tasks. ✓
- Global constraints: zero new deps; inline-styles-only (test asserts no `class="` in email html); dev-gating; no import cycle (views returns strings) → enforced. ✓

**Placeholder scan:** No TBD/TODO; every step has complete code. The smoke test's first `bin/app` line intentionally demonstrates the dev-gate (prod build lacks the route) then uses the debug `go run`.

**Type consistency:** `email.Message{To,Subject,HTML,Text}` and `Send(ctx, Message)`; `EmailContext{SiteName, Tokens}`; `LoginEmail(ec, link)` / `NotificationEmail(ec, heading, body, ctaLabel, ctaHref)` returning `(subject, html, text)`; `EmailPreviewLink{Name,Title,Subject}`; `DevEmailsIndex(pd, meta, items)`; `emailContext()`; `devEmail{Name,Title,Render}` — referenced identically across Tasks 1–4. `dom.CreateAttr`, `dom.Role`, `dom.Charset`, `dom.Width`, `dom.TitleEl`, `dom.Meta` all verified present in the vendored `dom`.
