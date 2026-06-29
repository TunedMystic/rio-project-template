# Google OAuth Login — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add "Continue with Google" as a second login method alongside email magic-link, plus connect/disconnect management in the account Security tab.

**Architecture:** A nullable `google_id` column on `users` holds Google's stable `sub`. A new `auth/google.go` wraps `golang.org/x/oauth2` (config, PKCE, a signed state cookie, and a userinfo fetch). Two new handlers (`/auth/google/login`, `/auth/google/callback`) drive the OAuth code flow and reuse the existing server-side session machinery; a third (`/account/google/disconnect`) unlinks. Login resolves a Google identity by `google_id` → verified email (linking) → create.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio` v0.26.0 (`rio`, `rio/dom`, `rio/ui`), `modernc.org/sqlite`, and **one new dependency: `golang.org/x/oauth2`** (vendored).

## Global Constraints

- Module path `app`; internal imports `app/config`, `app/database`, `app/auth`, `app/email`, `app/views`.
- **Exactly one new Go dependency: `golang.org/x/oauth2`.** Do NOT import `golang.org/x/oauth2/google` (it pulls in `cloud.google.com/go/compute/metadata`); define Google's endpoints inline instead. After any go.mod change run `go mod tidy && go mod vendor`.
- Google's endpoints (inline): AuthURL `https://accounts.google.com/o/oauth2/auth`, TokenURL `https://oauth2.googleapis.com/token`. Scopes: `openid email profile`. Userinfo: `https://openidconnect.googleapis.com/v1/userinfo`.
- Raw SQL over `database/sql`; account access methods hang off the existing `*database.Store`. Migrations forward-only, embedded; add `database/migrations/0003_google.sql`.
- `google_id` is nullable with a **partial unique index** (`WHERE google_id IS NOT NULL`) so many users may have none but each Google account maps to one user.
- **`email_verified` from Google must be true** before linking by email or creating a user. Reject otherwise.
- The OAuth `state` cookie is `HttpOnly`, `Secure` when `!Debug`, `SameSite=Lax`, 10-min TTL, and signed with `APP_SECRET` by reusing `auth.CSRFToken`/`auth.ValidCSRF`. PKCE S256 via `oauth2` helpers.
- **Graceful degradation:** Google routes are registered only when `Conf.GoogleEnabled()` (both `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` set). The login button and Security "Connect" action render only when enabled.
- Google login reuses the existing session path: `auth.GenerateToken` → `store.CreateSession` → `auth.SetSessionCookie`. Name is backfilled from Google only when the local name is empty.
- New config from env: `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`.
- Run all tests with `go test ./...` from the repo root. TDD: failing test first. Commit after each task.

---

### Task 1: `google_id` column + user store methods

**Files:**
- Create: `database/migrations/0003_google.sql`
- Modify: `database/users.go`
- Test: `database/users_test.go` (append)

**Interfaces:**
- Consumes: existing `Store`, `CreateUser`, `UserByEmail`, `UserByID`, `UpdateUserName`, `scanUser`.
- Produces:
  - `User` gains field `GoogleID string` (empty when unlinked).
  - `func (s *Store) UserByGoogleID(ctx context.Context, googleID string) (User, error)` (`sql.ErrNoRows` if absent)
  - `func (s *Store) SetUserGoogleID(ctx context.Context, id int64, googleID string) error`
  - `func (s *Store) ClearUserGoogleID(ctx context.Context, id int64) error`

- [ ] **Step 1: Write the migration**

```sql
-- database/migrations/0003_google.sql
ALTER TABLE users ADD COLUMN google_id TEXT;

-- Partial unique index: at most one user per Google account, while still
-- allowing many users with no Google link (NULL).
CREATE UNIQUE INDEX idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;
```

- [ ] **Step 2: Write the failing test**

```go
// database/users_test.go  (append)

func TestUsers_GoogleIDLinkLookupClear(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u, _ := s.CreateUser(ctx, "g@example.com", "G")
	if u.GoogleID != "" {
		t.Fatalf("new user GoogleID = %q, want empty", u.GoogleID)
	}

	if err := s.SetUserGoogleID(ctx, u.ID, "sub-123"); err != nil {
		t.Fatalf("SetUserGoogleID: %v", err)
	}

	got, err := s.UserByGoogleID(ctx, "sub-123")
	if err != nil || got.ID != u.ID {
		t.Fatalf("UserByGoogleID = %+v, err %v", got, err)
	}
	if got.GoogleID != "sub-123" {
		t.Errorf("GoogleID = %q, want sub-123", got.GoogleID)
	}

	// The same google_id cannot map to a second user (partial unique index).
	u2, _ := s.CreateUser(ctx, "h@example.com", "H")
	if err := s.SetUserGoogleID(ctx, u2.ID, "sub-123"); err == nil {
		t.Error("expected unique-constraint error linking a duplicate google_id")
	}

	// Unlinking clears it.
	if err := s.ClearUserGoogleID(ctx, u.ID); err != nil {
		t.Fatalf("ClearUserGoogleID: %v", err)
	}
	if _, err := s.UserByGoogleID(ctx, "sub-123"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after clear err = %v, want sql.ErrNoRows", err)
	}
}
```

(`context`, `database/sql`, `errors`, `testing` are already imported by `users_test.go`.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./database/ -run TestUsers_GoogleID -v`
Expected: FAIL — `u.GoogleID undefined` / `undefined: (*Store).SetUserGoogleID`.

- [ ] **Step 4: Write the implementation**

Edit `database/users.go`. Add `"database/sql"` to the imports. Add the field to `User`:

```go
// User is an account holder. Email is the case-insensitive identity.
type User struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
	GoogleID  string // empty when no Google account is linked
}
```

Update the two existing lookups to select the new column (keep `CreateUser` as-is — a new user's `google_id` is NULL and `GoogleID` stays `""`):

```go
// UserByEmail looks up a user by email (case-insensitive via column collation).
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, google_id FROM users WHERE email = ?", email))
}

