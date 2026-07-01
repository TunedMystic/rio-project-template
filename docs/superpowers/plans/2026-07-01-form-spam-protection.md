# Public-Form Spam Protection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reusable no-JavaScript honeypot and a per-IP rate limit for public forms, and wire both into the login and messages forms as reference examples.

**Architecture:** A honeypot renderer + shared field-name constant live in `views`; a `honeypotTripped(r)` check and the existing `auth.Limiter` (reused, keyed by client IP) live in the `main` package. Login gets a silent honeypot drop (its existing rate limit is unchanged); messages gets a silent honeypot drop plus a friendly 429 + notice when its IP rate limit is exceeded.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio` (`dom`, `ui`, `forms`), existing `auth.Limiter` (`auth/ratelimit.go`), `rio.MakeHandler`. No new dependencies.

## Global Constraints

- **Zero new Go module dependencies.**
- **No client JavaScript** — the honeypot is pure HTML/CSS (an off-screen inline-styled input).
- Reuse existing primitives: `auth.Limiter`, `clientIP(r, Conf.TrustProxy)` (`handlers_auth.go`), `rio/forms`, `rio/dom`, `rio/ui`, `rio.MakeHandler`.
- **Single-instance only:** the in-memory `auth.Limiter` is reused as-is (limits reset on restart, not shared across replicas). Acceptable and intended.
- The honeypot is **always silent** (never signals it was caught). Login keeps its existing silent anti-enumeration behavior; generic public forms get an honest 429.
- **No Tailwind rebuild is required** — the honeypot uses an inline style and the notice reuses the already-compiled `ui.Alert` classes. Do **not** run `make tailwind`.
- Follow existing idioms; keep the full test suite (`go test ./...`) green.

---

### Task 1: Honeypot primitive + login wiring

**Files:**
- Create: `views/forms.go`
- Create: `views/forms_test.go`
- Modify: `views/auth.go` (add `Honeypot()` to the `Login` form)
- Modify: `handlers_auth.go` (silent honeypot drop in `HandleLogin`)
- Modify: `handlers.go` (add `honeypotTripped` helper)
- Test: `handlers_auth_test.go`

**Interfaces:**
- Consumes: `dom.*` builders; `render(...)` test helper (`views/views_test.go`); `auth.NewLimiter`, `fakeSender`, `authTestStore` (existing test helpers).
- Produces:
  - `const views.HoneypotName = "website"`
  - `func views.Honeypot() dom.Node`
  - `func honeypotTripped(r *http.Request) bool` (package `main`)

- [ ] **Step 1: Write the failing view test**

Create `views/forms_test.go`:

```go
package views

import (
	"strings"
	"testing"
)

