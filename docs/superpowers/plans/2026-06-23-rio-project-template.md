# rio-project-template Rewrite — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the existing `rio-project-template` repo so a product spins up with rio + `rio/dom` rendering (no HTML templates) + `rio/ui` themed components + SQLite with a built-in migration system + a `scratch` Docker image, adding exactly one Go dependency (`modernc.org/sqlite`).

**Architecture:** Go module `app` with three internal packages — `app/config` (Config, Tokens, links, derived DB path), `app/database` (Open + pragmas, a driver-agnostic migration runner, a raw-SQL store), `app/views` (dom + rio/ui page builders) — plus root `package main` (wiring + handlers). Migrations are embedded and run at startup; the binary self-migrates on boot. Tailwind v4 scans the vendored `rio/ui` source so component classes are emitted.

**Tech Stack:** Go 1.26, `github.com/tunedmystic/rio` v0.26.0 (`rio`, `rio/dom`, `rio/ui`), `modernc.org/sqlite` (pure-Go, cgo-free), TailwindCSS v4 standalone binary.

## Global Constraints

- Module path is `app`; internal imports are `app/config`, `app/database`, `app/views`.
- Exactly ONE non-rio Go dependency: `modernc.org/sqlite`. No ORM, no query builder, no migration library.
- This is an **in-place rewrite** of the existing repo. The starting state has: `main.go`, `config.go`, `handlers.go` (using rio HTML templates), `Dockerfile`, `Makefile`, `tailwind.config.js` (v3), `tailwind.input.css` (v3), `fly.toml`, `templates/`, `static/`, `go.mod` (`module app`, `go 1.22.1`, `rio v0.0.3`).
- Driver is pure-Go `modernc.org/sqlite` (registers driver name `sqlite`). Keep `CGO_ENABLED=0` and `FROM scratch`.
- The migration runner (`app/database/migrate.go`) MUST stay driver-agnostic and stdlib-only (`database/sql`, `io/fs`, `sort`, `fmt`) with NO product imports, NO embed, NO driver — so it can later be promoted to `rio/migrate` unchanged.
- rio/ui components own their `class`; never pass a `class` attribute to them. Never construct Tailwind class names at runtime — select full literal strings.
- SQLite path derives from `ProjectName`: `<DB_DIR>/<ProjectName>.db`, `DB_DIR` defaulting to `/data` (prod) or `.` (dev/Debug).
- No fly.io, no HTML templates, no `tailwind.config.js` may remain at the end.
- Run all Go tests with `go test ./...` from the repo root.
- Whenever `go.mod` changes, re-run `go mod tidy` and `go mod vendor` so the committed `vendor/` stays consistent (Tailwind scans it).

---

### Task 1: Bump dependencies and Go version

**Files:**
- Modify: `go.mod`
- Create: `vendor/` (via `go mod vendor`)

**Interfaces:**
- Produces: a repo that compiles against rio v0.26.0 with a committed `vendor/` tree.

- [ ] **Step 1: Bump the Go directive and rio version**

Edit `go.mod` so the `go` line reads `go 1.26` and run:

```bash
go get github.com/tunedmystic/rio@v0.26.0
go mod tidy
```

Expected: `go.mod` now has `go 1.26` and `require github.com/tunedmystic/rio v0.26.0`.

- [ ] **Step 2: Verify the existing template still compiles**

Run: `go build ./...`
Expected: builds with no errors. (The existing `rio.Templates`/`rio.Render`/`rio.NewServer` calls all still exist in v0.26.0, so the old code compiles — it is replaced in later tasks.)

- [ ] **Step 3: Vendor dependencies**

Run: `go mod vendor`
Expected: a `vendor/` directory exists containing `vendor/github.com/tunedmystic/rio/ui/*.go`.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum vendor
git commit -m "chore: bump go to 1.26 and rio to v0.26.0, vendor deps"
```

---

### Task 2: Database — Open with pragmas

**Files:**
- Create: `database/database.go`
- Test: `database/database_test.go`

**Interfaces:**
- Consumes: `modernc.org/sqlite` (driver), `database/sql`.
- Produces: `func Open(path string) (*sql.DB, error)` — opens SQLite with WAL, foreign keys on, single writer.

- [ ] **Step 1: Write the failing test**

```go
// database/database_test.go
package database

import (
	"path/filepath"
	"testing"
)

