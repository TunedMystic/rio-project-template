# Accounts — Identity Foundation + Email Magic-Link — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reusable identity foundation — users, server-side sessions, passwordless email magic-link login, and a tabbed account/settings area — to the rio template.

**Architecture:** New `auth/` (crypto + http policy) and `email/` (Postmark/Console sender) packages; the existing `database/` store gains `users`/`sessions`/`login_tokens` access via migration `0002`; views gain auth + account pages; root handlers split into `handlers_auth.go` and `handlers_account.go`. A server-wide `LoadUser` middleware puts the current user in the request context; `RequireUser` guards the account area. Magic-link tokens are single-use, 15-min, hashed at rest; sessions are DB-backed and revocable; cookies are `HttpOnly`/`Secure`/`SameSite=Lax`.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio` v0.26.0 (`rio`, `rio/dom`, `rio/ui`, `rio/forms`), `modernc.org/sqlite`. **No new external dependency** — all stdlib (`crypto/rand`, `crypto/sha256`, `crypto/hmac`, `net/http`).

## Global Constraints

- Module path `app`; internal imports `app/config`, `app/database`, `app/email`, `app/auth`, `app/views`.
- **No new Go dependency** in this sub-project. If `go.mod` somehow changes, re-run `go mod tidy && go mod vendor`.
- Raw SQL over `database/sql`; account access methods hang off the existing `*database.Store` (one store, files split by responsibility).
- Migrations are forward-only, embedded, applied at startup; add `database/migrations/0002_accounts.sql`.
- All new config comes from env: `BASE_URL`, `APP_SECRET`, `POSTMARK_TOKEN`, `EMAIL_FROM` (alongside existing `DB_DIR`/`PORT`/`ADDR`).
- Secrets: `APP_SECRET` has a dev default (with a logged warning); in production (`!Debug`) the app must refuse to start if it is unset.
- Session cookie: name `session`, value is a random token, server stores only `sha256(token)`; `HttpOnly`, `Secure` when `!Debug`, `SameSite=Lax`, `Path=/`, 30-day expiry. Login tokens: 15-min, single-use, `sha256` at rest.
- Email comparison is case-insensitive (column `COLLATE NOCASE`). Expiry checks happen in Go (scan `time.Time`, compare), not in SQL, to avoid driver time-format pitfalls.
- Run all tests with `go test ./...` from the repo root. TDD: failing test first. Commit after each task.

---

### Task 1: Accounts migration + User store

**Files:**
- Create: `database/migrations/0002_accounts.sql`
- Create: `database/users.go`
- Test: `database/users_test.go`

**Interfaces:**
- Consumes: existing `Open`, `MigrateUp`, `Store`, `NewStore` from `database`.
- Produces:
  - `type User struct { ID int64; Email, Name string; CreatedAt time.Time }`
  - `func (s *Store) CreateUser(ctx context.Context, email, name string) (User, error)`
  - `func (s *Store) UserByEmail(ctx context.Context, email string) (User, error)` (returns `sql.ErrNoRows` if absent)
  - `func (s *Store) UserByID(ctx context.Context, id int64) (User, error)`
  - `func (s *Store) UpdateUserName(ctx context.Context, id int64, name string) error`
  - `func (s *Store) DeleteUser(ctx context.Context, id int64) error`

- [ ] **Step 1: Write the migration**

```sql
-- database/migrations/0002_accounts.sql
CREATE TABLE users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    name       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE login_tokens (
    token_hash TEXT PRIMARY KEY,
    email      TEXT NOT NULL COLLATE NOCASE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 2: Write the failing test**

```go
// database/users_test.go
package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return NewStore(db)
}

func TestUsers_CreateLookupUpdateDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "Person@Example.com", "Person")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.ID == 0 || u.CreatedAt.IsZero() {
		t.Fatalf("user not populated: %+v", u)
	}

	// Email lookup is case-insensitive.
	got, err := s.UserByEmail(ctx, "person@example.com")
	if err != nil {
		t.Fatalf("UserByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("UserByEmail id = %d, want %d", got.ID, u.ID)
	}

	// Duplicate email (different case) is rejected by the unique index.
	if _, err := s.CreateUser(ctx, "PERSON@example.com", ""); err == nil {
		t.Error("expected duplicate email error")
	}

	if err := s.UpdateUserName(ctx, u.ID, "Renamed"); err != nil {
		t.Fatalf("UpdateUserName: %v", err)
	}
	got, _ = s.UserByID(ctx, u.ID)
	if got.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", got.Name)
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.UserByID(ctx, u.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("after delete err = %v, want sql.ErrNoRows", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./database/ -run TestUsers -v`
Expected: FAIL — `undefined: (*Store).CreateUser`.

- [ ] **Step 4: Write the implementation**

```go
// database/users.go
package database

import (
	"context"
	"time"
)

// User is an account holder. Email is the case-insensitive identity.
type User struct {
	ID        int64
	Email     string
	Name      string
	CreatedAt time.Time
}

// CreateUser inserts a user and returns it with id and created_at populated.
func (s *Store) CreateUser(ctx context.Context, email, name string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO users (email, name) VALUES (?, ?) RETURNING id, email, name, created_at",
		email, name,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	return u, err
}

// UserByEmail looks up a user by email (case-insensitive via column collation).
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at FROM users WHERE email = ?", email))
}

// UserByID looks up a user by id.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx,
		"SELECT id, email, name, created_at FROM users WHERE id = ?", id))
}

// UpdateUserName sets the display name.
func (s *Store) UpdateUserName(ctx context.Context, id int64, name string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET name = ? WHERE id = ?", name, id)
	return err
}

// DeleteUser removes the user; sessions cascade via the foreign key.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanUser(row rowScanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	return u, err
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./database/ -run TestUsers -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add database/migrations/0002_accounts.sql database/users.go database/users_test.go
git commit -m "feat(db): accounts migration and user store"
```

---

### Task 2: Session store

**Files:**
- Create: `database/sessions.go`
- Test: `database/sessions_test.go`

**Interfaces:**
- Consumes: `Store`, `CreateUser` (Task 1).
- Produces:
  - `type Session struct { ID string; UserID int64; ExpiresAt, CreatedAt time.Time; UserAgent, IP string }`
  - `func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expiresAt time.Time, userAgent, ip string) error`
  - `func (s *Store) SessionByID(ctx context.Context, id string) (Session, error)` (`sql.ErrNoRows` if absent)
  - `func (s *Store) ListUserSessions(ctx context.Context, userID int64) ([]Session, error)` (newest first)
  - `func (s *Store) DeleteSession(ctx context.Context, id string) error`
  - `func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, exceptID string) error`
  - `func (s *Store) DeleteExpiredSessions(ctx context.Context) error`

- [ ] **Step 1: Write the failing test**

```go
// database/sessions_test.go
package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestSessions_LifecycleAndCascade(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, "a@example.com", "")
	exp := time.Now().Add(24 * time.Hour)

	if err := s.CreateSession(ctx, "hash1", u.ID, exp, "agent", "1.2.3.4"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.CreateSession(ctx, "hash2", u.ID, exp, "agent2", "5.6.7.8"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := s.SessionByID(ctx, "hash1")
	if err != nil || got.UserID != u.ID || got.IP != "1.2.3.4" {
		t.Fatalf("SessionByID = %+v, err %v", got, err)
	}

	list, _ := s.ListUserSessions(ctx, u.ID)
	if len(list) != 2 {
		t.Fatalf("ListUserSessions = %d, want 2", len(list))
	}

	// Sign out everywhere except hash1.
	if err := s.DeleteUserSessions(ctx, u.ID, "hash1"); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}
	list, _ = s.ListUserSessions(ctx, u.ID)
	if len(list) != 1 || list[0].ID != "hash1" {
		t.Fatalf("after delete-except = %+v", list)
	}

	// Deleting the user cascades to sessions.
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.SessionByID(ctx, "hash1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("session not cascaded: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./database/ -run TestSessions -v`
Expected: FAIL — `undefined: (*Store).CreateSession`.

- [ ] **Step 3: Write the implementation**

```go
// database/sessions.go
package database

import (
	"context"
	"time"
)

// Session is a server-side login session. ID is sha256(cookie token).
type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
	UserAgent string
	IP        string
}

func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expiresAt time.Time, userAgent, ip string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, user_id, expires_at, user_agent, ip) VALUES (?, ?, ?, ?, ?)",
		id, userID, expiresAt, userAgent, ip)
	return err
}

func (s *Store) SessionByID(ctx context.Context, id string) (Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, expires_at, created_at, user_agent, ip FROM sessions WHERE id = ?", id,
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt, &sess.UserAgent, &sess.IP)
	return sess, err
}

func (s *Store) ListUserSessions(ctx context.Context, userID int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, user_id, expires_at, created_at, user_agent, ip FROM sessions WHERE user_id = ? ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt, &sess.CreatedAt, &sess.UserAgent, &sess.IP); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	return err
}

// DeleteUserSessions deletes all of a user's sessions except exceptID (pass ""
// to delete all).
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64, exceptID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE user_id = ? AND id != ?", userID, exceptID)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now())
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./database/ -run TestSessions -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/sessions.go database/sessions_test.go
git commit -m "feat(db): session store with cascade and sign-out-everywhere"
```

---

### Task 3: Login-token store

**Files:**
- Create: `database/tokens.go`
- Test: `database/tokens_test.go`

**Interfaces:**
- Consumes: `Store`.
- Produces:
  - `type LoginToken struct { Email string; ExpiresAt time.Time }`
  - `func (s *Store) CreateToken(ctx context.Context, tokenHash, email string, expiresAt time.Time) error`
  - `func (s *Store) ConsumeToken(ctx context.Context, tokenHash string) (LoginToken, bool, error)` — atomically returns + deletes; `bool` is whether it existed.
  - `func (s *Store) DeleteExpiredTokens(ctx context.Context) error`

- [ ] **Step 1: Write the failing test**

```go
// database/tokens_test.go
package database

import (
	"context"
	"testing"
	"time"
)

func TestTokens_ConsumeIsSingleUse(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	exp := time.Now().Add(15 * time.Minute)

	if err := s.CreateToken(ctx, "h1", "a@example.com", exp); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	tok, ok, err := s.ConsumeToken(ctx, "h1")
	if err != nil || !ok {
		t.Fatalf("ConsumeToken ok=%v err=%v", ok, err)
	}
	if tok.Email != "a@example.com" {
		t.Errorf("email = %q", tok.Email)
	}

	// Second consume finds nothing (single-use).
	_, ok, err = s.ConsumeToken(ctx, "h1")
	if err != nil {
		t.Fatalf("2nd ConsumeToken err: %v", err)
	}
	if ok {
		t.Error("token was consumable twice")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./database/ -run TestTokens -v`
Expected: FAIL — `undefined: (*Store).CreateToken`.

- [ ] **Step 3: Write the implementation**

```go
// database/tokens.go
package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// LoginToken is a pending magic-link token (the hash is the primary key).
type LoginToken struct {
	Email     string
	ExpiresAt time.Time
}

func (s *Store) CreateToken(ctx context.Context, tokenHash, email string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO login_tokens (token_hash, email, expires_at) VALUES (?, ?, ?)",
		tokenHash, email, expiresAt)
	return err
}

// ConsumeToken atomically reads and deletes the token so it can be used only
// once. ok is false when no such token exists. The caller must still check
// ExpiresAt (expired tokens are deleted here too).
func (s *Store) ConsumeToken(ctx context.Context, tokenHash string) (LoginToken, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginToken{}, false, err
	}
	defer tx.Rollback()

	var tok LoginToken
	err = tx.QueryRowContext(ctx,
		"SELECT email, expires_at FROM login_tokens WHERE token_hash = ?", tokenHash,
	).Scan(&tok.Email, &tok.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LoginToken{}, false, nil
	}
	if err != nil {
		return LoginToken{}, false, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM login_tokens WHERE token_hash = ?", tokenHash); err != nil {
		return LoginToken{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return LoginToken{}, false, err
	}
	return tok, true, nil
}

func (s *Store) DeleteExpiredTokens(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM login_tokens WHERE expires_at < ?", time.Now())
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./database/ -run TestTokens -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/tokens.go database/tokens_test.go
git commit -m "feat(db): single-use login-token store"
```

---

### Task 4: Email sender (Console + Postmark)

**Files:**
- Create: `email/email.go`
- Test: `email/email_test.go`

**Interfaces:**
- Consumes: stdlib only.
- Produces:
  - `type Sender interface { Send(ctx context.Context, to, subject, textBody string) error }`
  - `type Console struct { Log *log.Logger }` implementing `Sender` (writes the message to the log)
  - `type Postmark struct { Token, From, BaseURL string; Client *http.Client }` implementing `Sender`
  - `func New(token, from string) Sender` — returns `Console` when `token == ""`, else a `Postmark` with `BaseURL = "https://api.postmarkapp.com"`.

- [ ] **Step 1: Write the failing test**

```go
// email/email_test.go
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsole_LogsMessage(t *testing.T) {
	var buf bytes.Buffer
	c := Console{Log: log.New(&buf, "", 0)}
	if err := c.Send(context.Background(), "to@example.com", "Subject", "Body link https://x/y"); err != nil {
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
	if err := p.Send(context.Background(), "to@example.com", "Subj", "the body"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotToken != "tok" {
		t.Errorf("token header = %q", gotToken)
	}
	if gotBody["To"] != "to@example.com" || gotBody["From"] != "from@example.com" {
		t.Errorf("body = %+v", gotBody)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./email/ -v`
Expected: FAIL — `undefined: Console`.

- [ ] **Step 3: Write the implementation**

```go
// email/email.go
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Sender delivers a plain-text email.
type Sender interface {
	Send(ctx context.Context, to, subject, textBody string) error
}

// New returns a Postmark sender when a token is set, else a Console sender that
// logs the message (so local dev needs no email account).
func New(token, from string) Sender {
	if token == "" {
		return Console{Log: log.Default()}
	}
	return Postmark{Token: token, From: from, BaseURL: "https://api.postmarkapp.com", Client: http.DefaultClient}
}

// Console logs emails instead of sending them.
type Console struct{ Log *log.Logger }

func (c Console) Send(ctx context.Context, to, subject, textBody string) error {
	c.Log.Printf("[email] to=%s subject=%q\n%s", to, subject, textBody)
	return nil
}

// Postmark sends via the Postmark REST API.
type Postmark struct {
	Token   string
	From    string
	BaseURL string
	Client  *http.Client
}

func (p Postmark) Send(ctx context.Context, to, subject, textBody string) error {
	payload, _ := json.Marshal(map[string]string{
		"From":          p.From,
		"To":            to,
		"Subject":       subject,
		"TextBody":      textBody,
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

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./email/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add email/
git commit -m "feat(email): Sender with Console (dev) and Postmark (prod)"
```

---

### Task 5: Config additions + PageData account fields

**Files:**
- Modify: `config/config.go`
- Test: `config/config_test.go` (append)

**Interfaces:**
- Consumes: existing `Config`, `New`, `PageData`.
- Produces:
  - `Config` gains `BaseURL, AppSecret, PostmarkToken, EmailFrom string`.
  - `PageData` gains `Account Account` where `type Account struct { LoggedIn bool; Name, Email string }`.
  - `func (c Config) PageDataFor(a Account) PageData` — like `PageData()` but with account info for the nav.
  - env helpers `baseURLFromEnv(port string) string`, `appSecretFromEnv(debug bool) string`.

- [ ] **Step 1: Write the failing test**

```go
// config/config_test.go  (append)

func TestNew_LoadsAuthEnv(t *testing.T) {
	t.Setenv("BASE_URL", "https://app.example.com")
	t.Setenv("APP_SECRET", "supersecret")
	t.Setenv("POSTMARK_TOKEN", "pm-tok")
	t.Setenv("EMAIL_FROM", "noreply@example.com")

	c := New("production", "h")
	if c.BaseURL != "https://app.example.com" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.AppSecret != "supersecret" || c.PostmarkToken != "pm-tok" || c.EmailFrom != "noreply@example.com" {
		t.Errorf("auth env not loaded: %+v", c)
	}
}

func TestPageDataFor_CarriesAccount(t *testing.T) {
	c := New("debug", "h")
	pd := c.PageDataFor(Account{LoggedIn: true, Name: "Sam", Email: "sam@example.com"})
	if !pd.Account.LoggedIn || pd.Account.Email != "sam@example.com" {
		t.Errorf("account not carried: %+v", pd.Account)
	}
}

func TestAppSecret_DevDefaults(t *testing.T) {
	t.Setenv("APP_SECRET", "")
	if got := appSecretFromEnv(true); got == "" {
		t.Error("dev APP_SECRET should fall back to a default")
	}
	t.Setenv("APP_SECRET", "")
	if got := appSecretFromEnv(false); got != "" {
		t.Error("prod APP_SECRET should stay empty when unset (caller fails fast)")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run 'TestNew_LoadsAuthEnv|TestPageDataFor|TestAppSecret' -v`
Expected: FAIL — `c.BaseURL undefined` / `undefined: appSecretFromEnv`.

- [ ] **Step 3: Add the fields, env helpers, and PageDataFor**

In `config/config.go`, add `Account` to `PageData`:

```go
// Account is the current-user info the nav needs (empty when logged out).
type Account struct {
	LoggedIn bool
	Name     string
	Email    string
}
```
Add `Account Account` to the `PageData` struct. Add to the `Config` struct: `BaseURL`, `AppSecret`, `PostmarkToken`, `EmailFrom string`. In `New`, after `Addr` is set, populate:

```go
	c.BaseURL = baseURLFromEnv(c.Addr)
	c.AppSecret = appSecretFromEnv(debug)
	c.PostmarkToken = os.Getenv("POSTMARK_TOKEN")
	c.EmailFrom = cmpOr(os.Getenv("EMAIL_FROM"), "noreply@localhost")
```
Add the helpers and `PageDataFor`:

```go
// baseURLFromEnv resolves the absolute base URL for links. BASE_URL wins; else
// http://localhost<addr-port> for dev convenience.
func baseURLFromEnv(addr string) string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return v
	}
	port := strings.TrimPrefix(addr, ":")
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		port = addr[i+1:]
	}
	return "http://localhost:" + port
}

// appSecretFromEnv returns APP_SECRET. In dev it falls back to a known default
// (with a warning); in prod it returns "" so the caller can fail fast.
func appSecretFromEnv(debug bool) string {
	if v := os.Getenv("APP_SECRET"); v != "" {
		return v
	}
	if debug {
		log.Println("WARNING: APP_SECRET unset; using an insecure dev default")
		return "dev-only-insecure-secret-change-me"
	}
	return ""
}

func cmpOr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

// PageDataFor returns view data including the current-user account info.
func (c Config) PageDataFor(a Account) PageData {
	pd := c.PageData()
	pd.Account = a
	return pd
}
```
Add `"log"` and `"strings"` to the `config` imports (`os`, `path/filepath` already present).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -v`
Expected: PASS (all config tests).

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "feat(config): auth env (BASE_URL/APP_SECRET/POSTMARK/EMAIL_FROM) and PageData account"
```

---

### Task 6: auth — tokens & next-path validation

**Files:**
- Create: `auth/token.go`
- Test: `auth/token_test.go`

**Interfaces:**
- Consumes: stdlib.
- Produces:
  - `func GenerateToken() (token, hash string, err error)` — 32 random bytes → base64url token; `hash = sha256hex(token)`.
  - `func HashToken(token string) string` — `sha256hex`.
  - `func SafeNext(next string) string` — returns `next` if it's a local path (`/...`, not `//`, no backslash), else `"/account"`.

- [ ] **Step 1: Write the failing test**

```go
// auth/token_test.go
package auth

import "testing"

func TestGenerateAndHashToken(t *testing.T) {
	tok, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if tok == "" || hash == "" || tok == hash {
		t.Fatalf("bad token/hash: %q %q", tok, hash)
	}
	if HashToken(tok) != hash {
		t.Error("HashToken does not match GenerateToken hash")
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"/account/security": "/account/security",
		"":                  "/account",
		"//evil.com":        "/account",
		"https://evil.com":  "/account",
		"/\\evil":           "/account",
		"relative":          "/account",
	}
	for in, want := range cases {
		if got := SafeNext(in); got != want {
			t.Errorf("SafeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./auth/ -run 'TestGenerate|TestSafeNext' -v`
Expected: FAIL — `undefined: GenerateToken`.

- [ ] **Step 3: Write the implementation**

```go
// auth/token.go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// GenerateToken returns a URL-safe random token and its sha256 hex hash. Store
// the hash; put the token in the link/cookie.
func GenerateToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the sha256 hex of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SafeNext returns next only if it is a local absolute path, else "/account".
// Guards against open redirects.
func SafeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.Contains(next, "\\") {
		return next
	}
	return "/account"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./auth/ -run 'TestGenerate|TestSafeNext' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add auth/token.go auth/token_test.go
git commit -m "feat(auth): token generation/hashing and safe-next validation"
```

---

### Task 7: auth — session cookie helpers

**Files:**
- Create: `auth/session.go`
- Test: `auth/session_test.go`

**Interfaces:**
- Consumes: `GenerateToken`, `HashToken` (Task 6); stdlib `net/http`.
- Produces:
  - `const CookieName = "session"`
  - `const SessionTTL = 30 * 24 * time.Hour`
  - `func SetSessionCookie(w http.ResponseWriter, token string, secure bool)`
  - `func ClearSessionCookie(w http.ResponseWriter, secure bool)`
  - `func SessionToken(r *http.Request) string` — cookie value or "".

- [ ] **Step 1: Write the failing test**

```go
// auth/session_test.go
package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionCookieRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "tok123", true)

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies", len(cookies))
	}
	c := cookies[0]
	if c.Name != CookieName || c.Value != "tok123" {
		t.Errorf("cookie = %+v", c)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("insecure cookie flags: %+v", c)
	}

	// Reading it back.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	if got := SessionToken(req); got != "tok123" {
		t.Errorf("SessionToken = %q", got)
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, false)
	c := rec.Result().Cookies()[0]
	if c.MaxAge >= 0 {
		t.Errorf("clear cookie MaxAge = %d, want < 0", c.MaxAge)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./auth/ -run TestSession -v` / `TestClear`
Expected: FAIL — `undefined: SetSessionCookie`.

- [ ] **Step 3: Write the implementation**

```go
// auth/session.go
package auth

import (
	"net/http"
	"time"
)

const (
	CookieName = "session"
	SessionTTL = 30 * 24 * time.Hour
)

func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(SessionTTL),
		MaxAge:   int(SessionTTL.Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func SessionToken(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./auth/ -run 'TestSession|TestClear' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add auth/session.go auth/session_test.go
git commit -m "feat(auth): secure session cookie helpers"
```

---

### Task 8: auth — CSRF tokens + rate limiter

**Files:**
- Create: `auth/csrf.go`, `auth/ratelimit.go`
- Test: `auth/csrf_test.go`, `auth/ratelimit_test.go`

**Interfaces:**
- Produces:
  - `func CSRFToken(secret, sessionID string) string` — hex HMAC-SHA256.
  - `func ValidCSRF(secret, sessionID, token string) bool` — constant-time compare.
  - `type Limiter struct { ... }`, `func NewLimiter(max int, window time.Duration) *Limiter`, `func (l *Limiter) Allow(key string) bool`.

- [ ] **Step 1: Write the failing tests**

```go
// auth/csrf_test.go
package auth

import "testing"

func TestCSRF(t *testing.T) {
	tok := CSRFToken("secret", "sess1")
	if tok == "" {
		t.Fatal("empty token")
	}
	if !ValidCSRF("secret", "sess1", tok) {
		t.Error("valid token rejected")
	}
	if ValidCSRF("secret", "sess1", "wrong") {
		t.Error("wrong token accepted")
	}
	if ValidCSRF("secret", "other-session", tok) {
		t.Error("token accepted for a different session")
	}
}
```

```go
// auth/ratelimit_test.go
package auth

import (
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	if !l.Allow("k") || !l.Allow("k") {
		t.Fatal("first two should be allowed")
	}
	if l.Allow("k") {
		t.Error("third should be denied")
	}
	if !l.Allow("other") {
		t.Error("different key should be allowed")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./auth/ -run 'TestCSRF|TestLimiter' -v`
Expected: FAIL — `undefined: CSRFToken`.

- [ ] **Step 3: Write the implementations**

```go
// auth/csrf.go
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// CSRFToken derives a per-session token from the app secret. No storage needed.
func CSRFToken(secret, sessionID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidCSRF compares a submitted token against the expected one in constant time.
func ValidCSRF(secret, sessionID, token string) bool {
	expected := CSRFToken(secret, sessionID)
	return hmac.Equal([]byte(expected), []byte(token))
}
```

```go
// auth/ratelimit.go
package auth

import (
	"sync"
	"time"
)

// Limiter is a simple in-memory fixed-window limiter (single-instance only).
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	max    int
	window time.Duration
}

func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{hits: map[string][]time.Time{}, max: max, window: window}
}

// Allow records an attempt for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./auth/ -run 'TestCSRF|TestLimiter' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add auth/csrf.go auth/csrf_test.go auth/ratelimit.go auth/ratelimit_test.go
git commit -m "feat(auth): CSRF tokens and in-memory rate limiter"
```

---

### Task 9: auth — middleware (LoadUser / RequireUser)

**Files:**
- Create: `auth/middleware.go`
- Test: `auth/middleware_test.go`

**Interfaces:**
- Consumes: `database.Store`, `database.User`, `database.Session`; `SessionToken`, `HashToken`.
- Produces:
  - `type ctxKey int` (unexported) + `func UserFrom(ctx context.Context) (database.User, bool)` and `func SessionFrom(ctx context.Context) (database.Session, bool)`.
  - `func LoadUser(store *database.Store) func(http.Handler) http.Handler` — reads cookie, hashes, loads non-expired session + user into context.
  - `func RequireUser(next http.Handler) http.Handler` — redirects to `/login?next=<path>` when no user.

- [ ] **Step 1: Write the failing test**

```go
// auth/middleware_test.go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"app/database"
)

func storeWithUser(t *testing.T) (*database.Store, database.User) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatal(err)
	}
	s := database.NewStore(db)
	u, _ := s.CreateUser(context.Background(), "u@example.com", "U")
	return s, u
}

func TestLoadUser_PopulatesContext(t *testing.T) {
	s, u := storeWithUser(t)
	token, hash, _ := GenerateToken()
	_ = s.CreateSession(context.Background(), hash, u.ID, time.Now().Add(time.Hour), "", "")

	var seen database.User
	var ok bool
	h := LoadUser(s)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, ok = UserFrom(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: token})
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !ok || seen.ID != u.ID {
		t.Fatalf("user not loaded: ok=%v seen=%+v", ok, seen)
	}
}

func TestRequireUser_RedirectsAnon(t *testing.T) {
	rec := httptest.NewRecorder()
	guarded := RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not run for anon")
	}))
	guarded.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/account", nil))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?next=%2Faccount" {
		t.Errorf("Location = %q", loc)
	}
}

var _ = context.Background
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./auth/ -run 'TestLoadUser|TestRequireUser' -v`
Expected: FAIL — `undefined: LoadUser`.

- [ ] **Step 3: Write the implementation**

```go
// auth/middleware.go
package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"app/database"
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

// UserFrom returns the authenticated user from the request context.
func UserFrom(ctx context.Context) (database.User, bool) {
	u, ok := ctx.Value(userKey).(database.User)
	return u, ok
}

// SessionFrom returns the current session from the request context.
func SessionFrom(ctx context.Context) (database.Session, bool) {
	s, ok := ctx.Value(sessionKey).(database.Session)
	return s, ok
}

// LoadUser loads the current user/session into the request context when the
// session cookie resolves to a live (non-expired) session. Otherwise it passes
// through unauthenticated.
func LoadUser(store *database.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := SessionToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := store.SessionByID(r.Context(), HashToken(token))
			if err != nil || sess.ExpiresAt.Before(time.Now()) {
				next.ServeHTTP(w, r)
				return
			}
			user, err := store.UserByID(r.Context(), sess.UserID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), userKey, user)
			ctx = context.WithValue(ctx, sessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireUser redirects to /login (preserving the destination) when there is no
// authenticated user.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./auth/ -v`
Expected: PASS (all auth tests).

- [ ] **Step 5: Commit**

```bash
git add auth/middleware.go auth/middleware_test.go
git commit -m "feat(auth): LoadUser/RequireUser context middleware"
```

---

### Task 10: views — auth pages + nav account state

**Files:**
- Create: `views/auth.go`
- Modify: `views/components.go` (nav: login link vs. account avatar)
- Test: `views/auth_test.go`

**Interfaces:**
- Consumes: `config.PageData`, `config.Meta`, existing helpers `Page`, `shell`, `card`, `ruledHeading`, `submitButton`, `ui.*`, `dom.*`.
- Produces:
  - `func Login(pd config.PageData, meta config.Meta, email, errMsg, next string) dom.Node`
  - `func LoginSent(pd config.PageData, meta config.Meta, email string) dom.Node`
  - `func VerifyError(pd config.PageData, meta config.Meta) dom.Node`

- [ ] **Step 1: Write the failing test**

```go
// views/auth_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
)

func TestLogin_RendersForm(t *testing.T) {
	pd := testPageData()
	var b bytes.Buffer
	_ = Login(pd, config.Meta{Title: "Log in"}, "you@example.com", "bad email", "/account/security").Render(&b)
	html := b.String()
	for _, want := range []string{
		`action="/login"`,
		`name="email"`,
		`value="you@example.com"`, // preserves entered email
		"bad email",               // shows the error
		`name="next"`,             // carries next through
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Login missing %q", want)
		}
	}
}

func TestNav_ShowsLoginWhenAnon(t *testing.T) {
	pd := testPageData() // Account.LoggedIn == false
	var b bytes.Buffer
	_ = Page(pd, config.Meta{Title: "x"}, nil).Render(&b)
	if !strings.Contains(b.String(), `href="/login"`) {
		t.Error("anon nav should show a login link")
	}
}

func TestNav_ShowsAccountWhenLoggedIn(t *testing.T) {
	c := config.New("debug", "h")
	pd := c.PageDataFor(config.Account{LoggedIn: true, Name: "Sam", Email: "sam@example.com"})
	var b bytes.Buffer
	_ = Page(pd, config.Meta{Title: "x"}, nil).Render(&b)
	if !strings.Contains(b.String(), `href="/account"`) {
		t.Error("logged-in nav should link to /account")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run 'TestLogin|TestNav' -v`
Expected: FAIL — `undefined: Login`.

- [ ] **Step 3: Update the nav to reflect account state**

In `views/components.go`, replace the body of `navbar` so the right side depends on `pd.Account`:

```go
func navbar(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.HeaderLinks)+2)
	links = append(links, dom.Class("flex items-center gap-6"))
	for _, l := range pd.HeaderLinks {
		links = append(links, navLink(l))
	}
	if pd.Account.LoggedIn {
		links = append(links, accountAvatar(pd.Account))
	} else {
		links = append(links, navLink(config.Link{Text: "Log in", Href: "/login"}))
	}
	return dom.Header(
		dom.Class("border-b border-[var(--color-border)] bg-[#f8f5ee]"),
		dom.Div(
			dom.Class("mx-auto flex w-full max-w-5xl items-center justify-between px-5 py-4"),
			brand(pd),
			dom.Nav(links...),
		),
	)
}

// accountAvatar links to the account area with a monogram of the user.
func accountAvatar(a config.Account) dom.Node {
	label := a.Email
	if a.Name != "" {
		label = a.Name
	}
	return dom.A(
		dom.Class("flex h-8 w-8 items-center justify-center rounded-full bg-[var(--color-primary)] text-[var(--color-on-primary)] text-[length:var(--font-size-sm)] font-bold"),
		dom.Href("/account"),
		dom.Title(label),
		dom.Text(initial(label)),
	)
}
```
(`dom.Title` renders the `title` attribute. If your dom version lacks it, use `dom.Aria("label", label)` instead.)

- [ ] **Step 4: Write the auth pages**

```go
// views/auth.go
package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// authCard centers a narrow card for the auth screens.
func authCard(children ...dom.Node) dom.Node {
	return dom.Section(
		dom.Class("py-16"),
		dom.Div(
			dom.Class("mx-auto w-full max-w-md px-5"),
			card(children...),
		),
	)
}

func Login(pd config.PageData, meta config.Meta, email, errMsg, next string) dom.Node {
	return Page(pd, meta,
		authCard(
			ruledHeading("Log in"),
			dom.P(
				dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("Enter your email and we'll send you a magic link."),
			),
			dom.Form(
				dom.Method("post"),
				dom.Action("/login"),
				dom.Class("mt-6"),
				dom.Input(dom.Type("hidden"), dom.Name("next"), dom.Value(next)),
				ui.TextField("email", "Email address", email, errMsg),
				submitButton("Send magic link"),
			),
		),
	)
}

func LoginSent(pd config.PageData, meta config.Meta, email string) dom.Node {
	return Page(pd, meta,
		authCard(
			ruledHeading("Check your email"),
			dom.P(
				dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("If an account exists for "+email+", a magic link is on its way. The link expires in 15 minutes."),
			),
			dom.Div(
				dom.Class("mt-6"),
				ghostLink("/login", "Use a different email"),
			),
		),
	)
}

func VerifyError(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		authCard(
			ruledHeading("Link expired"),
			dom.P(
				dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("That magic link is invalid or has already been used. Request a fresh one."),
			),
			dom.Div(
				dom.Class("mt-6"),
				ui.ButtonLink(ui.ButtonPrimary, "/login", "Back to log in"),
			),
		),
	)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./views/ -v`
Expected: PASS (all views tests).

- [ ] **Step 6: Commit**

```bash
git add views/auth.go views/auth_test.go views/components.go
git commit -m "feat(views): auth pages and account-aware nav"
```

---

### Task 11: views — account area (tabs + flash)

**Files:**
- Create: `views/account.go`
- Test: `views/account_test.go`

**Interfaces:**
- Consumes: `config.PageData`, `config.Meta`, `database.Session`, helpers, `ui.*`, `dom.*`.
- Produces:
  - `type AccountView struct { Active string; CSRF string; Flash string }` (Active is the tab key: "profile"/"security"/"billing"/"danger")
  - `func Profile(pd config.PageData, meta config.Meta, av AccountView, name, email string) dom.Node`
  - `func Security(pd config.PageData, meta config.Meta, av AccountView, sessions []database.Session, currentID string) dom.Node`
  - `func Billing(pd config.PageData, meta config.Meta, av AccountView) dom.Node`
  - `func Danger(pd config.PageData, meta config.Meta, av AccountView, email string) dom.Node`

- [ ] **Step 1: Write the failing test**

```go
// views/account_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func TestProfile_RendersTabsAndForm(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "profile", CSRF: "csrf-token", Flash: "Saved"}
	var b bytes.Buffer
	_ = Profile(pd, config.Meta{Title: "Profile"}, av, "Sam", "sam@example.com").Render(&b)
	html := b.String()
	for _, want := range []string{
		`href="/account"`,          // profile tab
		`href="/account/security"`, // security tab
		`href="/account/billing"`,  // billing tab
		`href="/account/delete"`,   // danger tab
		`value="csrf-token"`,       // hidden CSRF
		`value="Sam"`,              // editable name
		"sam@example.com",          // email shown
		"Saved",                    // flash
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Profile missing %q", want)
		}
	}
}

func TestSecurity_ListsSessions(t *testing.T) {
	pd := testPageData()
	av := AccountView{Active: "security", CSRF: "c"}
	sessions := []database.Session{{ID: "cur", IP: "1.1.1.1"}, {ID: "other", IP: "2.2.2.2"}}
	var b bytes.Buffer
	_ = Security(pd, config.Meta{Title: "Security"}, av, sessions, "cur").Render(&b)
	html := b.String()
	if !strings.Contains(html, "This device") {
		t.Error("current session not marked")
	}
	if !strings.Contains(html, `action="/account/sessions/revoke-all"`) {
		t.Error("missing sign-out-everywhere form")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -run 'TestProfile|TestSecurity' -v`
Expected: FAIL — `undefined: Profile`.

- [ ] **Step 3: Write the account views**

```go
// views/account.go
package views

import (
	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// AccountView is the per-request chrome state for the account area.
type AccountView struct {
	Active string // "profile" | "security" | "billing" | "danger"
	CSRF   string
	Flash  string
}

type accountTab struct{ key, label, href string }

var accountTabs = []accountTab{
	{"profile", "Profile", "/account"},
	{"security", "Security", "/account/security"},
	{"billing", "Billing", "/account/billing"},
	{"danger", "Danger", "/account/delete"},
}

// accountShell wraps a tab's content with the page header, flash, and tab nav.
func accountShell(pd config.PageData, meta config.Meta, av AccountView, body ...dom.Node) dom.Node {
	tabs := make([]dom.Node, 0, len(accountTabs)+1)
	tabs = append(tabs, dom.Class("flex gap-2 border-b border-[var(--color-border)]"))
	for _, tb := range accountTabs {
		cls := "px-4 py-2 text-[length:var(--font-size-sm)] font-medium text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
		if tb.key == av.Active {
			cls = "px-4 py-2 text-[length:var(--font-size-sm)] font-semibold text-[var(--color-primary)] border-b-2 border-[var(--color-primary)] -mb-px"
		}
		tabs = append(tabs, dom.A(dom.Class(cls), dom.Href(tb.href), dom.Text(tb.label)))
	}

	content := make([]dom.Node, 0, len(body)+2)
	if av.Flash != "" {
		content = append(content, ui.Alert(ui.AlertSuccess, dom.Text(av.Flash)))
	}
	content = append(content, dom.Nav(tabs...))
	content = append(content, body...)

	return Page(pd, meta,
		pageHeader("Account", "Manage your profile, security, and billing."),
		dom.Section(dom.Class("py-12"), shell(dom.Div(withClass("max-w-2xl space-y-6", content)...))),
	)
}

func csrfInput(token string) dom.Node {
	return dom.Input(dom.Type("hidden"), dom.Name("_csrf"), dom.Value(token))
}

func Profile(pd config.PageData, meta config.Meta, av AccountView, name, email string) dom.Node {
	return accountShell(pd, meta, av,
		card(
			ruledHeading("Profile"),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				ui.TextField("name", "Display name", name, ""),
				dom.Div(
					dom.Class("mb-4"),
					ui.Label("email_display", "Email"),
					dom.P(dom.Class("text-[var(--color-text-muted)]"), dom.Text(email)),
				),
				submitButton("Save changes"),
			),
		),
	)
}

func Security(pd config.PageData, meta config.Meta, av AccountView, sessions []database.Session, currentID string) dom.Node {
	rows := make([]dom.Node, 0, len(sessions))
	for _, s := range sessions {
		meta := s.IP
		if s.UserAgent != "" {
			meta = s.UserAgent + " · " + s.IP
		}
		badge := dom.Node(dom.Text(""))
		if s.ID == currentID {
			badge = ui.Badge(ui.BadgeSuccess, "This device")
		}
		right := dom.Node(dom.Text(""))
		if s.ID != currentID {
			right = dom.Form(
				dom.Method("post"),
				dom.Action("/account/sessions/revoke"),
				csrfInput(av.CSRF),
				dom.Input(dom.Type("hidden"), dom.Name("id"), dom.Value(s.ID)),
				dom.Button(dom.Type("submit"),
					dom.Class("text-[length:var(--font-size-sm)] font-medium text-[var(--color-danger)] hover:underline cursor-pointer"),
					dom.Text("Sign out")),
			)
		}
		rows = append(rows, dom.Div(
			dom.Class("flex items-center justify-between border-b border-[var(--color-border)] py-3"),
			dom.Div(dom.Class("flex items-center gap-3"),
				dom.Span(dom.Class("text-[var(--color-text)]"), dom.Text(meta)), badge),
			right,
		))
	}

	return accountShell(pd, meta, av,
		card(
			ruledHeading("Active sessions"),
			dom.Div(withClass("mt-2", rows)...),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/sessions/revoke-all"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				submitButton("Sign out everywhere else"),
			),
		),
	)
}

func Billing(pd config.PageData, meta config.Meta, av AccountView) dom.Node {
	return accountShell(pd, meta, av,
		card(
			ruledHeading("Billing"),
			dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"), dom.Text("You're on the free plan.")),
			dom.Div(dom.Class("mt-4"),
				dom.Span(
					dom.Class("inline-flex items-center rounded-[var(--radius-base)] border border-[var(--color-border)] px-4 py-2 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
					dom.Text("Manage billing (coming soon)"),
				),
			),
		),
	)
}

func Danger(pd config.PageData, meta config.Meta, av AccountView, email string) dom.Node {
	return accountShell(pd, meta, av,
		card(
			ruledHeading("Delete account"),
			dom.P(dom.Class("mt-4 text-[var(--color-text-muted)]"),
				dom.Text("This permanently deletes your account and all sessions. Type your email to confirm.")),
			dom.Form(
				dom.Method("post"),
				dom.Action("/account/delete"),
				dom.Class("mt-6"),
				csrfInput(av.CSRF),
				ui.TextField("confirm_email", "Confirm your email ("+email+")", "", ""),
				dom.Button(dom.Type("submit"),
					dom.Class("inline-flex items-center justify-center rounded-[var(--radius-base)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold bg-[var(--color-danger)] text-white shadow-sm hover:brightness-105 cursor-pointer"),
					dom.Text("Delete my account")),
			),
		),
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./views/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add views/account.go views/account_test.go
git commit -m "feat(views): account area tabs (profile/security/billing/danger)"
```

---

### Task 12: handlers — auth routes (login/verify/logout) + main wiring

**Files:**
- Create: `handlers_auth.go`
- Modify: `main.go` (email sender, `LoadUser` middleware, prod-secret fail-fast, routes)
- Test: `handlers_auth_test.go`

**Interfaces:**
- Consumes: `database.Store`, `email.Sender`, `auth.*`, `views.*`, `config`.
- Produces:
  - `func HandleLogin(store *database.Store, sender email.Sender, limiter *auth.Limiter) http.Handler`
  - `func HandleVerify(store *database.Store) http.Handler`
  - `func HandleLogout(store *database.Store) http.Handler`
  - `main.go` builds these and registers `/login`, `/login/sent`, `/auth/verify`, `/logout`; adds `s.Use(auth.LoadUser(store))`; selects the email sender; fails fast if `!Conf.Debug && Conf.AppSecret == ""`.

- [ ] **Step 1: Write the failing test**

```go
// handlers_auth_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/auth"
	"app/database"
	"app/email"
)

type fakeSender struct{ lastBody string }

func (f *fakeSender) Send(ctx context.Context, to, subject, textBody string) error {
	f.lastBody = textBody
	return nil
}

func authTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatal(err)
	}
	return database.NewStore(db)
}

func TestHandleLogin_POST_IssuesAndSends(t *testing.T) {
	store := authTestStore(t)
	sender := &fakeSender{}
	h := HandleLogin(store, sender, auth.NewLimiter(5, time.Minute))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("email=new@example.com&next=/account"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login/sent?email=new%40example.com" {
		t.Fatalf("status=%d loc=%q", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(sender.lastBody, "/auth/verify?token=") {
		t.Errorf("sent body missing verify link: %q", sender.lastBody)
	}
}

func TestHandleVerify_CreatesUserAndSession(t *testing.T) {
	store := authTestStore(t)

	// Seed a token the way the login handler would.
	token, hash, _ := auth.GenerateToken()
	_ = store.CreateToken(context.Background(), hash, "new@example.com", time.Now().Add(time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/auth/verify?token="+token, nil)
	rec := httptest.NewRecorder()
	HandleVerify(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("no session cookie set")
	}
	if _, err := store.UserByEmail(context.Background(), "new@example.com"); err != nil {
		t.Errorf("user not created: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'TestHandleLogin_POST|TestHandleVerify' -v`
Expected: FAIL — `undefined: HandleLogin`.

- [ ] **Step 3: Write the auth handlers**

```go
// handlers_auth.go
package main

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"app/auth"
	"app/database"
	"app/email"
	"app/views"

	"github.com/tunedmystic/rio"
	"github.com/tunedmystic/rio/forms"
)

const loginTokenTTL = 15 * time.Minute

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, _ := strings.Cut(r.RemoteAddr, ":")
	return host
}

func HandleLogin(store *database.Store, sender email.Sender, limiter *auth.Limiter) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		next := auth.SafeNext(r.URL.Query().Get("next"))
		if r.Method != http.MethodPost {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Log in")
			return render(w, http.StatusOK, views.Login(Conf.PageDataFor(account(r)), meta, "", "", next))
		}

		next = auth.SafeNext(r.FormValue("next"))
		emailAddr := strings.TrimSpace(r.FormValue("email"))
		form := forms.New()
		form.CleanString("email", emailAddr, forms.StrRequired(), forms.StrEmail())
		if !form.IsValid() {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Log in")
			field := form.MustField("email")
			return render(w, http.StatusUnprocessableEntity,
				views.Login(Conf.PageDataFor(account(r)), meta, field.Value(), field.Err().Error(), next))
		}

		// Rate-limit, then (best-effort) issue + send. Always show the same page.
		if limiter.Allow(emailAddr + "|" + clientIP(r)) {
			if token, hash, err := auth.GenerateToken(); err == nil {
				if err := store.CreateToken(r.Context(), hash, emailAddr, time.Now().Add(loginTokenTTL)); err == nil {
					link := Conf.BaseURL + "/auth/verify?token=" + token
					if next != "/account" {
						link += "&next=" + url.QueryEscape(next)
					}
					body := "Click to log in (expires in 15 minutes):\n\n" + link
					if err := sender.Send(r.Context(), emailAddr, "Your login link", body); err != nil {
						rio.LogError("send login email", err)
					}
				}
			}
		}
		http.Redirect(w, r, "/login/sent?email="+url.QueryEscape(emailAddr), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleLoginSent() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Check your email")
		return render(w, http.StatusOK,
			views.LoginSent(Conf.PageDataFor(account(r)), meta, r.URL.Query().Get("email")))
	}
	return rio.MakeHandler(fn)
}

func HandleVerify(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		token := r.URL.Query().Get("token")
		tok, ok, err := store.ConsumeToken(r.Context(), auth.HashToken(token))
		if err != nil {
			return err
		}
		if !ok || tok.ExpiresAt.Before(time.Now()) {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Link expired")
			return render(w, http.StatusOK, views.VerifyError(Conf.PageDataFor(account(r)), meta))
		}

		// Find-or-create the user (unified signup).
		user, err := store.UserByEmail(r.Context(), tok.Email)
		if err != nil {
			if user, err = store.CreateUser(r.Context(), tok.Email, ""); err != nil {
				return err
			}
		}

		// Create the session.
		sessTok, sessHash, err := auth.GenerateToken()
		if err != nil {
			return err
		}
		if err := store.CreateSession(r.Context(), sessHash, user.ID,
			time.Now().Add(auth.SessionTTL), r.UserAgent(), clientIP(r)); err != nil {
			return err
		}
		auth.SetSessionCookie(w, sessTok, !Conf.Debug)

		http.Redirect(w, r, auth.SafeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleLogout(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if sess, ok := auth.SessionFrom(r.Context()); ok {
			_ = store.DeleteSession(r.Context(), sess.ID)
		}
		auth.ClearSessionCookie(w, !Conf.Debug)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

// account builds the nav account view-model from the request context.
func account(r *http.Request) config.Account {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		return config.Account{}
	}
	return config.Account{LoggedIn: true, Name: u.Name, Email: u.Email}
}
```
Add `"app/config"` to this file's imports (used by `account`). `rio.LogError` is rio's error logger; if absent in your version use `log.Printf`.

- [ ] **Step 4: Wire main.go**

Modify `run()` in `main.go`: build the sender + limiter, fail fast on a missing prod secret, register routes, and add `LoadUser` middleware. Replace the server-construction block with:

```go
	if !Conf.Debug && Conf.AppSecret == "" {
		return fmt.Errorf("APP_SECRET must be set in production")
	}

	store := database.NewStore(db)
	sender := email.New(Conf.PostmarkToken, Conf.EmailFrom)
	loginLimiter := auth.NewLimiter(5, 15*time.Minute)

	s := rio.NewServer()
	s.Use(auth.LoadUser(store)) // server-wide: populate the current user

	s.Handle("/", HandleHome())
	s.Handle("/messages", HandleMessages(store))
	s.Handle("/about", HandleAbout())
	s.Handle("/privacy-policy", HandlePrivacyPolicy())
	s.Handle("/version", HandleVersion())
	s.Handle("/healthz", HandleHealth(db))
	s.Handle("/robots.txt", HandleRobots())

	// Auth
	s.Handle("/login", HandleLogin(store, sender, loginLimiter))
	s.Handle("/login/sent", HandleLoginSent())
	s.Handle("/auth/verify", HandleVerify(store))
	s.Handle("/logout", HandleLogout(store))

	s.Handle("/static/", HandleStatic())
```
Add imports to `main.go`: `"time"`, `"app/auth"`, `"app/email"`. (`fmt` is already imported.)

> Note: `s.Use` after `rio.NewServer()` appends to rio's default middleware (LogRequest, RecoverPanic, SecureHeaders), so LoadUser composes with them.

- [ ] **Step 5: Tidy, build, and run the suite**

```bash
go build ./...
go test ./...
```
Expected: builds clean; all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_auth.go handlers_auth_test.go main.go
git commit -m "feat: email magic-link login (login/verify/logout) + LoadUser wiring"
```

---

### Task 13: handlers — account routes (profile/security/delete)

**Files:**
- Create: `handlers_account.go`
- Modify: `main.go` (register protected account routes wrapped in `auth.RequireUser`)
- Test: `handlers_account_test.go`

**Interfaces:**
- Consumes: `database.Store`, `auth.*`, `views.*`, `config`.
- Produces:
  - `func HandleAccount(store *database.Store) http.Handler` — GET profile, POST save name (CSRF-checked).
  - `func HandleSecurity(store *database.Store) http.Handler`
  - `func HandleRevokeSession(store *database.Store) http.Handler` (POST, CSRF)
  - `func HandleRevokeAllSessions(store *database.Store) http.Handler` (POST, CSRF)
  - `func HandleBilling() http.Handler`
  - `func HandleDeleteAccount(store *database.Store) http.Handler` (GET danger page, POST delete; CSRF + email confirm)
  - main registers these under `auth.RequireUser`.

- [ ] **Step 1: Write the failing test**

```go
// handlers_account_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/auth"
	"app/database"
)