// UserByID looks up a user by id.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, google_id FROM users WHERE id = ?", id))
}
```

Add the new methods:

```go
// UserByGoogleID looks up a user by their linked Google account id (sub).
func (s *Store) UserByGoogleID(ctx context.Context, googleID string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at, google_id FROM users WHERE google_id = ?", googleID))
}

// SetUserGoogleID links a Google account to the user.
func (s *Store) SetUserGoogleID(ctx context.Context, id int64, googleID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET google_id = ? WHERE id = ?", googleID, id)
	return err
}

// ClearUserGoogleID unlinks the user's Google account.
func (s *Store) ClearUserGoogleID(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET google_id = NULL WHERE id = ?", id)
	return err
}
```

Update `scanUser` to scan the nullable column (this is why every lookup above now selects 5 columns):

```go
func (s *Store) scanUser(row rowScanner) (User, error) {
	var u User
	var gid sql.NullString
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &gid)
	u.GoogleID = gid.String
	return u, err
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./database/ -v`
Expected: PASS (all database tests, including the existing user/session/token suites which use the updated `scanUser`).

- [ ] **Step 6: Commit**

```bash
git add database/migrations/0003_google.sql database/users.go database/users_test.go
git commit -m "feat(db): google_id column and link/lookup/clear store methods"
```

---

### Task 2: Config — Google credentials + `GoogleEnabled` + PageData flag

**Files:**
- Modify: `config/config.go`
- Test: `config/config_test.go` (append)

**Interfaces:**
- Consumes: existing `Config`, `New`, `PageData`, `PageData()`.
- Produces:
  - `Config` gains `GoogleClientID, GoogleClientSecret string`.
  - `func (c Config) GoogleEnabled() bool` — true when both are set.
  - `PageData` gains `GoogleEnabled bool`, populated by `PageData()`.

- [ ] **Step 1: Write the failing test**

```go
// config/config_test.go  (append)