func TestOpen_SetsPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("query pragma: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./database/ -run TestOpen -v`
Expected: FAIL — `undefined: Open`.

- [ ] **Step 3: Write minimal implementation**

```go
// database/database.go
package database

import (
	"database/sql"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; registers "sqlite"
)

// Open opens a SQLite database at path with sane pragmas for a web app.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	db.SetMaxOpenConns(1) // simplest correct default for SQLite writes
	return db, nil
}
```

- [ ] **Step 4: Add the dependency and verify the test passes**

```bash
go get modernc.org/sqlite@latest
go mod tidy
go mod vendor
go test ./database/ -run TestOpen -v
```
Expected: PASS. `go.mod` now requires `modernc.org/sqlite`.

- [ ] **Step 5: Commit**

```bash
git add database/database.go database/database_test.go go.mod go.sum vendor
git commit -m "feat(db): add Open with SQLite pragmas via modernc driver"
```

---

### Task 3: Database — migration runner

**Files:**
- Create: `database/migrate.go`, `database/migrations.go`, `database/migrations/0001_init.sql`
- Test: `database/migrate_test.go`

**Interfaces:**
- Consumes: `Open` (Task 2), `database/sql`, `io/fs`, `testing/fstest`.
- Produces:
  - `func Migrate(db *sql.DB, files fs.FS) error` — generic, stdlib-only, promotable.
  - `func MigrateUp(db *sql.DB) error` — applies the embedded `migrations/*.sql`.

- [ ] **Step 1: Write the failing test**

```go
// database/migrate_test.go
package database

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestMigrate_AppliesAndIsIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	files := fstest.MapFS{
		"migrations/0001_init.sql": {Data: []byte(
			"CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL);")},
	}

	if err := Migrate(db, files); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Table exists.
	if _, err := db.Exec("INSERT INTO widgets (name) VALUES ('a')"); err != nil {
		t.Fatalf("insert into migrated table: %v", err)
	}

	// Tracking row recorded.
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n != 1 {
		t.Fatalf("schema_migrations count = %d, want 1", n)
	}

	// Idempotent: re-running applies nothing and does not error.
	if err := Migrate(db, files); err != nil {
		t.Fatalf("Migrate (2nd run): %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n != 1 {
		t.Errorf("after 2nd run count = %d, want 1", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./database/ -run TestMigrate -v`
Expected: FAIL — `undefined: Migrate`.

- [ ] **Step 3: Write the migration runner**

```go
// database/migrate.go
package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
)

// Migrate applies every migrations/*.sql file in files that has not yet been
// applied, in filename order, each inside a transaction. It is forward-only
// and idempotent. Driver- and embed-agnostic (stdlib only) so it can be lifted
// into a shared library unchanged.
func Migrate(db *sql.DB, files fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	paths, err := fs.Glob(files, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(paths)

	for _, path := range paths {
		var exists bool
		if err := db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = ?)", path,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		stmt, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(stmt)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", path, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (name) VALUES (?)", path); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./database/ -run TestMigrate -v`
Expected: PASS.

- [ ] **Step 5: Add the embedded migrations and MigrateUp**

```go
// database/migrations.go
package database

import (
	"database/sql"
	"embed"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrateUp applies the embedded migrations to db.
func MigrateUp(db *sql.DB) error {
	return Migrate(db, migrationsFS)
}
```

```sql
-- database/migrations/0001_init.sql
CREATE TABLE messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    body       TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 6: Verify the package builds**

Run: `go build ./database/`
Expected: builds (the embed directive resolves `migrations/0001_init.sql`).

- [ ] **Step 7: Commit**

```bash
git add database/migrate.go database/migrations.go database/migrations database/migrate_test.go
git commit -m "feat(db): add driver-agnostic migration runner and embedded migrations"
```

---

### Task 4: Database — Store

**Files:**
- Create: `database/store.go`
- Test: `database/store_test.go`

**Interfaces:**
- Consumes: `Open` (Task 2), `MigrateUp` (Task 3).
- Produces:
  - `type Message struct { ID int64; Body string; CreatedAt time.Time }`
  - `type Store struct { ... }`, `func NewStore(db *sql.DB) *Store`
  - `func (s *Store) ListMessages(ctx context.Context) ([]Message, error)`
  - `func (s *Store) CreateMessage(ctx context.Context, body string) error`

- [ ] **Step 1: Write the failing test**

```go
// database/store_test.go
package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStore_CreateAndList(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	if err := MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}

	store := NewStore(db)
	ctx := context.Background()

	if err := store.CreateMessage(ctx, "hello"); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if err := store.CreateMessage(ctx, "world"); err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}

	msgs, err := store.ListMessages(ctx)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	// Newest first.
	if msgs[0].Body != "world" {
		t.Errorf("msgs[0].Body = %q, want %q", msgs[0].Body, "world")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./database/ -run TestStore -v`
Expected: FAIL — `undefined: NewStore`.

- [ ] **Step 3: Write minimal implementation**

```go
// database/store.go
package database

import (
	"context"
	"database/sql"
	"time"
)

// Message is a row in the messages table (the demo resource).
type Message struct {
	ID        int64
	Body      string
	CreatedAt time.Time
}

// Store provides data access methods over a *sql.DB using raw SQL.
type Store struct {
	db *sql.DB
}

// NewStore constructs a Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateMessage inserts a new message.
func (s *Store) CreateMessage(ctx context.Context, body string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO messages (body) VALUES (?)", body)
	return err
}

// ListMessages returns all messages, newest first.
func (s *Store) ListMessages(ctx context.Context) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, body, created_at FROM messages ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./database/ -run TestStore -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add database/store.go database/store_test.go
git commit -m "feat(db): add Store with ListMessages and CreateMessage"
```

---

### Task 5: Config package

**Files:**
- Create: `config/config.go`
- Test: `config/config_test.go`
- Delete: `config.go` (root) — contents move/rewrite into `config/` and `main.go` (Task 7)

> The old root `config.go` is in `package main` and embeds `templates`/`static`. We replace it: presentation/config types move into `app/config`; the `static` embed and build-time vars move to `main.go` in Task 7. To keep the build green, this task ADDS the new `config/` package but does NOT yet delete the old `config.go` (deletion happens in the Task 7 cutover, where `main.go`/`handlers.go` stop referencing the old symbols). The new package compiles independently.

**Interfaces:**
- Consumes: `github.com/tunedmystic/rio/ui`.
- Produces:
  - `type Link struct { Text, Href string }`
  - `type Meta struct { Title, Description, Heading, PageURL string }`
  - `type PageData struct { SiteName string; Tokens ui.Tokens; HeaderLinks, FooterLinks []Link }`
  - `type Config struct { ... ProjectName, DBPath string; Tokens ui.Tokens; ... }`
  - `func New(buildEnv string) Config`
  - `func (c Config) PageData() PageData`
  - `func (c Config) NewMeta(pageURL, heading string) Meta`
  - `func DBPath(projectName string, debug bool) string`

- [ ] **Step 1: Write the failing test**

```go
// config/config_test.go
package config

import (
	"path/filepath"
	"testing"
)

func TestDBPath_DerivesFromProjectName(t *testing.T) {
	t.Setenv("DB_DIR", "/data")
	got := DBPath("RioProg", false)
	want := filepath.Join("/data", "RioProg.db")
	if got != want {
		t.Errorf("DBPath = %q, want %q", got, want)
	}
}

func TestDBPath_DevDefaultsToCurrentDir(t *testing.T) {
	t.Setenv("DB_DIR", "") // unset -> dev default when debug
	got := DBPath("RioProg", true)
	want := filepath.Join(".", "RioProg.db")
	if got != want {
		t.Errorf("DBPath = %q, want %q", got, want)
	}
}

func TestNew_PopulatesDBPathAndTokens(t *testing.T) {
	t.Setenv("DB_DIR", "/data")
	c := New("production")
	if c.ProjectName == "" {
		t.Fatal("ProjectName is empty")
	}
	if c.DBPath != filepath.Join("/data", c.ProjectName+".db") {
		t.Errorf("DBPath = %q", c.DBPath)
	}
	if c.Tokens.ColorPrimary == "" {
		t.Error("Tokens.ColorPrimary is empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -v`
Expected: FAIL — `undefined: DBPath` / `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

```go
// config/config.go
package config

import (
	"os"
	"path/filepath"

	"github.com/tunedmystic/rio/ui"
)

// Link is an anchor link used in nav/footer.
type Link struct {
	Text string
	Href string
}

// Meta is per-page metadata for the document head.
type Meta struct {
	Title       string
	Description string
	Heading     string
	PageURL     string
}

// PageData is the subset of config the views need to render a page.
type PageData struct {
	SiteName    string
	Tokens      ui.Tokens
	HeaderLinks []Link
	FooterLinks []Link
}

// Config holds the product configuration. ProjectName is the per-clone seam.
type Config struct {
	ProjectName string
	SiteName    string
	SiteURL     string
	Description string
	Addr        string
	Debug       bool
	DBPath      string
	Tokens      ui.Tokens
	HeaderLinks []Link
	FooterLinks []Link
}

// New builds the Config. buildEnv comes from the main package's build-time var;
// "debug" selects development defaults.
func New(buildEnv string) Config {
	debug := buildEnv == "debug"

	c := Config{
		ProjectName: "riostarter", // <-- change this per product; sets the db file name
		SiteName:    "Rio Starter",
		SiteURL:     "https://riostarter.example.com",
		Description: "A starter built with rio. Clone it, set ProjectName, ship.",
		Addr:        ":3000",
		Debug:       debug,
		Tokens:      defaultTokens(),
		HeaderLinks: []Link{
			{Text: "Messages", Href: "/messages"},
			{Text: "About", Href: "/about"},
		},
		FooterLinks: []Link{
			{Text: "Home", Href: "/"},
			{Text: "About", Href: "/about"},
			{Text: "Privacy Policy", Href: "/privacy-policy"},
		},
	}
	c.DBPath = DBPath(c.ProjectName, debug)
	return c
}

// DBPath derives the SQLite file path from the project name. The directory is
// DB_DIR (default /data in prod, the working dir in dev), the file is
// <projectName>.db — keeping each project's database unique on a shared volume.
func DBPath(projectName string, debug bool) string {
	dir := os.Getenv("DB_DIR")
	if dir == "" {
		dir = "/data"
		if debug {
			dir = "."
		}
	}
	return filepath.Join(dir, projectName+".db")
}

// PageData returns the view-facing subset of the config.
func (c Config) PageData() PageData {
	return PageData{
		SiteName:    c.SiteName,
		Tokens:      c.Tokens,
		HeaderLinks: c.HeaderLinks,
		FooterLinks: c.FooterLinks,
	}
}

// NewMeta builds per-page metadata, defaulting title/description from the config.
func (c Config) NewMeta(pageURL, heading string) Meta {
	title := c.SiteName
	if heading != "" {
		title = heading + " - " + c.SiteName
	}
	return Meta{
		Title:       title,
		Description: c.Description,
		Heading:     heading,
		PageURL:     pageURL,
	}
}

// defaultTokens is the starter brand. Products edit this literal.
func defaultTokens() ui.Tokens {
	return ui.Tokens{
		FontFamily:        `"Inter", ui-sans-serif, system-ui, sans-serif`,
		FontSizeSm:        "16px",
		FontSizeBase:      "18px",
		FontSizeLg:        "20px",
		FontSizeXl:        "24px",
		FontSize2xl:       "30px",
		ColorPrimary:      "#4f46e5",
		OnPrimary:         "#ffffff",
		ColorSecondary:    "#475569",
		OnSecondary:       "#ffffff",
		ColorBackground:   "#ffffff",
		ColorSurface:      "#f8fafc",
		ColorText:         "#0f172a",
		ColorTextMuted:    "#64748b",
		ColorBorder:       "#e2e8f0",
		ColorSuccess:      "#16a34a",
		ColorWarning:      "#d97706",
		ColorDanger:       "#dc2626",
		ColorInfo:         "#2563eb",
		RadiusBase:        "0.5rem",
		FontWeightHeading: "700",
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./config/ -v`
Expected: PASS.

> Note: the repo still has the old root `config.go` (`package main`) at this point; both compile (different packages). The old one is removed in Task 7.

- [ ] **Step 5: Commit**

```bash
git add config/config.go config/config_test.go
git commit -m "feat(config): add config package with ProjectName-derived DB path"
```

---

### Task 6: Views package (dom + rio/ui)

**Files:**
- Create: `views/layout.go`, `views/components.go`, `views/pages.go`
- Test: `views/views_test.go`

**Interfaces:**
- Consumes: `app/config` (Task 5), `app/database` (Task 4: `Message`), `rio/dom`, `rio/ui`.
- Produces:
  - `func Page(pd config.PageData, meta config.Meta, body ...dom.Node) dom.Node`
  - `func Home(pd config.PageData, meta config.Meta) dom.Node`
  - `func About(pd config.PageData, meta config.Meta) dom.Node`
  - `func PrivacyPolicy(pd config.PageData, meta config.Meta) dom.Node`
  - `func NotFound(pd config.PageData, meta config.Meta) dom.Node`
  - `func Messages(pd config.PageData, meta config.Meta, msgs []database.Message) dom.Node`

- [ ] **Step 1: Write the failing test**

```go
// views/views_test.go
package views

import (
	"bytes"
	"strings"
	"testing"

	"app/config"
	"app/database"
)

func render(n interface{ Render(w *bytes.Buffer) error }) string {
	var b bytes.Buffer
	_ = n.Render(&b)
	return b.String()
}

func testPageData() config.PageData {
	c := config.New("debug")
	return c.PageData()
}

func TestPage_RendersHeadAndChrome(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Hi - Rio Starter", Description: "d"}
	var b bytes.Buffer
	_ = Page(pd, meta, nil).Render(&b)
	html := b.String()

	for _, want := range []string{
		"<!DOCTYPE html>",
		"<title>Hi - Rio Starter</title>",
		"<style>",                          // StyleVars block
		"--color-primary:",                 // a token variable
		`href="/static/css/styles.css"`,    // stylesheet link
		"</html>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Page output missing %q", want)
		}
	}
}

func TestMessages_ListsBodies(t *testing.T) {
	pd := testPageData()
	meta := config.Meta{Title: "Messages"}
	msgs := []database.Message{{ID: 1, Body: "first-msg"}, {ID: 2, Body: "second-msg"}}
	var b bytes.Buffer
	_ = Messages(pd, meta, msgs).Render(&b)
	html := b.String()

	if !strings.Contains(html, "first-msg") || !strings.Contains(html, "second-msg") {
		t.Error("Messages output missing message bodies")
	}
	if !strings.Contains(html, `action="/messages"`) {
		t.Error("Messages output missing the create form")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./views/ -v`
Expected: FAIL — `undefined: Page` / `undefined: Messages`.

- [ ] **Step 3: Write the layout**

```go
// views/layout.go
package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

// Page wraps body content in the full HTML document: head (tokens + meta +
// stylesheet), navbar, main, footer.
func Page(pd config.PageData, meta config.Meta, body ...dom.Node) dom.Node {
	return dom.Doctype(dom.Html(
		dom.Lang("en"),
		dom.Head(
			dom.Meta(dom.Charset("utf-8")),
			dom.Meta(dom.Name("viewport"), dom.Content("width=device-width, initial-scale=1")),
			dom.TitleEl(dom.Text(meta.Title)),
			dom.Meta(dom.Name("description"), dom.Content(meta.Description)),
			pd.Tokens.StyleVars(),
			dom.Link(dom.Rel("stylesheet"), dom.Href("/static/css/styles.css")),
		),
		dom.Body(
			dom.Class("min-h-screen flex flex-col bg-[var(--color-background)] text-[var(--color-text)] font-[family-name:var(--font-family)] text-[length:var(--font-size-base)] leading-relaxed antialiased"),
			navbar(pd),
			dom.Main(dom.Class("flex-1 py-10"), ui.Container(body...)),
			footer(pd),
		),
	))
}
```

- [ ] **Step 4: Write the components (nav, footer, submit button)**

```go
// views/components.go
package views

import (
	"app/config"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

func navbar(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.HeaderLinks)+1)
	links = append(links, dom.Class("flex items-center gap-6"))
	for _, l := range pd.HeaderLinks {
		links = append(links, ui.Link(l.Href, l.Text))
	}
	return dom.Header(
		dom.Class("border-b border-[var(--color-border)]"),
		ui.Container(
			dom.Div(
				dom.Class("flex items-center justify-between py-4"),
				ui.Link("/", pd.SiteName),
				dom.Nav(links...),
			),
		),
	)
}

func footer(pd config.PageData) dom.Node {
	links := make([]dom.Node, 0, len(pd.FooterLinks)+1)
	links = append(links, dom.Class("flex flex-wrap items-center gap-6"))
	for _, l := range pd.FooterLinks {
		links = append(links, ui.Link(l.Href, l.Text))
	}
	return dom.Footer(
		dom.Class("border-t border-[var(--color-border)] py-8 text-[length:var(--font-size-sm)] text-[var(--color-text-muted)]"),
		ui.Container(dom.Nav(links...)),
	)
}

// submitButton renders a submit button styled like a ui primary button.
// ui.Button hardcodes type="button"; if submit buttons recur across products,
// promote a submit/type option into rio/ui (rule of three).
func submitButton(label string) dom.Node {
	return dom.Button(
		dom.Type("submit"),
		dom.Class("inline-flex items-center justify-center gap-2 rounded-[var(--radius-base)] px-4 py-2.5 text-[length:var(--font-size-sm)] font-semibold tracking-tight bg-[var(--color-primary)] text-[var(--color-on-primary)] shadow-sm hover:shadow-md hover:brightness-105 active:brightness-95 cursor-pointer"),
		dom.Text(label),
	)
}
```

- [ ] **Step 5: Write the pages**

```go
// views/pages.go
package views

import (
	"app/config"
	"app/database"

	"github.com/tunedmystic/rio/dom"
	"github.com/tunedmystic/rio/ui"
)

func Home(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Welcome to "+pd.SiteName),
			ui.Text(ui.TextDefault, "A starter built with rio, rio/dom and rio/ui. Clone it, set your ProjectName and tokens, and start building."),
			ui.Text(ui.TextMuted, "The Messages page below is a live SQLite demo — delete it to drop the example."),
			ui.ButtonLink(ui.ButtonPrimary, "/messages", "Open the messages demo"),
		),
	)
}

func About(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "About"),
			ui.Text(ui.TextDefault, "Replace this page with your product's story."),
		),
	)
}

