# rio project template

A starter for rio products: rio + rio/dom (no HTML templates) + rio/ui themed
components + SQLite with built-in migrations, in a scratch Docker image.

## Quick start

1. Clone this repo.
2. In `config/config.go`, set `ProjectName` (this names the SQLite file:
   `<DB_DIR>/<ProjectName>.db`) and edit the `defaultTokens()` brand.
3. Copy `.env.example` to `.env` and fill in values as needed.
4. `make run` — runs the app at http://localhost:3000 with hot reload.

## Database

- Driver: `modernc.org/sqlite` (pure Go, cgo-free) — the only non-rio dependency.
- Migrations live in `database/migrations/NNNN_name.sql`, embedded in the binary
  and applied at startup (`database.MigrateUp`). Forward-only.
- `make db-reset` deletes the local dev database.

## Accounts & auth

Email magic-link login + a tabbed account area (`/account`). Config via env:

| Env | Purpose | Default |
|-----|---------|---------|
| `APP_SECRET` | HMAC key for CSRF/signing | dev fallback (prod: **required**) |
| `BASE_URL` | Absolute base for magic-link URLs | `http://localhost:<port>` |
| `POSTMARK_TOKEN` | Postmark server token | unset → links logged to console |
| `EMAIL_FROM` | From address | `noreply@localhost` |
| `GOOGLE_CLIENT_ID` | Google OAuth client id | unset → Google login hidden |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret | unset → Google login hidden |
| `TRUST_PROXY` | Honor X-Forwarded-For for client IP (set behind a trusted proxy) | unset → use the socket peer IP |
| `SESSION_CLEANUP_INTERVAL` | Interval to prune expired sessions (`0` disables) | `1h` |
| `TOKEN_CLEANUP_INTERVAL` | Interval to prune expired login tokens (`0` disables) | `1h` |
| `ERROR_WEBHOOK_URL` | POST JSON error events here (Sentry/Slack/any); unset disables | unset |
| `ADMIN_EMAILS` | Comma-separated admin emails for `/admin` (empty disables) | unset |

In dev with no `POSTMARK_TOKEN`, the magic link is printed to the server log —
click it from your terminal. In production, set all four (`APP_SECRET` is
mandatory; the app refuses to start without it).

**Google login:** create an OAuth 2.0 Client (type "Web application") in Google
Cloud Console and register the redirect URI `<BASE_URL>/auth/google/callback`
(e.g. `http://localhost:3000/auth/google/callback` in dev). Set both env vars to
enable the "Continue with Google" button; leave them unset and the app falls
back to email magic-link only.

## Billing (Stripe)

Subscriptions and one-time purchases via Stripe Checkout + Billing Portal. Config via env:

| Env | Purpose | Default |
|-----|---------|---------|
| `STRIPE_SECRET_KEY` | Stripe secret key (enables billing) | unset → billing disabled |
| `STRIPE_WEBHOOK_SECRET` | Webhook signing secret | unset → webhook rejects events |
| `STRIPE_PRICE_PRO` | Price ID for the `pro` subscription | unset → Subscribe hidden |
| `STRIPE_PRICE_EBOOK` | Price ID for the `ebook` one-time product | unset → Buy hidden |

Edit the `Products` catalog in `config/config.go` to add/rename products (each is
`{Key, Name, Kind, PriceID}`; `Kind` is `Subscription` or `OneTime`). Gating:
`/premium` requires an active subscription; `/guide` requires owning `ebook`.

**Local webhooks:** run `stripe listen --forward-to localhost:3000/webhooks/stripe`
and put the printed `whsec_...` in `STRIPE_WEBHOOK_SECRET`. Use Stripe **test mode**
keys/prices; the live consent round-trip can't be automated.

## Admin

An auth-gated back-office at `/admin` (redirects to `/admin/users`). Set
`ADMIN_EMAILS` to a comma-separated allowlist; those users get a searchable,
paginated user list and a per-user detail page where you can grant/revoke an
entitlement (comp an account) and revoke a user's sessions. Non-admins receive a
404 (the surface is not advertised); leaving `ADMIN_EMAILS` empty disables it.

## Theming

The template ships with two compile-time theme presets, selected by one value in
`config.New`:

- **`ThemeSlateIndigo`** (default) — a light, indigo-on-slate look.
- **`ThemeDusk`** — a dark, cyan-on-navy look.

To switch, set the `Theme` field in `config.New`:

```go
Theme: ThemeDusk, // in config/config.go, inside New(...)
```

The selected preset builds `Config.Tokens` (the `:root` CSS variables every
component reads) plus a small set of extended variables (`--color-ring`,
`--color-on-danger`, `--color-surface-raised`, `--chart-1..--chart-4`) emitted by
the layout. After changing the theme, rebuild the CSS with `make tailwind`.

Every component in the system is shown under the active theme at **`/kit`** — use
it as the live component reference. Components are token-driven only, so both
presets render correctly without per-theme code.

## Build & deploy

- `make tailwind` builds CSS (Tailwind v4; scans vendored rio/ui).
- `make build` builds the binary; `make build-docker` builds the image.
- Run the container with a persistent volume:
  `docker run -p 3000:3000 -v ./data:/data <image>`.
  Multiple products share one `/data` volume — each has its own
  `<ProjectName>.db`.

## Backups

Data lives in a single SQLite file. Use [Litestream](https://litestream.io) for
continuous, offsite backups with point-in-time restore — see
[docs/deploy/litestream.md](docs/deploy/litestream.md) plus the root
`litestream.yml` and `docker-compose.yml` examples.

Expired sessions and login tokens are pruned automatically by a background
scheduler (intervals via `SESSION_CLEANUP_INTERVAL` / `TOKEN_CLEANUP_INTERVAL`;
set `0` to disable).
