# wasitdown.dev

[![Lint](https://github.com/hmitsis-dev/wasitdown/actions/workflows/lint.yml/badge.svg)](https://github.com/hmitsis-dev/wasitdown/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Was it down? Historical cloud & AI provider incident tracker.

Aggregates incident history from public status pages across major cloud and AI providers. A scraper collects data into PostgreSQL, and a generator builds a fully static site deployed to Cloudflare Pages — no server-side runtime.

---

## What it tracks

AWS, Azure, Google Cloud, Anthropic, OpenAI, Cloudflare, GitHub, Vercel, Groq, Discord, Zoom, and more. View incidents by provider, by date, or compare providers side by side.

---

## Features

- **Provider Status Grid** — 30-day uptime at a glance, filterable by name
- **Chaos Buddies** — provider pairs that go down within 2 hours of each other most often (cross-provider correlation)
- **Uptime Overview** — sortable table with 30/90/365-day uptime, incident count, and average duration per provider
- **Provider pages** — full incident history, uptime stats, and compare links per provider
- **Incident pages** — full timeline with status updates
- **Date pages** — all incidents on a given day with concurrent-outage grouping
- **Compare pages** — side-by-side uptime and incident comparison for any two providers
- **JSON API** — `/api/v1/providers.json`, `/api/v1/recent.json`, `/api/v1/uptime.json`

---

## Architecture

```
GitHub Actions (cron)
  └─ every 15min → scraper → PostgreSQL (Neon / Supabase / Railway)
  └─ every 20min → generator → dist/ → Cloudflare Pages
```

All status page polling is outbound HTTP GET only. The site is pure static HTML — no backend at runtime.

---

## Running Locally

**Prerequisites:** Go 1.22+ and either Docker or a free [Neon](https://neon.tech) account.

### Option A — Docker (no external accounts needed)

```bash
git clone https://github.com/hmitsis-dev/wasitdown
cd wasitdown
cp .env.example .env
docker compose -f docker-compose.local.yml up -d
# Open http://localhost:8080
```

Starts a local Postgres, scraper, generator, and nginx all in one command.

### Option B — Go + Neon (no Docker needed)

```bash
git clone https://github.com/hmitsis-dev/wasitdown
cd wasitdown
export DATABASE_URL="your-neon-connection-string"
go run ./cmd/scraper      # fetch incidents
go run ./cmd/generator    # build dist/
cd dist && python3 -m http.server 8080
# Open http://localhost:8080
```

> Use your own free Neon project — never point at the production database.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_PASSWORD` | `changeme` | PostgreSQL password (Docker only) |
| `DATABASE_URL` | _(local DSN)_ | Full Postgres connection string |
| `OUTPUT_DIR` | `dist` | Generator output directory |
| `TEMPLATES_DIR` | `templates` | HTML template directory |
| `STATIC_DIR` | `static` | Static assets directory |

---

## Adding a Provider

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full walkthrough. The short version:

1. **Add a DB row** in a new migration file under `db/migrations/`:

```sql
INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
  ('My Provider', 'myprovider', 'https://status.myprovider.com',
   'https://status.myprovider.com/api/v2/incidents.json', 'statuspage')
ON CONFLICT (slug) DO NOTHING;
```

2. **Wire up the scraper** — for standard [Atlassian Statuspage](https://www.atlassian.com/software/statuspage) providers, add the slug to `AllProviders()` in `internal/scraper/provider.go`:

```go
&StatuspageScraper{slug: "myprovider"},
```

For custom formats, implement the `Provider` interface in a new file under `internal/scraper/`.

3. **Re-run** the scraper and generator.

---

## Deploying

### GitHub Secrets required

| Secret | Description |
|---|---|
| `DATABASE_URL` | Postgres DSN |
| `CLOUDFLARE_API_TOKEN` | CF API token with Pages:Edit |
| `CLOUDFLARE_ACCOUNT_ID` | Your Cloudflare account ID |

Push to `main` — the GitHub Actions workflows handle the rest.

---

## Tech Stack

- **Go 1.22+** — scraper + static site generator
- **PostgreSQL** — incident storage (Neon free tier recommended)
- **Tailwind CSS** (CDN) — styling
- **nginx** — local static file serving
- **Cloudflare Pages** — production hosting
- **GitHub Actions** — cron scheduling

---

## Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

**Using an AI agent?** Copy [`llms.md`](llms.md) to your tool's config file — `CLAUDE.md` for Claude Code, `AGENTS.md` for OpenAI Codex, `.cursorrules` for Cursor.

## License

See [LICENSE](LICENSE).