func TestGoogleEnabled_RequiresBothCreds(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	c := New("debug", "h")
	if c.GoogleClientID != "client-id" || c.GoogleClientSecret != "client-secret" {
		t.Fatalf("google creds not loaded: %+v", c)
	}
	if !c.GoogleEnabled() {
		t.Error("GoogleEnabled should be true when both creds are set")
	}
	if !c.PageData().GoogleEnabled {
		t.Error("PageData.GoogleEnabled should mirror GoogleEnabled()")
	}

	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	if New("debug", "h").GoogleEnabled() {
		t.Error("GoogleEnabled should be false when the secret is missing")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestGoogleEnabled -v`
Expected: FAIL — `c.GoogleClientID undefined` / `c.GoogleEnabled undefined`.

- [ ] **Step 3: Write the implementation**

In `config/config.go`:

Add to the `PageData` struct a field:

```go
	GoogleEnabled bool
```

Add to the `Config` struct:

```go
	GoogleClientID     string
	GoogleClientSecret string
```

In `New`, after the existing auth-env block (where `EmailFrom` is set), add:

```go
	c.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	c.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
```

In the `PageData()` method, add the field to the returned literal:

```go
		GoogleEnabled: c.GoogleEnabled(),
```

Add the method:

```go
// GoogleEnabled reports whether Google OAuth login is configured.
func (c Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "feat(config): GOOGLE_CLIENT_ID/SECRET, GoogleEnabled, PageData flag"
```

---

### Task 3: `auth/google.go` — OAuth config, PKCE, state cookie, userinfo

**Files:**
- Modify: `go.mod`, `go.sum`, `vendor/` (add `golang.org/x/oauth2`)
- Create: `auth/google.go`
- Test: `auth/google_test.go`

**Interfaces:**
- Consumes: stdlib; `golang.org/x/oauth2`; existing `auth.CSRFToken`, `auth.ValidCSRF`.
- Produces:
  - `type GoogleUser struct { Sub, Email, Name string; EmailVerified bool }`
  - `type OAuthState struct { State, Next, Mode, Verifier string }`
  - `type GoogleOAuth struct { ... ; UserinfoURL string }`
  - `func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth`
  - `func (g *GoogleOAuth) AuthCodeURL(state, verifier string) string`
  - `func (g *GoogleOAuth) Exchange(ctx, code, verifier string) (*oauth2.Token, error)`
  - `func (g *GoogleOAuth) FetchUser(ctx, token *oauth2.Token) (GoogleUser, error)`
  - `func NewVerifier() string`
  - `func SetStateCookie(w http.ResponseWriter, secret string, st OAuthState, secure bool)`
  - `func ReadStateCookie(r *http.Request, secret string) (OAuthState, bool)`
  - `func ClearStateCookie(w http.ResponseWriter, secure bool)`

- [ ] **Step 1: Add the dependency**

```bash
go get golang.org/x/oauth2@latest
go mod tidy
go mod vendor
```
Expected: `go.mod` gains `golang.org/x/oauth2`; `vendor/golang.org/x/oauth2/` appears. (`cloud.google.com/go/...` must NOT appear — we don't import the `/google` subpackage.)

- [ ] **Step 2: Write the failing test**

```go
// auth/google_test.go
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestStateCookie_RoundTripAndTamper(t *testing.T) {
	rec := httptest.NewRecorder()
	st := OAuthState{State: "abc", Next: "/account", Mode: "login", Verifier: "ver"}
	SetStateCookie(rec, "secret", st, true)

	c := rec.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("insecure state cookie flags: %+v", c)
	}

	req := httptest.NewRequest(http.MethodGet, "/cb", nil)
	req.AddCookie(c)
	got, ok := ReadStateCookie(req, "secret")
	if !ok || got != st {
		t.Fatalf("ReadStateCookie = %+v ok=%v, want %+v", got, ok, st)
	}

	// A wrong secret (forged signature) is rejected.
	req2 := httptest.NewRequest(http.MethodGet, "/cb", nil)
	req2.AddCookie(c)
	if _, ok := ReadStateCookie(req2, "different-secret"); ok {
		t.Error("ReadStateCookie accepted a cookie signed with another secret")
	}
}

func TestAuthCodeURL_HasStateAndPKCE(t *testing.T) {
	g := NewGoogleOAuth("cid", "csec", "https://app/cb")
	u := g.AuthCodeURL("state-xyz", NewVerifier())
	parsed, _ := url.Parse(u)
	if !strings.HasPrefix(u, "https://accounts.google.com/o/oauth2/auth") {
		t.Errorf("auth url = %q", u)
	}
	q := parsed.Query()
	if q.Get("state") != "state-xyz" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Errorf("missing PKCE challenge: %v", q)
	}
	if q.Get("client_id") != "cid" {
		t.Errorf("client_id = %q", q.Get("client_id"))
	}
}

func TestFetchUser_DecodesUserinfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":            "sub-1",
			"email":          "a@example.com",
			"email_verified": true,
			"name":           "Ada",
		})
	}))
	defer srv.Close()

	g := NewGoogleOAuth("cid", "csec", "https://app/cb")
	g.UserinfoURL = srv.URL
	gu, err := g.FetchUser(context.Background(), &oauth2.Token{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("FetchUser: %v", err)
	}
	if gu.Sub != "sub-1" || gu.Email != "a@example.com" || !gu.EmailVerified || gu.Name != "Ada" {
		t.Errorf("decoded user = %+v", gu)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./auth/ -run 'TestStateCookie|TestAuthCodeURL|TestFetchUser' -v`
Expected: FAIL — `undefined: OAuthState` / `undefined: NewGoogleOAuth`.

- [ ] **Step 4: Write the implementation**

```go
// auth/google.go
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	stateCookieName = "g_oauth"
	stateCookieTTL  = 10 * time.Minute
	googleUserinfo  = "https://openidconnect.googleapis.com/v1/userinfo"
)

// googleEndpoint is Google's OAuth2 endpoint, declared inline so we depend only
// on golang.org/x/oauth2 (importing .../oauth2/google would pull in
// cloud.google.com/go/compute/metadata).
var googleEndpoint = oauth2.Endpoint{
	AuthURL:  "https://accounts.google.com/o/oauth2/auth",
	TokenURL: "https://oauth2.googleapis.com/token",
}

// GoogleUser is the subset of Google's userinfo response we consume.
type GoogleUser struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// OAuthState is the data round-tripped through the signed state cookie.
type OAuthState struct {
	State    string `json:"s"`
	Next     string `json:"n"`
	Mode     string `json:"m"` // "login" | "link"
	Verifier string `json:"v"` // PKCE code verifier
}

// GoogleOAuth wraps the OAuth2 config and the userinfo endpoint (overridable in tests).
type GoogleOAuth struct {
	cfg         *oauth2.Config
	UserinfoURL string
}

// NewGoogleOAuth builds the Google OAuth2 client.
func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	return &GoogleOAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     googleEndpoint,
			Scopes:       []string{"openid", "email", "profile"},
		},
		UserinfoURL: googleUserinfo,
	}
}

// NewVerifier returns a fresh PKCE code verifier.
func NewVerifier() string { return oauth2.GenerateVerifier() }

// AuthCodeURL returns the consent URL carrying state and a PKCE S256 challenge.
func (g *GoogleOAuth) AuthCodeURL(state, verifier string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.S256ChallengeOption(verifier))
}

// Exchange swaps an authorization code (+ PKCE verifier) for a token.
func (g *GoogleOAuth) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	return g.cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
}

// FetchUser calls the userinfo endpoint with the token and decodes the profile.
func (g *GoogleOAuth) FetchUser(ctx context.Context, token *oauth2.Token) (GoogleUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.UserinfoURL, nil)
	if err != nil {
		return GoogleUser{}, err
	}
	resp, err := g.cfg.Client(ctx, token).Do(req)
	if err != nil {
		return GoogleUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GoogleUser{}, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var gu GoogleUser
	if err := json.NewDecoder(resp.Body).Decode(&gu); err != nil {
		return GoogleUser{}, err
	}
	return gu, nil
}

// SetStateCookie stores a signed OAuthState in a short-lived cookie. The value
// is base64(json) + "." + HMAC(secret, base64) using the existing CSRF primitive.
func SetStateCookie(w http.ResponseWriter, secret string, st OAuthState, secure bool) {
	payload, _ := json.Marshal(st)
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    b64 + "." + CSRFToken(secret, b64),
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(stateCookieTTL),
		MaxAge:   int(stateCookieTTL.Seconds()),
	})
}