// loggedInRequest builds a request whose context carries a real user+session,
// as the LoadUser middleware would.
func loggedInRequest(t *testing.T, store *database.Store, method, target, body string) (*http.Request, database.User) {
	t.Helper()
	u, _ := store.CreateUser(context.Background(), "me@example.com", "Me")
	token, hash, _ := auth.GenerateToken()
	_ = store.CreateSession(context.Background(), hash, u.ID, time.Now().Add(time.Hour), "ua", "ip")

	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	// Run through LoadUser so the context is populated exactly like prod.
	var out *http.Request
	auth.LoadUser(store)(http.HandlerFunc(func(w http.ResponseWriter, rr *http.Request) { out = rr })).ServeHTTP(httptest.NewRecorder(), r)
	return out, u
}

func TestHandleAccount_POSTUpdatesName(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	csrf := auth.CSRFToken(Conf.AppSecret, sess.ID)

	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account", "name=Renamed&_csrf="+csrf)
	rec := httptest.NewRecorder()
	HandleAccount(store).ServeHTTP(rec, r)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303", rec.Code)
	}
	got, _ := store.UserByID(context.Background(), u.ID)
	if got.Name != "Renamed" {
		t.Errorf("name=%q, want Renamed", got.Name)
	}
}

func TestHandleAccount_POSTBadCSRF(t *testing.T) {
	store := authTestStore(t)
	sess, u := loggedInRequestSession(t, store)
	r, _ := loggedInWith(t, store, u, sess, http.MethodPost, "/account", "name=X&_csrf=wrong")
	rec := httptest.NewRecorder()
	HandleAccount(store).ServeHTTP(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}
```

> Implementer note: replace the `loggedInRequest` helper above with the two small helpers the tests call — `loggedInRequestSession(t, store)` returns a created `(database.Session, database.User)` (create user, create session row, return them), and `loggedInWith(t, store, u, sess, method, target, body)` builds a request with that session's cookie and runs it through `auth.LoadUser(store)` to populate context. Keep the single `loggedInWith` helper and delete the unused `loggedInRequest` to avoid dead code. The point: the request context must carry the user+session, and the CSRF token must be `auth.CSRFToken(Conf.AppSecret, sess.ID)`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'TestHandleAccount' -v`
Expected: FAIL — `undefined: HandleAccount`.

- [ ] **Step 3: Write the account handlers**

```go
// handlers_account.go
package main

import (
	"net/http"
	"strings"

	"app/auth"
	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
)

// requireCSRF verifies the _csrf form field against the current session. It
// writes 403 and returns false on mismatch.
func requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	sess, ok := auth.SessionFrom(r.Context())
	if !ok || !auth.ValidCSRF(Conf.AppSecret, sess.ID, r.FormValue("_csrf")) {
		w.WriteHeader(http.StatusForbidden)
		return false
	}
	return true
}

func accountView(r *http.Request, active string) views.AccountView {
	sess, _ := auth.SessionFrom(r.Context())
	return views.AccountView{Active: active, CSRF: auth.CSRFToken(Conf.AppSecret, sess.ID)}
}

func HandleAccount(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		if r.Method == http.MethodPost {
			if !requireCSRF(w, r) {
				return nil
			}
			name := strings.TrimSpace(r.FormValue("name"))
			if err := store.UpdateUserName(r.Context(), user.ID, name); err != nil {
				return err
			}
			http.Redirect(w, r, "/account", http.StatusSeeOther)
			return nil
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Profile")
		av := accountView(r, "profile")
		return render(w, http.StatusOK, views.Profile(Conf.PageDataFor(account(r)), meta, av, user.Name, user.Email))
	}
	return rio.MakeHandler(fn)
}

func HandleSecurity(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		sess, _ := auth.SessionFrom(r.Context())
		sessions, err := store.ListUserSessions(r.Context(), user.ID)
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Security")
		av := accountView(r, "security")
		return render(w, http.StatusOK, views.Security(Conf.PageDataFor(account(r)), meta, av, sessions, sess.ID))
	}
	return rio.MakeHandler(fn)
}

func HandleRevokeSession(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		id := r.FormValue("id")
		// Only allow revoking the caller's own sessions.
		if sess, err := store.SessionByID(r.Context(), id); err == nil && sess.UserID == user.ID {
			_ = store.DeleteSession(r.Context(), id)
		}
		http.Redirect(w, r, "/account/security", http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleRevokeAllSessions(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if !requireCSRF(w, r) {
			return nil
		}
		user, _ := auth.UserFrom(r.Context())
		sess, _ := auth.SessionFrom(r.Context())
		if err := store.DeleteUserSessions(r.Context(), user.ID, sess.ID); err != nil {
			return err
		}
		http.Redirect(w, r, "/account/security", http.StatusSeeOther)
		return nil
	}
	return rio.MakeHandler(fn)
}

func HandleBilling() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Billing")
		return render(w, http.StatusOK, views.Billing(Conf.PageDataFor(account(r)), meta, accountView(r, "billing")))
	}
	return rio.MakeHandler(fn)
}

func HandleDeleteAccount(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		user, _ := auth.UserFrom(r.Context())
		if r.Method == http.MethodPost {
			if !requireCSRF(w, r) {
				return nil
			}
			if !strings.EqualFold(strings.TrimSpace(r.FormValue("confirm_email")), user.Email) {
				http.Redirect(w, r, "/account/delete", http.StatusSeeOther)
				return nil
			}
			if err := store.DeleteUser(r.Context(), user.ID); err != nil {
				return err
			}
			auth.ClearSessionCookie(w, !Conf.Debug)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return nil
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Delete account")
		return render(w, http.StatusOK, views.Danger(Conf.PageDataFor(account(r)), meta, accountView(r, "danger"), user.Email))
	}
	return rio.MakeHandler(fn)
}
```

- [ ] **Step 4: Register protected routes in main.go**

In `run()`, after the auth routes and before `/static/`, add the account area wrapped in `RequireUser`:

```go
	// Account (authenticated)
	s.Handle("/account", auth.RequireUser(HandleAccount(store)))
	s.Handle("/account/security", auth.RequireUser(HandleSecurity(store)))
	s.Handle("/account/sessions/revoke", auth.RequireUser(HandleRevokeSession(store)))
	s.Handle("/account/sessions/revoke-all", auth.RequireUser(HandleRevokeAllSessions(store)))
	s.Handle("/account/billing", auth.RequireUser(HandleBilling()))
	s.Handle("/account/delete", auth.RequireUser(HandleDeleteAccount(store)))
```

- [ ] **Step 5: Build and run the suite**

```bash
go build ./...
go test ./...
```
Expected: builds clean; all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add handlers_account.go handlers_account_test.go main.go
git commit -m "feat: account area handlers (profile/security/billing/delete) with CSRF"
```

---

### Task 14: Docs + full verification

**Files:**
- Modify: `README.md`, `Dockerfile` (env docs), `.gitignore` (already ignores `*.db`)

**Interfaces:**
- Produces: documented env + a verified end-to-end magic-link round-trip.

- [ ] **Step 1: Document the new env in README**

Add an "Accounts & auth" section to `README.md`:

```markdown
## Accounts & auth

Email magic-link login + a tabbed account area (`/account`). Config via env:

| Env | Purpose | Default |
|-----|---------|---------|
| `APP_SECRET` | HMAC key for CSRF/signing | dev fallback (prod: **required**) |
| `BASE_URL` | Absolute base for magic-link URLs | `http://localhost:<port>` |
| `POSTMARK_TOKEN` | Postmark server token | unset → links logged to console |
| `EMAIL_FROM` | From address | `noreply@localhost` |

In dev with no `POSTMARK_TOKEN`, the magic link is printed to the server log —
click it from your terminal. In production, set all four (`APP_SECRET` is
mandatory; the app refuses to start without it).
```

- [ ] **Step 2: Document env in the Dockerfile**

Under the existing `ENV DB_DIR=/data` line in `Dockerfile`, add a comment block:

```dockerfile
# Auth/email (set at runtime): APP_SECRET (required in prod), BASE_URL,
# POSTMARK_TOKEN, EMAIL_FROM.
```

- [ ] **Step 3: Full verification (vet, test, live magic-link round-trip)**

```bash
make check
go build -ldflags="-X 'main.BuildEnv=debug'" -o /tmp/app .
DB_DIR=/tmp ./tmp/app &      # console sender: no email account needed
sleep 1
# Request a link; capture it from the server log, then verify it:
curl -s -X POST -d 'email=you@example.com&next=/account' localhost:3000/login -o /dev/null -w '%{http_code}\n'   # 303
# -> find the "/auth/verify?token=..." line in the server log, then:
#    curl -s -c /tmp/jar "localhost:3000/auth/verify?token=<TOKEN>" -o /dev/null -w '%{http_code}\n'  # 303 + sets cookie
#    curl -s -b /tmp/jar localhost:3000/account | grep -q "Account" && echo "account ok"
kill %1
```
Expected: login POST returns 303; the server log shows the verify link; visiting it sets a session cookie and `/account` renders for the cookie.

- [ ] **Step 4: Commit**

```bash
git add README.md Dockerfile
git commit -m "docs: document accounts env and dev magic-link flow"
```

---

## Self-Review

**Spec coverage:**
- Server-side sessions, revocable → Tasks 2, 9, 13 (list/revoke/revoke-all) ✓
- Individual users, case-insensitive email, find-or-create → Tasks 1, 12 ✓
- Unified signup, always "check your inbox", no enumeration → Task 12 (`HandleLogin` always redirects to `/login/sent`) ✓
- Email: Postmark prod / Console dev, hashed single-use 15-min tokens → Tasks 3, 4, 12 ✓
- Cookie `HttpOnly`/`Secure`/`SameSite=Lax`, server stores hash → Tasks 7, 12 ✓
- `LoadUser`/`RequireUser` → Task 9; CSRF (`SameSite` + HMAC token on authed forms) → Tasks 8, 13 ✓
- Rate limit on /login → Tasks 8, 12 ✓
- Account area (profile/security/billing-stub/danger) + nav + flash → Tasks 10, 11, 13 ✓
- Config env + prod-secret fail-fast → Tasks 5, 12 ✓
- Open-redirect guard (`SafeNext`) → Tasks 6, 12 ✓
- No new dependency → entire plan is stdlib + existing rio ✓
- Docs → Task 14 ✓

**Placeholder scan:** Every code step shows complete code; every test step has real assertions. The only prose note is the Task 13 helper-consolidation instruction, which specifies exact helper signatures and behavior. ✓

**Type consistency:** `database.User/Session/LoginToken` and store methods (`CreateUser`/`UserByEmail`/`UserByID`/`UpdateUserName`/`DeleteUser`, `CreateSession`/`SessionByID`/`ListUserSessions`/`DeleteSession`/`DeleteUserSessions`, `CreateToken`/`ConsumeToken`); `email.Sender`/`Send(ctx,to,subject,textBody)`; `auth.GenerateToken`/`HashToken`/`SafeNext`/`SetSessionCookie`/`ClearSessionCookie`/`SessionToken`/`CSRFToken`/`ValidCSRF`/`NewLimiter`/`Allow`/`LoadUser`/`RequireUser`/`UserFrom`/`SessionFrom`/`CookieName`/`SessionTTL`; `config.Account`/`PageDataFor`/`BaseURL`/`AppSecret`/`PostmarkToken`/`EmailFrom`; `views.Login`/`LoginSent`/`VerifyError`/`AccountView`/`Profile`/`Security`/`Billing`/`Danger`; handlers `HandleLogin`/`HandleLoginSent`/`HandleVerify`/`HandleLogout`/`HandleAccount`/`HandleSecurity`/`HandleRevokeSession`/`HandleRevokeAllSessions`/`HandleBilling`/`HandleDeleteAccount` — used consistently across tasks. ✓

**Notes for the implementer:**
- `account(r)` (in `handlers_auth.go`) is shared by both handler files — define it once.
- If `dom.Title`/`rio.LogError` aren't in your rio version, the fallbacks are noted inline (`dom.Aria("label", …)` / `log.Printf`).
- modernc supports SQLite `RETURNING`; if you pin an older driver, switch `CreateUser` to `ExecContext` + `LastInsertId` + a follow-up `UserByID`.
