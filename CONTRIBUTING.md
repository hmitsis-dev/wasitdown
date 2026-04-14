# Contributing to wasitdown.dev

Thanks for your interest in contributing. The two most common contributions are **adding a new provider** and **fixing a bug**.

---

## Getting Started

1. Fork the repo and clone it locally.
2. Choose a local setup option (see below).
3. Make your changes, test locally, then open a pull request against `main`.

### Option A — Docker (fully local, no external accounts needed)

```bash
cp .env.example .env
docker compose -f docker-compose.local.yml up -d
# Site runs at http://localhost:8080
```

Spins up a local Postgres, scraper, generator, and nginx — no Neon account needed.

### Option B — Go + Neon (lighter, no Docker needed)

Sign up for a free [Neon](https://neon.tech) account, create a project, then:

```bash
export DATABASE_URL="your-neon-connection-string"
go run ./cmd/scraper    # fetch incidents into your DB
go run ./cmd/generator  # build the site into dist/
cd dist && python3 -m http.server 8080
# Site runs at http://localhost:8080
```

> **Note:** Use your own Neon project — never point `DATABASE_URL` at the production database.

---

## Adding a Provider

This is the most welcome contribution. Before opening a PR:

- Check existing providers in `internal/scraper/provider.go` — the slug must be unique.
- Confirm the provider has a public, machine-readable status page.
- If it's a standard [Atlassian Statuspage](https://www.atlassian.com/software/statuspage), no custom code is needed — just a slug and a DB row.
- If it uses a custom format (RSS, proprietary JSON, etc.), implement the `Provider` interface in a new file under `internal/scraper/`.

See [Adding a Provider](README.md#adding-a-provider) in the README for the full walkthrough.

**Please include in your PR:**
- The migration SQL added as a new file under `db/migrations/`
- The `AllProviders()` entry
- A note on what format the status page uses and the correct `status_page_url` for the human-readable page

---

## Changing Templates or the Generator

The static site is built by `cmd/generator` from HTML templates in `templates/`. A few things to know:

- **Each template is isolated.** `base.html` is cloned per page so `{{define "head-extra"}}` and `{{define "content"}}` don't collide across pages. Don't use ParseGlob.
- **Template functions** are registered in `generator.New()` via `template.FuncMap`. Add new helpers there, not inline in templates.
- After changing a template, rebuild and restart the generator: `docker compose up generator`

---

## Changing DB Queries

All queries live in `internal/db/queries.go`. Rules:

- Use `pgx` directly — no ORM.
- Never modify existing migration files. Add a new `NNN_description.sql` under `db/migrations/`.
- The generator runs once at deploy time, so expensive queries are acceptable; just explain the tradeoff in the PR.

---

## Bug Fixes & Other Changes

- Keep PRs focused — one fix or feature per PR.
- Run the scraper and generator locally to confirm nothing is broken before submitting.
- Handle all errors explicitly — no `_` ignores.

---

## Code Style

- No global state — pass dependencies explicitly.
- All DB queries go in `internal/db/queries.go`.
- Errors are logged with `slog`, never panic'd.
- The generator must remain idempotent — safe to run multiple times.
- Use context for cancellation and deadlines.

---

## Opening an Issue

Not ready to contribute code? [Open an issue](../../issues) — provider requests and bug reports are both welcome. Use the issue templates.