// ReadStateCookie verifies the signature and decodes the OAuthState.
func ReadStateCookie(r *http.Request, secret string) (OAuthState, bool) {
	c, err := r.Cookie(stateCookieName)
	if err != nil {
		return OAuthState{}, false
	}
	b64, mac, found := strings.Cut(c.Value, ".")
	if !found || !ValidCSRF(secret, b64, mac) {
		return OAuthState{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return OAuthState{}, false
	}
	var st OAuthState
	if err := json.Unmarshal(payload, &st); err != nil {
		return OAuthState{}, false
	}
	return st, true
}

// ClearStateCookie removes the state cookie.
func ClearStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./auth/ -v`
Expected: PASS (all auth tests).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum vendor auth/google.go auth/google_test.go
git commit -m "feat(auth): Google OAuth config, PKCE, signed state cookie, userinfo fetch"
```

---

### Task 4: Views — login Google button + Security "Login methods" + flash plumbing

**Files:**
- Modify: `views/auth.go`, `views/account.go`, `handlers_account.go`
- Test: `views/auth_test.go` (append), `views/account_test.go` (modify)

**Interfaces:**
- Consumes: `config.PageData` (now has `GoogleEnabled`), `database.User.GoogleID`, existing view helpers + `ui.*`/`dom.*`, `auth.SessionFrom`, `auth.UserFrom`.
- Produces:
  - `views.AccountView` gains `Error string`.
  - `views.Security` signature gains a trailing `googleLinked bool`: `func Security(pd config.PageData, meta config.Meta, av AccountView, sessions []database.Session, currentID string, googleLinked bool) dom.Node`.
  - `views.Login` renders a "Continue with Google" button when `pd.GoogleEnabled` (signature unchanged).
  - `accountView(r, active)` now also reads `flash` and `err` query params.
  - `HandleSecurity` passes `user.GoogleID != ""` as `googleLinked`.

- [ ] **Step 1: Write the failing tests**

```go
// views/auth_test.go  (append)

func TestLogin_ShowsGoogleWhenEnabled(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "x")
	t.Setenv("GOOGLE_CLIENT_SECRET", "y")
	pd := config.New("debug", "h").PageData()
	var b bytes.Buffer
	_ = Login(pd, config.Meta{Title: "Log in"}, "", "", "/account").Render(&b)
	if !strings.Contains(b.String(), `href="/auth/google/login"`) {
		t.Error("expected Google login button when enabled")
	}
}

func TestLogin_HidesGoogleWhenDisabled(t *testing.T) {
	pd := testPageData() // GoogleEnabled == false
	var b bytes.Buffer
	_ = Login(pd, config.Meta{Title: "Log in"}, "", "", "/account").Render(&b)
	if strings.Contains(b.String(), `/auth/google/login`) {
		t.Error("Google button should be hidden when disabled")
	}
}
```

```go
// views/account_test.go  — update the existing TestSecurity_ListsSessions call
// to pass the new googleLinked arg, and add a connect/disconnect assertion.

func TestSecurity_ShowsLoginMethods(t *testing.T) {
	c := config.New("debug", "h")
	pdLinked := c.PageData()
	pdLinked.GoogleEnabled = true
	av := AccountView{Active: "security", CSRF: "c"}
	sessions := []database.Session{{ID: "cur", IP: "1.1.1.1"}}

	// Linked → shows a Disconnect form.
	var b bytes.Buffer
	_ = Security(pdLinked, config.Meta{Title: "Security"}, av, sessions, "cur", true).Render(&b)
	if !strings.Contains(b.String(), `action="/account/google/disconnect"`) {
		t.Error("linked account should show a Google disconnect form")
	}

	// Not linked, enabled → shows a Connect link.
	var b2 bytes.Buffer
	_ = Security(pdLinked, config.Meta{Title: "Security"}, av, sessions, "cur", false).Render(&b2)
	if !strings.Contains(b2.String(), `href="/auth/google/login?mode=link"`) {
		t.Error("unlinked account should show a Google connect link")
	}
}
```

Also update the **existing** `TestSecurity_ListsSessions` (from sub-project #1) to pass the new arg: change its `Security(...)` call to end with `, "cur", false)`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./views/ -run 'TestLogin_ShowsGoogle|TestSecurity_ShowsLoginMethods' -v`
Expected: FAIL — `pd.GoogleEnabled`/`AccountView.Error` undefined and `Security` arity mismatch.

- [ ] **Step 3: Add the Google login button to `views/auth.go`**

Add a helper and render it in `Login` (only when `pd.GoogleEnabled`), above the email form:

```go
// googleButton links to the Google OAuth login, styled as a neutral outline
// button with the Google "G" mark.
func googleButton() dom.Node {
	const g = `<svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.76h3.56c2.08-1.92 3.28-4.74 3.28-8.09z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.56-2.76c-.98.66-2.23 1.06-3.72 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23z"/><path fill="#FBBC05" d="M5.84 14.11a6.6 6.6 0 0 1 0-4.22V7.05H2.18a11 11 0 0 0 0 9.9z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.05l3.66 2.84C6.71 7.31 9.14 5.38 12 5.38z"/></svg>`
	return dom.A(
		dom.Class("inline-flex w-full items-center justify-center gap-2 rounded-[var(--radius-base)] border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-text)] shadow-sm transition hover:shadow-md hover:brightness-[0.99] cursor-pointer"),
		dom.Href("/auth/google/login"),
		dom.Raw(g),
		dom.Text("Continue with Google"),
	)
}

// orDivider is a horizontal rule with a centered "or" label.
func orDivider() dom.Node {
	return dom.Div(
		dom.Class("my-6 flex items-center gap-3"),
		dom.Div(dom.Class("h-px flex-1 bg-[var(--color-border)]")),
		dom.Span(dom.Class("text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("or")),
		dom.Div(dom.Class("h-px flex-1 bg-[var(--color-border)]")),
	)
}
```

In `Login`, insert the button + divider between the subtitle (`authText(...)`) and the `dom.Form(...)`. Build the card children conditionally:

```go
func Login(pd config.PageData, meta config.Meta, email, errMsg, next string) dom.Node {
	children := []dom.Node{
		authHeading("Log in"),
		// Non-breaking hyphen keeps "sign-in" from splitting across lines.
		authText("Enter your email to receive a sign‑in link."),
	}
	if pd.GoogleEnabled {
		children = append(children, dom.Div(dom.Class("mt-8"), googleButton()), orDivider())
	}
	children = append(children,
		dom.Form(
			dom.Method("post"),
			dom.Action("/login"),
			dom.Class(map[bool]string{true: "", false: "mt-8"}[pd.GoogleEnabled]),
			dom.Input(dom.Type("hidden"), dom.Name("next"), dom.Value(next)),
			ui.TextField("email", "Email address", email, errMsg,
				dom.Placeholder("you@example.com"),
				dom.Autocomplete("email"),
				dom.Autofocus(),
			),
			authSubmit("Send login link"),
		),
		dom.Hr(dom.Class("my-6 border-[var(--color-border)]")),
		dom.P(
			dom.Class("text-center text-balance text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
			dom.Text("We'll send you a magic link to sign in instantly. No password needed."),
		),
	)
	return Page(pd, meta, authCard(children...))
}
```

- [ ] **Step 4: Add `Error` + "Login methods" to `views/account.go`**

Add `Error string` to `AccountView`:

```go
type AccountView struct {
	Active string // "profile" | "security" | "billing" | "danger"
	CSRF   string
	Flash  string
	Error  string
}
```

In `accountShell`, render an error alert next to the existing flash alert. Replace the flash block:

```go
	if av.Flash != "" {
		content = append(content, ui.Alert(ui.AlertSuccess, dom.Text(av.Flash)))
	}
	if av.Error != "" {
		content = append(content, ui.Alert(ui.AlertError, dom.Text(av.Error)))
	}
```

Add a "Login methods" card and extend `Security`'s signature with `googleLinked bool`. Add this helper:

```go
// loginMethodsCard shows the account's sign-in methods with Google
// connect/disconnect controls.
func loginMethodsCard(pd config.PageData, av AccountView, googleLinked bool) dom.Node {
	googleRow := dom.Node(dom.Text(""))
	switch {
	case googleLinked:
		googleRow = dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-4 last:border-0"),
			dom.Div(dom.Class("min-w-0"),
				dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text("Google")),
				dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Connected")),
			),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/google/disconnect"),
				csrfInput(av.CSRF),
				dom.Button(dom.Type("submit"),
					dom.Class("shrink-0 rounded-[var(--radius-base)] border border-[var(--color-border)] px-3 py-1.5 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] transition hover:border-[var(--color-danger)] hover:text-[var(--color-danger)] cursor-pointer"),
					dom.Text("Disconnect")),
			),
		)
	case pd.GoogleEnabled:
		googleRow = dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-4 last:border-0"),
			dom.Div(dom.Class("min-w-0"),
				dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text("Google")),
				dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Not connected")),
			),
			dom.A(
				dom.Class("shrink-0 rounded-[var(--radius-base)] border border-[var(--color-border)] px-3 py-1.5 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text)] transition hover:border-[var(--color-primary)] hover:text-[var(--color-primary)] cursor-pointer"),
				dom.Href("/auth/google/login?mode=link"),
				dom.Text("Connect")),
		)
	}

	return card(
		ruledHeading("Login methods"),
		dom.Div(
			dom.Class("mt-2"),
			dom.Div(
				dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-4"),
				dom.Div(dom.Class("min-w-0"),
					dom.Span(dom.Class("font-medium text-[var(--color-text)]"), dom.Text("Email magic link")),
					dom.P(dom.Class("mt-0.5 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"), dom.Text("Always available")),
				),
			),
			googleRow,
		),
	)
}
```

Change `Security` to accept `googleLinked bool` and append the new card after the existing sessions card:

```go
func Security(pd config.PageData, meta config.Meta, av AccountView, sessions []database.Session, currentID string, googleLinked bool) dom.Node {
	// ... existing row/body construction unchanged ...

	return accountShell(pd, meta, av,
		card(body...),
		loginMethodsCard(pd, av, googleLinked),
	)
}
```

(The existing `Security` body builds a `card(body...)`; wrap it and the new `loginMethodsCard` as the two `accountShell` children. Keep the sessions logic exactly as-is.)

- [ ] **Step 5: Wire `handlers_account.go`**

Update `accountView` to read flash/error query params:

```go
func accountView(r *http.Request, active string) views.AccountView {
	sess, _ := auth.SessionFrom(r.Context())
	return views.AccountView{
		Active: active,
		CSRF:   auth.CSRFToken(Conf.AppSecret, sess.ID),
		Flash:  r.URL.Query().Get("flash"),
		Error:  r.URL.Query().Get("err"),
	}
}
```

Update the `HandleSecurity` render call to pass `googleLinked`:

```go
		return render(w, http.StatusOK, views.Security(Conf.PageDataFor(account(r)), meta, av, sessions, sess.ID, user.GoogleID != ""))
