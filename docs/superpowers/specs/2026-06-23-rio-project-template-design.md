# rio-project-template Rewrite — Design

**Date:** 2026-06-23
**Status:** Approved, ready for implementation planning
**Target repo:** `github.com/TunedMystic/rio-project-template` (rewritten in place)

## Goal

Rewrite the existing `rio-project-template` starter so a new product can be spun
up fast with: rio for the web layer, `rio/dom` for rendering (no HTML
templates), `rio/ui` for themed components, SQLite for storage with a built-in
migration system, and a `scratch` Docker image. The whole product adds exactly
**one** Go dependency beyond rio: `modernc.org/sqlite`.

## Context: the existing template

The current `rio-project-template` (read during brainstorming) provides:

- Two-stage Dockerfile → `FROM scratch`, `CGO_ENABLED=0`, upx-compressed static
  binary, `EXPOSE 3000`.
- Self-documenting Makefile: `run` (watchexec hot reload), `build`, `deploy`
  (fly.io), `clean`, Docker targets, vendored `bin/tailwind`, `watchexec`,
  `cwebp`.
- `config.go`: a `Config` struct (SiteName, links, meta, Image) + `RenderData` +
  `embed.FS` for `static` and `templates`. Build-time ldflags
  (`BuildHash`/`BuildDate`/`BuildEnv`).
- `main.go`: `rio.Templates(...)`, `rio.NewServer()`, `s.Handle(...)`,
  `s.Serve(Conf.Addr)`. Routes: `/`, `/static/`, `/version`, `/about`,
  `/privacy-policy`.
- `handlers.go`: `rio.Render(w, "index", ...)` against HTML templates,
  `rio.MakeHandler`, `rio.FileServer`, `rio.Json200`, `rio.CacheControlWithAge`.
- Tailwind **v3** (`tailwind.config.js`).
- **No database.**

## Scope

Rewrite in place. Keep the Docker/Makefile/config DNA; replace rendering, add a
database, drop fly.io, upgrade Tailwind.

| Keep | Change | Drop |
|---|---|---|
| 2-stage → `scratch` Docker, `CGO_ENABLED=0`, upx | HTML templates → `dom` + `rio/ui` views | `fly.toml` |
| Self-documenting Makefile, vendored `bin/tailwind`, hot reload | Tailwind v3 → v4 | `deploy:` (fly.io) Makefile target |
| `config.go` as the product seam | Add `ui.Tokens` to config | `templates/` dir |
| One-dependency ethos | Add one dep: `modernc.org/sqlite` | `tailwind.config.js` |
| build-time ldflags; `/version`, `/about`, `/privacy-policy`, `404` | Routes render via dom; Go 1.22 → 1.26 | `rio.Templates` / `rio.Render` usage |

### Out of scope (deferred)

- Deployment/hosting config (was fly.io). The boundary is `docker build` /
  `docker run`; how it's hosted is the product's concern.
- Product-specific pages (pricing, account settings, auth). These are
  copy-and-own per product, not template furniture.
- Promoting `nav`/`footer`/the migration runner into `rio`/`rio/ui` — they are
  built here first; promotion waits for the rule of three (and, for the
  migration runner, a second product).

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Repo target | Rewrite `rio-project-template` in place | User choice; one canonical template. |
| Rendering | `rio/dom` view functions, no HTML templates | User requirement. |
| Components | `rio/ui` + a `ui.Tokens` brand literal | User requirement; reuse, not from-scratch dom. |
| SQLite driver | `modernc.org/sqlite` (pure Go) | Existing Dockerfile is `CGO_ENABLED=0` + scratch; cgo `mattn` would break it. One go.mod line. |
| SQL access | raw SQL over `database/sql` (stdlib) | No ORM/builder dependency; queries are readable. |
| Migrations | home-grown runner, stdlib only | Avoids golang-migrate/goose deps; ~40 lines. |
| Migration runner shape | `Migrate(db *sql.DB, files fs.FS) error`, isolated file | Driver- and embed-agnostic → promotable to `rio/migrate` unchanged. |
| DB demo | one example table + page (minimal working example) | Exercises Open+migrate+store+view end-to-end and is testable, without product-specific screens. |
| DB name | derived from `Config.ProjectName` → `<dataDir>/<ProjectName>.db` | Multiple projects co-host on one droplet under a shared `/data` volume; each needs a unique db file. The project name is the single per-clone seam. |
| Tailwind | v4, standalone binary, vendored scanning | rio/ui is v4; vendoring resolves the module-cache scan caveat. |
| Go version | 1.26 | Match the rio library bump. |

## Architecture

### Repo structure (after rewrite)