func TestHoneypot_RendersHiddenDecoyField(t *testing.T) {
	html := render(Honeypot())
	for _, want := range []string{`name="website"`, "position:absolute", `aria-hidden="true"`, `tabindex="-1"`} {
		if !strings.Contains(html, want) {
			t.Errorf("Honeypot output missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./views/ -run TestHoneypot_RendersHiddenDecoyField -v`
Expected: FAIL — `undefined: Honeypot`.

- [ ] **Step 3: Implement the honeypot primitive**

Create `views/forms.go`:

```go
package views

import "github.com/tunedmystic/rio/dom"

// HoneypotName is the decoy field name shared by the honeypot renderer and the
// handler-side honeypotTripped check. A non-empty value on submit means a bot
// filled a field that real users and screen readers never see.
const HoneypotName = "website"

// Honeypot renders an off-screen decoy input for spam protection. It is hidden
// with an inline off-screen style (not display:none, so naive bots still fill
// it; inline so no Tailwind build is needed) and marked aria-hidden with a
// negative tabindex so humans and screen readers never interact with it.
func Honeypot() dom.Node {
	return dom.Div(
		dom.Style("position:absolute;left:-9999px"),
		dom.Aria("hidden", "true"),
		dom.Input(
			dom.Type("text"),
			dom.Name(HoneypotName),
			dom.Tabindex("-1"),
			dom.Autocomplete("off"),
		),
	)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./views/ -run TestHoneypot_RendersHiddenDecoyField -v`
Expected: PASS.

- [ ] **Step 5: Write the failing login honeypot test**

Add to `handlers_auth_test.go` (it already imports `app/auth`, `email`, `net/http`, `net/http/httptest`, `strings`, `testing`, `time`):

```go
func TestHandleLogin_HoneypotDropped(t *testing.T) {
	store := authTestStore(t)
	sender := &fakeSender{}
	h := HandleLogin(store, sender, auth.NewLimiter(5, time.Minute))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=bot@example.com&website=filled"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if sender.lastMsg.Text != "" {
		t.Errorf("honeypot submission should not send an email, got %q", sender.lastMsg.Text)
	}
}
```

- [ ] **Step 6: Run the test to verify it fails**

Run: `go test ./ -run TestHandleLogin_HoneypotDropped -v`
Expected: FAIL — `undefined: honeypotTripped` (the login handler doesn't drop honeypot submissions yet).

- [ ] **Step 7: Add the helper, wire login, and add the field**

In `handlers.go`, add the helper (the file already imports `net/http`, `strings`, and `app/views`):

```go
// honeypotTripped reports whether the decoy honeypot field was filled, which
// indicates an automated (bot) submission of a public form.
func honeypotTripped(r *http.Request) bool {
	return strings.TrimSpace(r.FormValue(views.HoneypotName)) != ""
}
```

In `handlers_auth.go` `HandleLogin`, insert the silent drop immediately after `emailAddr := strings.TrimSpace(r.FormValue("email"))` and before `form := forms.New()`:

```go
		// Silently drop bot submissions caught by the honeypot.
		if honeypotTripped(r) {
			http.Redirect(w, r, "/login/sent?email="+url.QueryEscape(emailAddr), http.StatusSeeOther)
			return nil
		}
```

In `views/auth.go`, add `Honeypot()` to the login `dom.Form`, immediately after the hidden `next` input:

```go
			dom.Input(dom.Type("hidden"), dom.Name("next"), dom.Value(next)),
			Honeypot(),
			ui.TextField("email", "Email address", email, errMsg,
```

- [ ] **Step 8: Run the tests and build**

Run: `go build ./... && go test ./ -run TestHandleLogin -v && go test ./views/`
Expected: PASS (login honeypot test + existing login tests + full views suite).

- [ ] **Step 9: Commit**

```bash
git add views/forms.go views/forms_test.go views/auth.go handlers.go handlers_auth.go handlers_auth_test.go
git commit -m "feat: honeypot spam protection for public forms + login wiring"
```

---

### Task 2: Public-form rate limiter + messages wiring + docs

**Files:**
- Modify: `views/pages.go` (`Messages` gains a `notice` param + `Honeypot()` field)
- Modify: `views/pages_test.go` (view test for honeypot + notice)
- Modify: `handlers.go` (`HandleMessages` signature + honeypot + rate limit + notice; add `app/auth` import)
- Modify: `handlers_test.go` (update existing calls to new signature; add honeypot + rate-limit tests; add `app/auth` + `time` imports)
- Modify: `main.go` (construct `publicFormLimiter`, inject into `HandleMessages`)
- Modify: `README.md` (protect-a-new-form note)

**Interfaces:**
- Consumes: `views.Honeypot`, `views.HoneypotName`, `honeypotTripped` (Task 1); `auth.NewLimiter`, `auth.Limiter.Allow`, `clientIP`, `ui.Alert`, `ui.AlertWarning`.
- Produces:
  - `func views.Messages(pd config.PageData, meta config.Meta, msgs []database.Message, bodyValue, bodyErr, notice string) dom.Node`
  - `func HandleMessages(store *database.Store, limiter *auth.Limiter) http.Handler`

- [ ] **Step 1: Write the failing view test**

Add to `views/pages_test.go`. It imports `bytes`, `strings`, `testing`, and `app/config`; **add `"app/database"` to its imports** (it is not currently imported):

```go
func TestMessages_RendersHoneypotAndNotice(t *testing.T) {
	var msgs []database.Message
	withNotice := render(Messages(testPageData(), config.Meta{Title: "Messages"}, msgs, "", "", "Too many submissions, please try again shortly."))
	if !strings.Contains(withNotice, `name="website"`) {
		t.Error("Messages missing honeypot field")
	}
	if !strings.Contains(withNotice, "Too many submissions") {
		t.Error("Messages missing notice text when notice is set")
	}

	noNotice := render(Messages(testPageData(), config.Meta{Title: "Messages"}, msgs, "", "", ""))
	if strings.Contains(noNotice, "Too many submissions") {
		t.Error("Messages should not render a notice when it is empty")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./views/ -run TestMessages_RendersHoneypotAndNotice -v`
Expected: FAIL — too many arguments to `Messages` (old signature has no `notice`).

- [ ] **Step 3: Update the `Messages` view**

In `views/pages.go`, replace the whole `Messages` function with this version (adds the `notice` param, an optional `ui.Alert` above the form, and the `Honeypot()` field):

```go
func Messages(pd config.PageData, meta config.Meta, msgs []database.Message, bodyValue, bodyErr, notice string) dom.Node {
	formCard := []dom.Node{ruledHeading("Add a message")}
	if notice != "" {
		formCard = append(formCard, dom.Div(dom.Class("mt-4"), ui.Alert(ui.AlertWarning, dom.Text(notice))))
	}
	formCard = append(formCard,
		dom.Form(
			dom.Method("post"),
			dom.Action("/messages"),
			dom.Class("mt-6"),
			Honeypot(),
			ui.TextField("body", "Message", bodyValue, bodyErr),
			submitButton("Add message"),
		),
	)
	return Page(pd, meta,
		pageHeader("Messages", "A SQLite-backed demo. Add a message and it persists across restarts."),
		dom.Section(
			dom.Class("py-12"),
			shell(
				dom.Div(
					dom.Class("grid items-start gap-6 lg:grid-cols-[5fr_3fr]"),

					// Left: the form, then the recent messages.
					dom.Div(
						card(formCard...),
						messagesList(msgs),
					),

					// Right: a summary card echoing the checkout "order summary".
					card(
						ruledHeading("About this demo"),
						dom.Div(
							dom.Class("mt-2"),
							summaryRow("Storage", "SQLite"),
							summaryRow("Driver", "modernc (cgo-free)"),
							summaryRow("Persistence", "Across restarts"),
							summaryRow("Rendering", "rio/dom"),
							summaryTotal("Messages stored", strconv.Itoa(len(msgs))),
						),
					),
				),
			),
		),
	)
}
```

- [ ] **Step 4: Run the view test to verify it passes**

Run: `go test ./views/ -run TestMessages_RendersHoneypotAndNotice -v && go test ./views/`
Expected: PASS (new test + full views suite). The `main` package will not compile yet — that is fixed in the next steps.

- [ ] **Step 5: Write the failing handler tests (and update existing ones)**

In `handlers_test.go`, **add `"app/auth"` and `"time"` to the imports**. Update the three existing `HandleMessages(store)` calls to pass a limiter:

- Line ~55 (`TestHandleMessages_GET`): `HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)`
- Line ~71 (`TestHandleMessages_POSTCreatesAndRedirects`): `HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)`
- Line ~105 (`TestHandleMessages_POSTBlankShowsError`): `HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)`

Then add the two new tests:

```go
func TestHandleMessages_HoneypotDropped(t *testing.T) {
	store := newTestStore(t)

	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body=spam&website=filled"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	HandleMessages(store, auth.NewLimiter(5, time.Minute)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 0 {
		t.Errorf("honeypot submission should not persist, got %d", len(msgs))
	}
}

func TestHandleMessages_RateLimited(t *testing.T) {
	store := newTestStore(t)
	h := HandleMessages(store, auth.NewLimiter(1, time.Minute)) // allow 1, block the 2nd

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body="+body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	if rec := post("first"); rec.Code != http.StatusSeeOther {
		t.Fatalf("first status=%d, want 303", rec.Code)
	}
	rec := post("second")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Too many submissions") {
		t.Error("rate-limited response missing notice")
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 1 {
		t.Errorf("only the first message should persist, got %d", len(msgs))
	}
}
```

(Both requests share the default `httptest` `RemoteAddr` `192.0.2.1`, so the second trips the per-IP limit.)

- [ ] **Step 6: Run the tests to verify they fail**

Run: `go test ./ -run TestHandleMessages -v`
Expected: FAIL — too many arguments to `HandleMessages` (signature not updated yet).

- [ ] **Step 7: Update `HandleMessages` and wire the limiter**

In `handlers.go`, **add `"app/auth"` to the imports**, then replace the whole `HandleMessages` function:

```go
func HandleMessages(store *database.Store, limiter *auth.Limiter) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == http.MethodPost {
			// Silently drop bot submissions caught by the honeypot.
			if honeypotTripped(r) {
				http.Redirect(w, r, "/messages", http.StatusSeeOther)
				return nil
			}

			// Rate-limit this public form by client IP.
			if !limiter.Allow(clientIP(r, Conf.TrustProxy)) {
				msgs, err := store.ListMessages(r.Context())
				if err != nil {
					return err
				}
				meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
				return render(w, http.StatusTooManyRequests,
					views.Messages(Conf.PageDataFor(account(r)), meta, msgs, r.FormValue("body"), "",
						"Too many submissions, please try again shortly."))
			}

			// Validate with rio/forms: required, and a sane max length.
			body := strings.TrimSpace(r.FormValue("body"))
			form := forms.New()
			form.CleanString("body", body, forms.StrRequired(), forms.StrLte(280))

			if !form.IsValid() {
				msgs, err := store.ListMessages(r.Context())
				if err != nil {
					return err
				}
				field := form.MustField("body")
				meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
				return render(w, http.StatusUnprocessableEntity,
					views.Messages(Conf.PageDataFor(account(r)), meta, msgs, field.Value(), field.Err().Error(), ""))
			}

			if err := store.CreateMessage(r.Context(), form.CleanedString("body")); err != nil {
				return err
			}
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
			return nil
		}

		msgs, err := store.ListMessages(r.Context())
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
		return render(w, http.StatusOK, views.Messages(Conf.PageDataFor(account(r)), meta, msgs, "", "", ""))
	}
	return rio.MakeHandler(fn)
}
```

In `main.go` `run()`, construct the limiter near the existing `loginLimiter := auth.NewLimiter(5, 15*time.Minute)` line:

```go
	publicFormLimiter := auth.NewLimiter(5, 10*time.Minute)
```

and update the messages route registration:

```go
	s.Handle("/messages", HandleMessages(store, publicFormLimiter))
```

- [ ] **Step 8: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 9: Add the README note**

In `README.md`, add a short subsection near the theming/dev notes (e.g. right after the `/pages` + `/dev/emails` note added earlier):

```markdown
Public forms are protected from bots with a no-JavaScript honeypot and a per-IP
rate limit (single-instance, in-memory). To protect a new public form: add
`views.Honeypot()` inside the `<form>`, call `honeypotTripped(r)` at the top of
the POST handler and drop silently if true, and gate submissions with an injected
`auth.Limiter` via `limiter.Allow(clientIP(r, Conf.TrustProxy))`.
```

- [ ] **Step 10: Commit**

```bash
git add views/pages.go views/pages_test.go handlers.go handlers_test.go main.go README.md
git commit -m "feat: per-IP rate limiting + honeypot on the messages form"
```

---

## Self-Review

**Spec coverage:**
- Honeypot primitive (`views.HoneypotName` + `views.Honeypot()`) → Task 1 Steps 1–4. ✓
- `honeypotTripped(r)` handler helper → Task 1 Step 7. ✓
- Rate-limit primitive (reuse `auth.Limiter`, keyed by IP, injected from `main`) → Task 2 Steps 5–7. ✓
- Login: silent honeypot drop, existing rate limit unchanged → Task 1 Steps 7–8. ✓
- Messages: silent honeypot drop + friendly 429 + notice → Task 2 Steps 3, 7. ✓
- Form-level `notice` param on `views.Messages` (rendered via `ui.Alert`) → Task 2 Steps 1–3. ✓
- README "protect a new form" note → Task 2 Step 9. ✓
- Tests: honeypot drop (login + messages), rate-limit 429, view render → Tasks 1 & 2. ✓
- No new deps; no Tailwind rebuild; single-instance limiter → Global Constraints, enforced. ✓

**Placeholder scan:** No TBD/TODO; every code step contains complete code and exact commands.

**Type consistency:** `views.HoneypotName` (const), `views.Honeypot() dom.Node`, `honeypotTripped(r *http.Request) bool`, `views.Messages(pd, meta, msgs, bodyValue, bodyErr, notice string)`, and `HandleMessages(store *database.Store, limiter *auth.Limiter)` are referenced identically across both tasks and the tests. New imports called out explicitly: `app/auth` in `handlers.go` and `handlers_test.go`; `time` in `handlers_test.go`; `app/database` in `views/pages_test.go`.