```

- [ ] **Step 6: Run tests + build**

Run: `go build ./... && go test ./views/ ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add views/auth.go views/account.go views/auth_test.go views/account_test.go handlers_account.go
git commit -m "feat(views): Google login button, Security login-methods, flash/error alerts"
```

---

### Task 5: Handlers — Google login + callback (sign-in & link) + main wiring

**Files:**
- Modify: `handlers_auth.go`, `main.go`
- Test: `handlers_auth_test.go` (append)

**Interfaces:**
- Consumes: `database.Store`, `auth.GoogleOAuth`, `auth.OAuthState`, `auth.GoogleUser`, `auth.NewVerifier`, `auth.SetStateCookie`/`ReadStateCookie`/`ClearStateCookie`, `auth.GenerateToken`, `auth.SetSessionCookie`, `auth.SessionTTL`, existing `account`, `render`, `clientIP`.
- Produces:
  - `func googleSignIn(ctx context.Context, store *database.Store, gu auth.GoogleUser) (database.User, error)`
  - `func googleLink(ctx context.Context, store *database.Store, current database.User, gu auth.GoogleUser) error`
  - `func HandleGoogleLogin(oauth *auth.GoogleOAuth) http.Handler`
  - `func HandleGoogleCallback(store *database.Store, oauth *auth.GoogleOAuth) http.Handler`
  - `main.go` registers the two routes when `Conf.GoogleEnabled()`.

- [ ] **Step 1: Write the failing test**

```go
// handlers_auth_test.go  (append — app/auth, context, net/http,
// net/http/httptest, strings are already imported by this file)