func PrivacyPolicy(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Privacy Policy"),
			ui.Text(ui.TextDefault, "Replace this page with your product's privacy policy."),
		),
	)
}

func NotFound(pd config.PageData, meta config.Meta) dom.Node {
	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Page not found"),
			ui.Text(ui.TextMuted, "That page does not exist."),
			ui.ButtonLink(ui.ButtonPrimary, "/", "Go home"),
		),
	)
}

func Messages(pd config.PageData, meta config.Meta, msgs []database.Message) dom.Node {
	items := make([]dom.Node, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, ui.Card(ui.Text(ui.TextDefault, m.Body)))
	}

	form := dom.Form(
		dom.Method("post"),
		dom.Action("/messages"),
		dom.Class("mb-8"),
		ui.TextField("body", "New message", "", ""),
		submitButton("Add message"),
	)

	return Page(pd, meta,
		ui.Stack(ui.GapMd,
			ui.Heading(ui.H1, "Messages"),
			ui.Text(ui.TextMuted, "A SQLite-backed demo: add a message and it persists."),
			form,
			ui.Stack(ui.GapMd, items...),
		),
	)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./views/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add views/
git commit -m "feat(views): add dom + rio/ui layout, components and pages"
```

---

### Task 7: Cutover — main.go, handlers.go, deletions

**Files:**
- Rewrite: `main.go`, `handlers.go`
- Delete: `config.go` (root), `templates/` (whole dir), `fly.toml`, `tailwind.config.js`
- Test: `handlers_test.go`

**Interfaces:**
- Consumes: `app/config`, `app/database`, `app/views`, `rio`, `rio/dom`.
- Produces: a running server whose routes render via dom and whose `/messages` route reads+writes the DB.

> This is the atomic cutover: the new packages replace the old template rendering. After it, no HTML templates or `rio.Templates`/`rio.Render` remain.

- [ ] **Step 1: Write the failing handler test**

```go
// handlers_test.go
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"app/database"
)

func newTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.MigrateUp(db); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	return database.NewStore(db)
}

func TestHandleMessages_GET(t *testing.T) {
	store := newTestStore(t)
	_ = store.CreateMessage(context.Background(), "seeded-message")

	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	rec := httptest.NewRecorder()
	HandleMessages(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "seeded-message") {
		t.Error("response missing seeded message")
	}
}

func TestHandleMessages_POSTCreatesAndRedirects(t *testing.T) {
	store := newTestStore(t)

	req := httptest.NewRequest(http.MethodPost, "/messages", strings.NewReader("body=created-here"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	HandleMessages(store).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	msgs, _ := store.ListMessages(context.Background())
	if len(msgs) != 1 || msgs[0].Body != "created-here" {
		t.Errorf("message not persisted: %+v", msgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestHandleMessages -v`
Expected: FAIL — `undefined: HandleMessages` (and the old `package main` may not compile; that is resolved in the next steps which rewrite it).

- [ ] **Step 3: Delete the old files**

```bash
git rm config.go fly.toml tailwind.config.js
git rm -r templates
```

- [ ] **Step 4: Write the new main.go**

```go
// main.go
package main

import (
	"embed"
	"log"

	"app/config"
	"app/database"

	"github.com/tunedmystic/rio"
)

// Build-time variables, injected via -ldflags.
var (
	BuildDate = "build-date"
	BuildHash = "build-hash"
	BuildEnv  = "production"
)

//go:embed all:static
var staticFS embed.FS

// Conf is the application configuration.
var Conf = config.New(BuildEnv)

func main() {
	db, err := database.Open(Conf.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := database.MigrateUp(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := database.NewStore(db)

	s := rio.NewServer()
	s.Handle("/", HandleHome())
	s.Handle("/messages", HandleMessages(store))
	s.Handle("/about", HandleAbout())
	s.Handle("/privacy-policy", HandlePrivacyPolicy())
	s.Handle("/version", HandleVersion())
	s.Handle("/static/", HandleStatic())

	log.Fatal(s.Serve(Conf.Addr))
}
```

- [ ] **Step 5: Write the new handlers.go**

```go
// handlers.go
package main

import (
	"net/http"
	"strings"

	"app/database"
	"app/views"

	"github.com/tunedmystic/rio"
	"github.com/tunedmystic/rio/dom"
)

// render writes an HTML dom node with the given status.
func render(w http.ResponseWriter, status int, node dom.Node) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	return node.Render(w)
}

func HandleHome() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		// Treat any unknown path under "/" as 404.
		if r.URL.Path != "/" {
			meta := Conf.NewMeta(r.URL.RequestURI(), "Not found")
			return render(w, http.StatusNotFound, views.NotFound(Conf.PageData(), meta))
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "")
		return render(w, http.StatusOK, views.Home(Conf.PageData(), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleMessages(store *database.Store) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == http.MethodPost {
			body := strings.TrimSpace(r.FormValue("body"))
			if body != "" {
				if err := store.CreateMessage(r.Context(), body); err != nil {
					return err
				}
			}
			http.Redirect(w, r, "/messages", http.StatusSeeOther)
			return nil
		}

		msgs, err := store.ListMessages(r.Context())
		if err != nil {
			return err
		}
		meta := Conf.NewMeta(r.URL.RequestURI(), "Messages")
		return render(w, http.StatusOK, views.Messages(Conf.PageData(), meta, msgs))
	}
	return rio.MakeHandler(fn)
}

func HandleAbout() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "About")
		return render(w, http.StatusOK, views.About(Conf.PageData(), meta))
	}
	return rio.MakeHandler(fn)
}

func HandlePrivacyPolicy() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) error {
		meta := Conf.NewMeta(r.URL.RequestURI(), "Privacy Policy")
		return render(w, http.StatusOK, views.PrivacyPolicy(Conf.PageData(), meta))
	}
	return rio.MakeHandler(fn)
}

func HandleVersion() http.Handler {
	version := struct {
		BuildDate string
		BuildHash string
		BuildProd bool
	}{BuildDate: BuildDate, BuildHash: BuildHash, BuildProd: !Conf.Debug}

	fn := func(w http.ResponseWriter, r *http.Request) error {
		return rio.Json200(w, version)
	}
	return rio.MakeHandler(fn)
}

func HandleStatic() http.Handler {
	cache := rio.CacheControlWithAge(1_209_600) // 2 weeks
	return cache(rio.FileServer(staticFS))
}
```

- [ ] **Step 6: Tidy, vendor, and run the full test suite**

```bash
go mod tidy
go mod vendor
go build ./...
go test ./...
```
Expected: builds clean; all tests PASS. (`staticFS` embeds the existing `static/` dir; ensure at least `static/css/` exists — create `static/css/.gitkeep` if needed so the embed has content.)

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: cut over to dom + rio/ui rendering and DB-backed routes"
```

---

### Task 8: Tailwind v4, Makefile, Dockerfile, ignore files

**Files:**
- Rewrite: `tailwind.input.css`, `Makefile`, `Dockerfile`, `.gitignore`, `.dockerignore`

**Interfaces:**
- Produces: a v4 CSS build that scans vendored rio/ui, a fly-free Makefile, a Go 1.26 scratch image with a `/data` volume.

- [ ] **Step 1: Rewrite the Tailwind input CSS (v3 → v4)**

```css
/* tailwind.input.css */
@import "tailwindcss";

/* Scan the product's Go source and the vendored rio/ui source so the
   bg-[var(--...)] component class literals are emitted. */
@source "./**/*.go";
@source "./vendor/github.com/tunedmystic/rio/ui/**/*.go";
```

- [ ] **Step 2: Update the Makefile**

Replace the Tailwind download target, the tailwind target, and remove `deploy`. The `bin/tailwind` target becomes (pinned v4):

```make
bin/tailwind:
	@echo "✨📦✨ Downloading tailwindcss v4 binary\n"
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v4.1.1/tailwindcss-macos-arm64
	chmod +x tailwindcss-macos-arm64
	mkdir -p bin
	mv tailwindcss-macos-arm64 ./bin/tailwind
```

The `tailwind` target runs vendor first (so the scan source exists), then builds:

```make
## @(app) - 📦 Build the CSS with Tailwind v4
tailwind: bin/tailwind
	@echo "✨📦✨ Running tailwind\n"
	@go mod vendor
	@./bin/tailwind --input ./tailwind.input.css --output ./static/css/styles.css --minify
```

Delete the `deploy:` target entirely. Add a db-reset helper:

```make
## @(app) - 🗑️ Delete the local dev database
db-reset:
	@echo "✨🗑️✨ Removing local database\n"
	@rm -f ./*.db ./*.db-shm ./*.db-wal
```

- [ ] **Step 3: Update the Dockerfile (Go 1.26, vendored build, /data)**

```dockerfile
# ---- build ----
FROM golang:1.26-alpine AS builder

ENV GO111MODULE=on \
    GOOS=linux \
    CGO_ENABLED=0

ARG BUILD_HASH

WORKDIR /build
RUN apk add --no-cache upx

COPY . .
RUN go build -mod=vendor \
    -ldflags="-s -w -X 'main.BuildHash=$BUILD_HASH' -X 'main.BuildDate=$(date)'" \
    -o app ./...
RUN upx /build/app

# ---- final ----
FROM scratch AS final

WORKDIR /x
COPY --from=builder /build/app .

# SQLite database directory (mount a volume here to persist:
#   docker run -v ./data:/data ...). DB file is /data/<ProjectName>.db.
ENV DB_DIR=/data
EXPOSE 3000

CMD ["/x/app"]
```

- [ ] **Step 4: Update ignore files**

`.gitignore` — add the database files and keep vendor tracked:

```gitignore
# Binaries
bin/

# Database
*.db
*.db-shm
*.db-wal
/data/

# Directories
.notes/
.misc/
.tmp/
```

`.dockerignore` — keep build context lean but DO NOT ignore `vendor/`:

```
bin
*.db
*.db-shm
*.db-wal
.DS_Store
.vscode
.notes
.misc
.tmp
data
```

- [ ] **Step 5: Verify the CSS build and the Docker build**

```bash
make tailwind
test -s static/css/styles.css && echo "css built"
grep -q 'var(--color-primary)' static/css/styles.css && echo "ui classes emitted"
docker build --build-arg BUILD_HASH=test -t rio-template-test .
```
Expected: `css built`, `ui classes emitted`, and the Docker image builds to a `scratch` final stage.

- [ ] **Step 6: Commit**

```bash
git add tailwind.input.css Makefile Dockerfile .gitignore .dockerignore static/css/styles.css
git commit -m "build: Tailwind v4 (vendored scan), drop fly.io, Go 1.26 scratch image with /data"
```

---

### Task 9: README and final verification

**Files:**
- Rewrite: `README.md`

**Interfaces:**
- Produces: usage docs and a verified end-to-end run.

- [ ] **Step 1: Rewrite the README**

```markdown
# rio project template

A starter for rio products: rio + rio/dom (no HTML templates) + rio/ui themed
components + SQLite with built-in migrations, in a scratch Docker image.

## Quick start

1. Clone this repo.
2. In `config/config.go`, set `ProjectName` (this names the SQLite file:
   `<DB_DIR>/<ProjectName>.db`) and edit the `defaultTokens()` brand.
3. `make run` — runs the app at http://localhost:3000 with hot reload.

## Database

- Driver: `modernc.org/sqlite` (pure Go, cgo-free) — the only non-rio dependency.
- Migrations live in `database/migrations/NNNN_name.sql`, embedded in the binary
  and applied at startup (`database.MigrateUp`). Forward-only.
- `make db-reset` deletes the local dev database.

## Build & deploy

- `make tailwind` builds CSS (Tailwind v4; scans vendored rio/ui).
- `make build` builds the binary; `make build-docker` builds the image.
- Run the container with a persistent volume:
  `docker run -p 3000:3000 -v ./data:/data <image>`.
  Multiple products share one `/data` volume — each has its own
  `<ProjectName>.db`.
```

- [ ] **Step 2: Run the full verification**

```bash
go test ./...
make tailwind
docker build --build-arg BUILD_HASH=verify -t rio-template-verify .
docker run --rm -d -p 3000:3000 -v "$(pwd)/data:/data" --name rio-verify rio-template-verify
sleep 2
curl -sf localhost:3000/ | grep -q "Welcome to" && echo "home ok"
curl -sf -X POST -d "body=hello-from-curl" localhost:3000/messages -o /dev/null && echo "post ok"
curl -sf localhost:3000/messages | grep -q "hello-from-curl" && echo "persisted ok"
curl -sf localhost:3000/version | grep -q "BuildHash" && echo "version ok"
docker rm -f rio-verify
```
Expected: `home ok`, `post ok`, `persisted ok`, `version ok`.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: rewrite README for dom + db template"
```

---

## Self-Review

**Spec coverage:**
- dom instead of HTML templates → Tasks 6, 7 (views package, handlers render dom; templates/ deleted) ✓
- Database setup + usage + migrations, Docker-wired → Tasks 2–4 (Open, Migrate, Store), 7 (startup migrate), 8 (Dockerfile /data) ✓
- Drop fly.io → Task 7 (`fly.toml`), Task 8 (Makefile `deploy`) ✓
- rio/ui + tokens for UI/layout → Tasks 5 (Tokens in config), 6 (views compose ui) ✓
- One dependency (modernc) → Task 2 ✓
- Project-name-derived DB path → Task 5 (`DBPath`), Task 8 (`DB_DIR`/`/data`) ✓
- Tailwind v3 → v4 with vendored scan → Task 8 ✓
- Migration runner promotable (driver/embed-agnostic, isolated) → Task 3 ✓
- Go 1.22 → 1.26, rio v0.0.3 → v0.26.0 → Task 1 ✓
- Vendoring for hermetic build + Tailwind scan → Tasks 1, 7, 8 ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete code; every test step shows real assertions. ✓

**Type consistency:** `config.PageData`/`config.Meta`/`config.Config`/`config.New`/`config.DBPath`, `database.Open`/`Migrate`/`MigrateUp`/`Store`/`NewStore`/`Message`/`ListMessages`/`CreateMessage`, `views.Page`/`Home`/`About`/`PrivacyPolicy`/`NotFound`/`Messages`, handler names `HandleHome`/`HandleMessages`/`HandleAbout`/`HandlePrivacyPolicy`/`HandleVersion`/`HandleStatic` — used consistently across Tasks 5–7. `Conf` (package main) built via `config.New(BuildEnv)`. ✓

**Notes for the implementer:**
- The module is `app`; keep imports as `app/config`, `app/database`, `app/views`.
- `views` test uses an inline `Render(*bytes.Buffer)` adapter because `dom.Node.Render` takes an `io.Writer` — `*bytes.Buffer` satisfies it.
- `ui.Button` hardcodes `type="button"`; the message form uses the local `submitButton` helper instead. If submit buttons recur across products, that's the rule-of-three signal to add a type option to `rio/ui`.
- `database/migrate.go` must remain free of product imports and the driver, so it can later be promoted to `rio/migrate` after a second product proves the API.
