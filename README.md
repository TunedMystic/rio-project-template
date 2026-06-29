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

## Build & deploy

- `make tailwind` builds CSS (Tailwind v4; scans vendored rio/ui).
- `make build` builds the binary; `make build-docker` builds the image.
- Run the container with a persistent volume:
  `docker run -p 3000:3000 -v ./data:/data <image>`.
  Multiple products share one `/data` volume — each has its own
  `<ProjectName>.db`.