func TestGoogleSignIn_CreateLinkAndVerify(t *testing.T) {
	store := authTestStore(t)
	ctx := context.Background()

	// New user is created with google_id and name backfilled.
	u, err := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s1", Email: "new@example.com", EmailVerified: true, Name: "Neo"})
	if err != nil {
		t.Fatalf("googleSignIn(new): %v", err)
	}
	if u.GoogleID != "s1" || u.Name != "Neo" {
		t.Errorf("created user = %+v", u)
	}

	// Same google_id returns the same user.
	again, _ := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s1", Email: "new@example.com", EmailVerified: true})
	if again.ID != u.ID {
		t.Errorf("second sign-in id = %d, want %d", again.ID, u.ID)
	}

	// Existing email (magic-link user) gets linked.
	existing, _ := store.CreateUser(ctx, "old@example.com", "Old")
	linked, err := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s2", Email: "old@example.com", EmailVerified: true})
	if err != nil || linked.ID != existing.ID || linked.GoogleID != "s2" {
		t.Errorf("link-by-email = %+v, err %v", linked, err)
	}

	// Unverified email is rejected.
	if _, err := googleSignIn(ctx, store, auth.GoogleUser{Sub: "s3", Email: "x@example.com", EmailVerified: false}); err == nil {
		t.Error("expected rejection for unverified email")
	}
}

func TestGoogleLink_RejectsAlreadyLinked(t *testing.T) {
	store := authTestStore(t)
	ctx := context.Background()
	a, _ := store.CreateUser(ctx, "a@example.com", "A")
	_ = store.SetUserGoogleID(ctx, a.ID, "shared")
	b, _ := store.CreateUser(ctx, "b@example.com", "B")

	if err := googleLink(ctx, store, b, auth.GoogleUser{Sub: "shared", Email: "b@example.com", EmailVerified: true}); err == nil {
		t.Error("expected error linking a google_id already used by another user")
	}
	if err := googleLink(ctx, store, b, auth.GoogleUser{Sub: "fresh", Email: "b@example.com", EmailVerified: true}); err != nil {
		t.Errorf("googleLink(fresh): %v", err)
	}
	got, _ := store.UserByID(ctx, b.ID)
	if got.GoogleID != "fresh" {
		t.Errorf("b.GoogleID = %q, want fresh", got.GoogleID)
	}
}

