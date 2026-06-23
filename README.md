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