```
rio-project-template/
  main.go              // open db -> migrate -> store -> server -> routes
  config.go            // Config (+ DBPath, + ui.Tokens), Meta/RenderData
  handlers.go          // handlers return dom nodes; one is db-backed
  go.mod               // rio + modernc.org/sqlite
  go.sum
  Dockerfile           // scratch, CGO_ENABLED=0, Go 1.26, SQLite volume
  Makefile             // run/build/clean/docker/tailwind + db-reset; no fly
  tailwind.input.css   // v4: @import + @source (incl. vendored rio/ui)
  vendor/              // go mod vendor — lets Tailwind scan rio/ui source
  database/
    database.go        // Open(path) (*sql.DB, error) + pragmas
    migrate.go         // Migrate(db, fs.FS) error — generic, promotable
    migrations.go      // //go:embed migrations/*.sql + MigrateUp(db)
    migrations/
      0001_init.sql    // creates the messages example table
    store.go           // Store: ListMessages / CreateMessage (raw SQL)
  views/
    layout.go          // Page shell: doctype/head (StyleVars) + nav + footer
    pages.go           // Home, About, PrivacyPolicy, NotFound
    components.go      // nav(), footer() — inline, copy-and-own
  static/
    css/styles.css     // tailwind output (gitignored, built)
    img/...            // meta image etc.
```

### Database layer

**`database.go`**
```go
func Open(path string) (*sql.DB, error)
```
Opens via `sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")` (modernc
registers the driver name `sqlite`), then sets `journal_mode=WAL`,
`foreign_keys=ON`, `synchronous=NORMAL`, and `SetMaxOpenConns(1)` (simplest
correct default for SQLite writes; tune later). Driver imported as
`_ "modernc.org/sqlite"`.

**`migrate.go`** — the promotable runner.
```go
func Migrate(db *sql.DB, files fs.FS) error
```
- Creates `schema_migrations(name TEXT PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`.
- `fs.Glob(files, "migrations/*.sql")`, sort by name (`0001_`, `0002_`, …).
- For each not already in `schema_migrations`: run the file and the tracking
  insert inside one transaction; rollback on error with a wrapped message.
- Forward-only. Idempotent: re-running applies nothing.
- Imports only `database/sql`, `io/fs`, `sort`, `fmt`. **No product imports, no
  embed, no driver** — isolated so promotion to `rio/migrate` is a file move.

**`migrations.go`** — keeps the embed product-side (embed paths are
source-relative):
```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

func MigrateUp(db *sql.DB) error { return Migrate(db, migrationsFS) }
```