func TestGoogleCallback_RejectsBadState(t *testing.T) {
	store := authTestStore(t)
	oauth := auth.NewGoogleOAuth("cid", "csec", "http://localhost/cb")
	// No state cookie at all → redirect to /login, no network call.
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=x&state=y", nil)
	rec := httptest.NewRecorder()
	HandleGoogleCallback(store, oauth).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("bad-state callback = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestGoogleLogin_RedirectsToGoogle(t *testing.T) {
	oauth := auth.NewGoogleOAuth("cid", "csec", "http://localhost/cb")
	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	rec := httptest.NewRecorder()
	HandleGoogleLogin(oauth).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "https://accounts.google.com/") {
		t.Errorf("Location = %q", loc)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Error("expected a state cookie to be set")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'TestGoogleSignIn|TestGoogleLink|TestGoogleCallback|TestGoogleLogin' -v`
Expected: FAIL — `undefined: googleSignIn` / `undefined: HandleGoogleLogin`.

- [ ] **Step 3: Write the handlers**

Append to `handlers_auth.go`. Add imports `"database/sql"`, `"errors"`, `"fmt"` to the file's import block.

```go
// googleSignIn resolves a Google identity to a user: by google_id, else by
// verified email (linking it), else by creating the user. Unverified emails
// are rejected.
func googleSignIn(ctx context.Context, store *database.Store, gu auth.GoogleUser) (database.User, error) {
	if !gu.EmailVerified {
		return database.User{}, fmt.Errorf("google email not verified")
	}
	if u, err := store.UserByGoogleID(ctx, gu.Sub); err == nil {
		return u, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return database.User{}, err
	}

	u, err := store.UserByEmail(ctx, gu.Email)
	if err == nil {
		if err := store.SetUserGoogleID(ctx, u.ID, gu.Sub); err != nil {
			return database.User{}, err
		}
		if u.Name == "" && gu.Name != "" {
			_ = store.UpdateUserName(ctx, u.ID, gu.Name)
		}
		return store.UserByID(ctx, u.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return database.User{}, err
	}

	u, err = store.CreateUser(ctx, gu.Email, gu.Name)
	if err != nil {
		return database.User{}, err
	}
	if err := store.SetUserGoogleID(ctx, u.ID, gu.Sub); err != nil {
		return database.User{}, err
	}
	u.GoogleID = gu.Sub
	return u, nil
}

// googleLink attaches a Google identity to the current user, rejecting a
// google_id already linked to a different account.
func googleLink(ctx context.Context, store *database.Store, current database.User, gu auth.GoogleUser) error {
	if !gu.EmailVerified {
		return fmt.Errorf("google email not verified")
	}
	if existing, err := store.UserByGoogleID(ctx, gu.Sub); err == nil {
		if existing.ID != current.ID {
			return fmt.Errorf("That Google account is already linked to another user")
		}
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err := store.SetUserGoogleID(ctx, current.ID, gu.Sub); err != nil {
		return err
	}
	if current.Name == "" && gu.Name != "" {
		_ = store.UpdateUserName(ctx, current.ID, gu.Name)
	}
	return nil
}

// HandleGoogleLogin starts the OAuth flow: it stores a signed state cookie and
// redirects to Google's consent screen. mode=link (only honored for a
// signed-in user) attaches Google to the current account on callback.
func HandleGoogleLogin(oauth *auth.GoogleOAuth) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		mode := "login"
		if r.URL.Query().Get("mode") == "link" {
			if _, ok := auth.UserFrom(r.Context()); ok {
				mode = "link"
			}
		}
		next := auth.SafeNext(r.URL.Query().Get("next"))
		state, _, err := auth.GenerateToken()
		if err != nil {
			return err
		}
		verifier := auth.NewVerifier()
		auth.SetStateCookie(w, Conf.AppSecret,
			auth.OAuthState{State: state, Next: next, Mode: mode, Verifier: verifier}, !Conf.Debug)
		http.Redirect(w, r, oauth.AuthCodeURL(state, verifier), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

// HandleGoogleCallback completes the OAuth flow: verify state, exchange the
// code, fetch the profile, then either link to the current user or sign in.
func HandleGoogleCallback(store *database.Store, oauth *auth.GoogleOAuth) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		st, ok := auth.ReadStateCookie(r, Conf.AppSecret)
		auth.ClearStateCookie(w, !Conf.Debug)
		if !ok || r.URL.Query().Get("state") != st.State {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return nil
		}

		token, err := oauth.Exchange(r.Context(), r.URL.Query().Get("code"), st.Verifier)
		if err != nil {
			return err
		}
		gu, err := oauth.FetchUser(r.Context(), token)
		if err != nil {
			return err
		}

		// Link mode: attach to the already-signed-in user.
		if st.Mode == "link" {
			if cur, ok := auth.UserFrom(r.Context()); ok {
				if err := googleLink(r.Context(), store, cur, gu); err != nil {
					http.Redirect(w, r, "/account/security?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
					return nil
				}
				http.Redirect(w, r, "/account/security?flash="+url.QueryEscape("Google connected"), http.StatusSeeOther)
				return nil
			}
			// Session vanished mid-flow → fall through and sign in.
		}

		user, err := googleSignIn(r.Context(), store, gu)
		if err != nil {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Sign-in failed")
			return render(w, http.StatusOK, views.VerifyError(Conf.PageDataFor(account(r)), meta))
		}

		sessTok, sessHash, err := auth.GenerateToken()
		if err != nil {
			return err
		}
		if err := store.CreateSession(r.Context(), sessHash, user.ID,
			time.Now().Add(auth.SessionTTL), r.UserAgent(), clientIP(r, Conf.TrustProxy)); err != nil {
			return err
		}
		auth.SetSessionCookie(w, sessTok, !Conf.Debug)
		http.Redirect(w, r, auth.SafeNext(st.Next), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Wire `main.go`**

In `run()`, after the existing magic-link auth routes (`/logout`) and before the account routes, register the Google routes only when enabled:

```go
	// Google OAuth (optional: only when configured)
	if Conf.GoogleEnabled() {
		goauth := auth.NewGoogleOAuth(Conf.GoogleClientID, Conf.GoogleClientSecret, Conf.BaseURL+"/auth/google/callback")
		s.Handle("/auth/google/login", HandleGoogleLogin(goauth))
		s.Handle("/auth/google/callback", HandleGoogleCallback(store, goauth))
	}
```

(`auth` is already imported in `main.go`.)

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && go test ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_auth.go handlers_auth_test.go main.go
git commit -m "feat: Google OAuth login + callback (sign-in, link) with session creation"
```

---

### Task 6: Handler — disconnect Google + main wiring

**Files:**
- Modify: `handlers_account.go`, `main.go`
- Test: `handlers_account_test.go` (append)

**Interfaces:**
- Consumes: `database.Store.ClearUserGoogleID`, `requireCSRF`, `auth.UserFrom`, `auth.RequireUser`, the test helpers `loggedInRequestSession`/`loggedInWith` from sub-project #1.
- Produces:
  - `func HandleDisconnectGoogle(store *database.Store) http.Handler` (POST, CSRF)
  - `main.go` registers `/account/google/disconnect` under `auth.RequireUser`.

- [ ] **Step 1: Write the failing test**

```go
// handlers_account_test.go  (append)

func TestHandleDisconnectGoogle_ClearsLink(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	_ = store.SetUserGoogleID(context.Background(), u.ID, "sub-xyz")
	csrf := auth.CSRFToken(Conf.AppSecret, sess.ID)

	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/google/disconnect", "_csrf="+csrf)
	rec := httptest.NewRecorder()
	HandleDisconnectGoogle(store).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	got, _ := store.UserByID(context.Background(), u.ID)
	if got.GoogleID != "" {
		t.Errorf("GoogleID = %q, want empty after disconnect", got.GoogleID)
	}
}

func TestHandleDisconnectGoogle_BadCSRF(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account/google/disconnect", "_csrf=wrong")
	rec := httptest.NewRecorder()
	HandleDisconnectGoogle(store).ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestHandleDisconnectGoogle -v`
Expected: FAIL — `undefined: HandleDisconnectGoogle`.

- [ ] **Step 3: Write the handler**

Append to `handlers_account.go`. Add `"net/url"` to its imports.

```go
func HandleDisconnectGoogle(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		if err := store.ClearUserGoogleID(r.Context(), user.ID); err != nil {
			return err
		}
		http.Redirect(w, r, "/account/security?flash="+url.QueryEscape("Google disconnected"), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Wire `main.go`**

In the account-routes block (with the other `auth.RequireUser` handlers), add:

```go
	s.Handle("/account/google/disconnect", auth.RequireUser(HandleDisconnectGoogle(store)))
```

- [ ] **Step 5: Run tests + build**

Run: `go build ./... && go test ./...`
Expected: builds clean; all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_account.go handlers_account_test.go main.go
git commit -m "feat: disconnect Google from the account (CSRF-protected)"
```

---

### Task 7: Docs, CSS rebuild, and full verification

**Files:**
- Modify: `README.md`, `Dockerfile`
- Rebuild: `static/css/styles.css`

**Interfaces:**
- Produces: documented Google env + a clean build with the new view classes emitted.

- [ ] **Step 1: Document the env in README**

Add to the "Accounts & auth" env table in `README.md` (below `EMAIL_FROM`):

```markdown
| `GOOGLE_CLIENT_ID` | Google OAuth client id | unset → Google login hidden |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret | unset → Google login hidden |
```

And add a short note under the table:

```markdown
**Google login:** create an OAuth 2.0 Client (type "Web application") in Google
Cloud Console and register the redirect URI `<BASE_URL>/auth/google/callback`
(e.g. `http://localhost:3000/auth/google/callback` in dev). Set both env vars to
enable the "Continue with Google" button; leave them unset and the app falls
back to email magic-link only.
```

- [ ] **Step 2: Document env in the Dockerfile**

Extend the auth/email env comment block in `Dockerfile`:

```dockerfile
# Auth/email (set at runtime): APP_SECRET (required in prod), BASE_URL,
# POSTMARK_TOKEN, EMAIL_FROM, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET.
```

- [ ] **Step 3: Rebuild Tailwind CSS**

The new views added utility classes (e.g. the Google button, `or` divider). Regenerate the committed stylesheet so they are emitted:

```bash
./bin/tailwind --input ./tailwind.input.css --output ./static/css/styles.css --minify
```
Expected: `static/css/styles.css` updates (the `@source` globs already scan `./**/*.go`).

- [ ] **Step 4: Full verification (vet, test, build, optional live flow)**

```bash
go vet ./...
go test ./...
go build -ldflags="-X 'main.BuildEnv=debug'" -o /tmp/gapp .
```
Expected: vet clean, all tests pass, binary builds.

Optional live check (requires real Google creds + a registered redirect URI):
```bash
# With GOOGLE_CLIENT_ID/SECRET and BASE_URL set, run the app and confirm the
# "Continue with Google" button appears on /login. The full consent round-trip
# is manual (Google's screen cannot be automated). Without creds, confirm the
# button is absent and /auth/google/login returns 404.
```

- [ ] **Step 5: Commit**

```bash
git add README.md Dockerfile static/css/styles.css
git commit -m "docs: document Google OAuth env and dev setup; rebuild CSS"
```

---

## Self-Review

**Spec coverage:**
- `google_id` column + partial unique index + store methods → Task 1 ✓
- Google creds config + `GoogleEnabled` + PageData flag → Task 2 ✓
- `x/oauth2` (endpoints inline, no `/google` subpackage), PKCE, signed state cookie, userinfo fetch → Task 3 ✓
- Login button (graceful), Security "Login methods", flash/error alerts → Task 4 ✓
- Login (find/link/create, email_verified reject), callback (login + link modes), session reuse, main wiring (registered only when enabled) → Task 5 ✓
- Connect (`?mode=link`) → Task 5 (login handler) + Task 4 (Connect link); Disconnect (CSRF) → Task 6 ✓
- Reject unverified email → Tasks 5 (`googleSignIn`/`googleLink`) ✓
- Name backfill only, monogram kept → Tasks 5 ✓
- Docs + manual round-trip note → Task 7 ✓
- One new dependency only → Task 3 (inline endpoints avoid `cloud.google.com/go`) ✓

**Placeholder scan:** Every code step contains complete code; every test step has real assertions. No TBD/TODO.

**Type consistency:** `database.User.GoogleID`, `UserByGoogleID`/`SetUserGoogleID`/`ClearUserGoogleID`; `config.GoogleEnabled`/`PageData.GoogleEnabled`/`GoogleClientID`/`GoogleClientSecret`; `auth.GoogleUser`/`OAuthState`/`GoogleOAuth`/`NewGoogleOAuth`/`AuthCodeURL`/`Exchange`/`FetchUser`/`NewVerifier`/`SetStateCookie`/`ReadStateCookie`/`ClearStateCookie`; `views.AccountView.Error`, `views.Security(..., googleLinked bool)`, `views.Login` reading `pd.GoogleEnabled`; handlers `googleSignIn`/`googleLink`/`HandleGoogleLogin`/`HandleGoogleCallback`/`HandleDisconnectGoogle` — used consistently across tasks.

**Notes for the implementer:**
- Task 4 changes `views.Security`'s arity; the existing `TestSecurity_ListsSessions` from sub-project #1 must be updated to pass the new trailing `false` (called out in Task 4 Step 1).
- `oauth2.GenerateVerifier`/`S256ChallengeOption`/`VerifierOption` require a reasonably recent `x/oauth2` (≥ v0.10). `go get @latest` satisfies this.
- The OAuth `state` cookie HMAC reuses `auth.CSRFToken`/`auth.ValidCSRF` with the base64 payload as the message — no new crypto.
