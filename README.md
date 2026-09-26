# 📒 Budget Book

[![Test](https://github.com/steled/budget-book/actions/workflows/test.yaml/badge.svg)](https://github.com/steled/budget-book/actions/workflows/test.yaml)
[![Latest Tag](https://img.shields.io/github/v/tag/steled/budget-book?label=release)](https://github.com/steled/budget-book/tags)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

A lightweight, single-user personal household budget book. Go + SQLite,
compiled into a single static binary with all templates and static assets
embedded — no external dependencies at runtime beyond the SQLite file.

## Features

- **Accounts** — track balances across multiple accounts (checking, cash,
  savings, ...); every transaction belongs to exactly one account.
- **Categories** — a predefined set of income/expense categories, each with
  a fixed display color, extendable with your own.
- **Transactions** — plain income/expense bookings, freely editable
  afterwards regardless of how they were created.
- **Recurring bookings** — templates (amount, category, account, interval)
  that book themselves automatically. A daily in-process scheduler (no
  external cron) catches up on any occurrences missed while the app was
  offline.
- **Übersicht** — total balance, per-account tiles, and a combined,
  day-grouped transaction list.
- **Monat** — income vs. expense for a given month, with a ratio bar and
  month navigation.
- No budgets/limits — this is pure recording, not budgeting.

## Tech Stack

- Go (stdlib `net/http` with method-pattern routing, `html/template`), no
  web framework
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- `golang.org/x/crypto/bcrypt` for password hashing
- Vanilla JS/CSS frontend — no build step, no bundler, no frontend
  dependencies; dark mode via `prefers-color-scheme` (with a manual toggle,
  persisted in `localStorage`)
- Single static binary via `go:embed` (templates + static assets baked in)

## Authentication

Single-user, environment-variable-configured:

- `APP_PASSWORD` may be a plaintext password (bcrypt-hashed in memory at
  startup) or an already bcrypt-hashed value (e.g. from
  `htpasswd -bnBC 12 "" '<password>'`), so a plaintext password never has
  to be stored at rest in a Secret. Either way, login compares via
  `bcrypt.CompareHashAndPassword`.
- Sessions are a signed cookie: `base64url(JSON{exp}) + "." +
  base64url(HMAC-SHA256(secret, payload))`, `HttpOnly`, `SameSite=Lax`,
  `Secure` when `APP_SECURE_COOKIES=true` or the request is already TLS.
- Login is rate-limited per client IP (5 attempts / 15 minutes, in-memory).

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `APP_USERNAME` | no | `admin` | Login username |
| `APP_PASSWORD` | **yes** | – | Login password: a bcrypt hash (recommended, e.g. via `htpasswd -bnBC 12 "" '<password>'`) or a plaintext password, hashed with bcrypt at startup |
| `APP_SESSION_SECRET` | **yes** | – | HMAC signing secret, ≥32 characters (`openssl rand -hex 32`) |
| `APP_SECURE_COOKIES` | no | `false` | Set `true` behind a TLS-terminating proxy to force the `Secure` cookie flag |
| `DATABASE_PATH` | no | `/data/budget-book.db` | SQLite database file path |
| `APP_ADDR` | no | `:8080` | HTTP listen address |

## API Reference

All `/api/*` routes and `/` require an authenticated session; unauthenticated
API requests get a JSON `401`.

| Method | Path | Description |
|---|---|---|
| GET | `/api/overview` | Total balance, account balances, combined transaction list |
| GET | `/api/month?year=&month=` | Income/expense/balance for one calendar month (defaults to current) |
| GET/POST | `/api/accounts` | List / create accounts |
| PUT/DELETE | `/api/accounts/{id}` | Update / delete an account (409 if still referenced) |
| GET/POST | `/api/categories` | List / create categories |
| PUT/DELETE | `/api/categories/{id}` | Update / delete a category (409 if still referenced) |
| GET/POST | `/api/transactions` | List (`?limit=`) / create transactions |
| PUT/DELETE | `/api/transactions/{id}` | Update / delete a transaction |
| GET/POST | `/api/recurring` | List / create recurring templates |
| PUT/DELETE | `/api/recurring/{id}` | Update / delete a recurring template |
| GET | `/healthz` | Liveness probe, no auth |

## Development

```bash
APP_PASSWORD=dev-password \
APP_SESSION_SECRET=$(openssl rand -hex 32) \
DATABASE_PATH=./dev.db \
go run .
```

```bash
go test ./...
```

## Docker

```bash
docker build -f docker/Dockerfile -t budget-book .
docker run -p 8080:8080 \
  -e APP_PASSWORD=changeme \
  -e APP_SESSION_SECRET=$(openssl rand -hex 32) \
  -v budget-book-data:/data \
  budget-book
```

## Helm

```bash
helm install budget-book oci://ghcr.io/steled/charts/budget-book \
  --version 0.1.5 \
  --set auth.password='<your-password>' \
  --set auth.sessionSecret="$(openssl rand -hex 32)"
```

See [`helm/budget-book/README.md`](helm/budget-book/README.md) for the
full values reference.

## CI/CD

- **🧪 Test** (`test.yaml`) — Dockerfile lint (hadolint), `golangci-lint`,
  Helm chart lint/template (both gateway and ingress networking modes),
  `go test -race` with coverage.
- **📝 Commitlint** (`commitlint.yaml`) — enforces Conventional Commits on
  every PR.
- **🏷️ Auto Release** (`auto-release.yaml`) — on push to `main`, derives the
  next semver from Conventional Commit subjects and tags `vX.Y.Z`.
- **🚀 Release** (`release.yaml`) — on tag push, builds and pushes the
  multi-arch Docker image and the Helm chart (OCI) to GHCR, and creates a
  GitHub Release with a generated changelog.
- **🤖 Renovate** (`renovate.yaml`) — weekly dependency updates.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/):
`feat:`/`fix:`/`perf:` drive minor/patch releases, `!:`/`BREAKING CHANGE`
drives a major release, `chore:`/`docs:`/`ci:`/`refactor:` do not release.
