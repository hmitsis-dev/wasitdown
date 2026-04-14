# wasitdown.dev — Developer & AI Agent Guide

Historical cloud & AI provider incident aggregator. Scrapes public status pages, stores to PostgreSQL, generates a fully static site deployed to Cloudflare Pages.

---

## Project Structure

```
wasitdown/
  cmd/
    scraper/main.go       — entry point: runs all provider scrapers
    generator/main.go     — entry point: reads DB, writes dist/
  internal/
    db/
      db.go               — pgxpool connection + migration runner
      queries.go          — all DB queries (pgx, no ORM)
    models/
      models.go           — shared types (Provider, Incident, etc.)
    scraper/
      provider.go         — Provider interface + StatuspageScraper + AllProviders()
      gcp.go              — Google Cloud custom JSON format
      azure.go            — Azure RSS feed
      aws.go              — AWS Health JSON (UTF-16 encoded)
      runner.go           — RunAll() orchestrator
    generator/
      generator.go        — Generator.Run() → builds dist/ HTML + JSON
  templates/
    base.html             — shared layout (header, footer, Tailwind)
    index.html            — dashboard (status grid, chaos buddies, uptime table)
    provider.html         — per-provider incident history
    date.html             — cross-provider view for a given date
    incident.html         — full incident timeline
    compare.html          — side-by-side provider comparison
  static/
    css/custom.css        — minimal custom styles beyond Tailwind
    js/main.js            — vanilla JS (no framework)
    robots.txt
  db/
    migrations/           — plain SQL files, applied in order on scraper startup
  Dockerfile.scraper
  docker-compose.local.yml — postgres + scraper + generator + nginx (local dev)
  .env.example
  .github/
    workflows/
      scraper.yml         — cron: every 15min → runs scraper once
      generate.yml        — cron: every 20min → generates site + deploys to CF Pages
      lint.yml            — golangci-lint on push/PR to main
```

---

## Architecture

```
GitHub Actions (cron)
  └─ scraper.yml  → cmd/scraper  → PostgreSQL (Neon free tier)
  └─ generate.yml → cmd/generator → dist/ → Cloudflare Pages

All status page polling is outbound HTTP GET only.
The static site has zero server-side runtime — pure CDN delivery.
```

---

## Running Locally

**Prerequisites:** Go 1.24+ and Docker (or a free [Neon](https://neon.tech) account).

### Option A — Docker

```bash
cp .env.example .env
docker compose -f docker-compose.local.yml up -d
# Open http://localhost:8080
```

### Option B — Go + Neon (no Docker)

```bash
export DATABASE_URL="your-neon-connection-string"
go run ./cmd/scraper      # fetch incidents
go run ./cmd/generator    # build dist/
cd dist && python3 -m http.server 8080
```

---

## Adding a Provider

### Step 1 — DB migration

Create `db/migrations/NNN_add_myprovider.sql`:

```sql
INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
  ('My Provider', 'myprovider', 'https://status.myprovider.com',
   'https://status.myprovider.com/api/v2/incidents.json', 'statuspage')
ON CONFLICT (slug) DO NOTHING;
```

### Step 2 — Scraper

**Standard Atlassian Statuspage** — add slug to `AllProviders()` in `internal/scraper/provider.go`:

```go
&StatuspageScraper{slug: "myprovider"},
```

**Custom format** — implement the `Provider` interface in a new file:

```go
type MyProviderScraper struct{}
func (s *MyProviderScraper) Slug() string { return "myprovider" }
func (s *MyProviderScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error { ... }
```

### Step 3 — Re-run

```bash
go run ./cmd/scraper
go run ./cmd/generator
```

---

## Database Migrations

Plain SQL files in `db/migrations/`, named `NNN_description.sql`. Applied in filename order on scraper startup via `internal/db/db.go`. Never edit applied migrations — always add new files.

---

## Code Conventions

- **Go 1.24+**, stdlib where possible. No ORM — `pgx` directly.
- **No global state** — all dependencies passed explicitly.
- **Error handling** — explicit everywhere; no naked `_` ignores.
- **Context** — use for cancellation and deadlines throughout.
- **Provider interface** — each scraper is an independent struct. Errors logged to `scrape_log`, never panic.
- **DB layer** — all queries in `internal/db/queries.go`.
- **Generator** — idempotent; safe to re-run. Uses `os.Create` (overwrites).
- **Templates** — `html/template` with a func map registered in `generator.New()`.
- **Tests** — table-driven.

---

## GitHub Actions — Required Secrets

| Secret | Description |
|---|---|
| `DATABASE_URL` | Full Postgres DSN |
| `CLOUDFLARE_API_TOKEN` | CF API token with Pages:Edit |
| `CLOUDFLARE_ACCOUNT_ID` | Your Cloudflare account ID |
| `GA_MEASUREMENT_ID` | Google Analytics 4 measurement ID (optional) |