**`migrations/0001_init.sql`**
```sql
CREATE TABLE messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    body       TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**`store.go`**
```go
type Message struct { ID int64; Body string; CreatedAt time.Time }
type Store struct { db *sql.DB }
func NewStore(db *sql.DB) *Store
func (s *Store) ListMessages(ctx context.Context) ([]Message, error)
func (s *Store) CreateMessage(ctx context.Context, body string) error
```
Raw `database/sql` queries. Demonstrates the read/write path end to end.

Migrations run at **startup** in `main.go` via `database.MigrateUp(db)`. The
embedded `.sql` ships inside the static binary, so the container self-migrates
on boot — no separate migration step in Docker.

### Views + rio/ui integration

- `views.Page(cfg Config, meta Meta, body ...dom.Node) dom.Node` builds the full
  document: `dom.Doctype(dom.Html(dom.Head(cfg.Tokens.StyleVars(), <meta>,
  <css link>), dom.Body(<body classes>, nav(cfg), body..., footer(cfg))))`. The
  body carries the `font-family` + background/text token classes (the lesson
  from the rio/ui gallery: something must consume `--font-family`).
- Page builders (`Home`, `About`, `PrivacyPolicy`, `NotFound`) compose
  `ui.Container`, `ui.Heading`, `ui.Text`, `ui.Card`, `ui.Button`, etc.
- A dedicated `/messages` page (keeping home as a clean landing page products
  can reshape) lists `messages` via `ui` components and posts a new one through
  a `ui` form (`ui.TextField` + `ui.Button`) to a handler that calls
  `store.CreateMessage`. A product deletes this one page to drop the demo.
- `nav()` / `footer()` live inline in `views/components.go` — copy-and-own;
  promote to `rio/ui` only after the rule of three.
- `config.go` gains `Tokens ui.Tokens` (the brand) alongside `SiteName`/links,
  plus a `ProjectName string` (the per-clone seam) that derives the SQLite path.
  `RenderData` is slimmed into a small per-page `Meta` (title, description,
  heading, page URL) since dom view functions take what they need directly.

#### Project name → database path

A product is cloned, the developer sets one field — `ProjectName` (e.g.
`"RioProg"`) — and the database file name follows from it. This keeps every
project's SQLite db unique so several can co-host on one droplet under a shared
`/data` volume.

```go
// config.go
c.ProjectName = "RioProg"        // <-- the per-clone seam
dataDir := os.Getenv("DB_DIR")   // container sets /data
if dataDir == "" {
    dataDir = "/data"            // prod default
    if c.Debug {
        dataDir = "."            // dev: ./RioProg.db in the project root
    }
}
c.DBPath = filepath.Join(dataDir, c.ProjectName+".db") // /data/RioProg.db
```

- `ProjectName` is the source of truth for the db file name (a slug-like
  identifier, distinct from the human-facing `SiteName`).
- `DB_DIR` env chooses the directory only: `/data` in the container, project
  root in dev. No `DB_PATH` full-path override — the derived path is the model.
- Three projects on one droplet → `/data/RioProg.db`, `/data/Other.db`,
  `/data/Third.db`, each isolated, all under the same mounted `/data` volume.

### Tailwind v4 and the module-cache scan caveat

`tailwind.input.css`:
```css
@import "tailwindcss";
@source "./**/*.go";
@source "./vendor/**/*.go";
```
rio/ui is a **dependency**, so its `bg-[var(--…)]` class literals live in the Go
module cache, which the Tailwind scanner won't reach by default — components
would render unstyled. Resolution: **`go mod vendor`** and scan the vendored
`rio/ui` source. The `make tailwind` target runs `go mod vendor` first; the
build stays hermetic and the components render styled. (This is the rio/ui spec
§8 module-cache caveat, resolved by vendoring.) `vendor/` is committed.

### Docker / Makefile

- **Dockerfile:** bump `golang:1.22.1-alpine` → a 1.26 alpine; keep
  `CGO_ENABLED=0`, `FROM scratch`, upx, `EXPOSE 3000`. SQLite directory from
  `DB_DIR` env (default `/data`); the file name derives from `ProjectName`
  (`/data/<ProjectName>.db`). Document `-v ./data:/data` for persistence — a
  shared volume that holds each project's distinct db file. With `go mod
  vendor`, the build uses `-mod=vendor`.
- **`.dockerignore`:** exclude `*.db`, `data/`.
- **Makefile:** drop `deploy` (fly.io); bump `bin/tailwind` download to a pinned
  v4 release; update the tailwind invocation (no `--config`); add `db-reset`
  (delete the local db file); keep `run`/`build`/`clean`/docker targets +
  `watchexec` hot reload. `cwebp` retained (dev-only, not a Go dep).

## Testing

- **`migrate.go`:** test with `fstest.MapFS` (proves driver/embed independence);
  apply against a temp SQLite db, then re-run and assert nothing new is applied
  (idempotence); assert ordering and the `schema_migrations` contents.
- **`Open`:** opens a temp file db and the expected pragmas are in effect
  (e.g. `PRAGMA foreign_keys` returns 1).
- **`Store`:** `CreateMessage` then `ListMessages` round-trips against a temp db.
- **Views:** render to a buffer, assert HTML substrings (rio/ui test style),
  including that `StyleVars()` and token classes appear.
- **Handlers:** `httptest` for status codes and body content; the db-backed
  handler against a migrated temp db.

## Build order

1. `go.mod`: add `modernc.org/sqlite`, bump Go to 1.26.
2. `database/`: `Open` + pragmas; `Migrate` runner (+ `fstest` tests);
   `migrations.go` + `0001_init.sql`; `store.go` (+ round-trip tests).
3. Remove `fly.toml`, `templates/`, `tailwind.config.js`.
4. Tailwind v4: `tailwind.input.css` with vendored `@source`; `go mod vendor`;
   update `make tailwind`.
5. `views/`: `layout.go` (Page shell + `StyleVars` + token body classes),
   `pages.go`, `components.go`; add `Tokens` + `ProjectName` (deriving `DBPath`
   via `DB_DIR`) to `config.go`.
6. `handlers.go`: render via dom; add the `/messages` db-backed page (GET list +
   POST create) plus the static/home/about/privacy/404/version handlers.
7. `main.go`: `Open` → `MigrateUp` → `NewStore` → server → routes.
8. Dockerfile (Go 1.26, `DB_PATH`, vendor) + Makefile (drop `deploy`, v4
   tailwind, `db-reset`) + `.dockerignore`.
9. README update; full verify: `go test ./...`, `docker build`, `docker run`,
   load the pages, exercise the db-backed page.

## Success criteria

- `go test ./...` passes; the migration runner is proven generic via `fstest`.
- One product = one direct Go dependency (`modernc.org/sqlite`); rio provides the
  rest of the web/UI layer.
- `docker build` produces a `scratch` image; `docker run` boots, self-migrates,
  serves all pages, and the db-backed page reads + writes `messages`.
- Changing `ProjectName` changes the db file (`/data/<ProjectName>.db`), so
  multiple projects co-host on one droplet's `/data` volume without collision.
- rio/ui components render **styled** (Tailwind sees vendored class literals).
- No fly.io, no HTML templates, no `tailwind.config.js` remain.
- `database/migrate.go` is isolated and driver/embed-agnostic — ready to promote
  to `rio/migrate` after a second product.
