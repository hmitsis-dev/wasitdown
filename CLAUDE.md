# wasitdown.dev — Developer Guide

Historical cloud & AI provider incident aggregator. Scrapes public status pages, stores to PostgreSQL, generates a static site deployed to Cloudflare Pages.

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
      aws.go              — AWS Health RSS feed
      runner.go           — RunAll() orchestrator
    generator/
      generator.go        — Generator.Run() → builds dist/ HTML + JSON
  templates/
    base.html             — shared layout (header, footer, Tailwind)
    index.html            — dashboard
    provider.html         — per-provider incident history
    date.html             — cross-provider view for a given date
    incident.html         — full incident timeline
    compare.html          — side-by-side provider comparison
  static/
    css/custom.css        — minimal custom styles beyond Tailwind
    js/main.js            — vanilla JS (no framework)
    robots.txt
  db/
    migrations/
      001_initial.sql     — schema + provider seed data
  Dockerfile.scraper
  docker-compose.yml      — postgres + scraper (daemon mode)
  .env.example
  .github/
    workflows/
      scraper.yml         — cron: every 15min → runs scraper once
      generate.yml        — cron: every 20min → generates site + deploys to CF Pages
```

---

## Running Locally

### Prerequisites
- Docker & Docker Compose
- Go 1.22+

### 1. Start PostgreSQL + scraper

```bash
cp .env.example .env
# Edit .env — set POSTGRES_PASSWORD and DATABASE_URL

docker-compose up -d
# Postgres starts, scraper daemon runs every 15min
```

### 2. Run the scraper manually (one pass)

```bash
export DATABASE_URL="postgres://wasitdown:changeme@localhost:5432/wasitdown?sslmode=disable"
go run ./cmd/scraper
```

### 3. Generate the static site

```bash
export DATABASE_URL="postgres://wasitdown:changeme@localhost:5432/wasitdown?sslmode=disable"
go run ./cmd/generator
# Output written to dist/
```

### 4. Preview the site

```bash
cd dist && python3 -m http.server 8080
# Open http://localhost:8080
```

---

## Adding a New Provider

All providers are defined in two places:

### Step 1 — Add a DB seed row

In `db/migrations/001_initial.sql` (or a new migration file), add:

```sql
INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
  ('My Provider', 'myprovider', 'https://status.myprovider.com',
   'https://status.myprovider.com/api/v2/incidents.json', 'statuspage')
ON CONFLICT (slug) DO NOTHING;
```

Then run the scraper once to apply — it auto-migrates on startup.

### Step 2 — Wire up the scraper

**If it's a standard Atlassian Statuspage** (most providers): no code needed. Just add the `StatuspageScraper` slug to `AllProviders()` in `internal/scraper/provider.go`:

```go
func AllProviders() []Provider {
    return []Provider{
        &StatuspageScraper{slug: "anthropic"},
        // ...
        &StatuspageScraper{slug: "myprovider"},  // ← add this
    }
}
```

**If it's a custom format**: create `internal/scraper/myprovider.go` implementing the `Provider` interface:

```go
type MyProviderScraper struct{}

func (s *MyProviderScraper) Slug() string { return "myprovider" }

func (s *MyProviderScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
    body, err := httpGet(ctx, p.APIURL)
    // parse body, upsert incidents via db.UpsertIncident()
    return err
}
```

Then add it to `AllProviders()`.

### Step 3 — Re-run

```bash
go run ./cmd/scraper   # fetches historical incidents
go run ./cmd/generator # rebuilds static site
```

---

## Database Migrations

Plain SQL files in `db/migrations/`, named `NNN_description.sql`. The migration runner in `internal/db/db.go` tracks applied versions in a `schema_migrations` table and applies new files in filename order — each in a transaction.

To add a migration:
1. Create `db/migrations/002_add_something.sql`
2. Restart the scraper — it applies on startup automatically.

Never modify already-applied migrations; always add new files.

---

## GitHub Actions Setup

Two workflows:

| Workflow | Trigger | What it does |
|---|---|---|
| `scraper.yml` | Every 15min + manual | Runs `go run ./cmd/scraper` (once mode) |
| `generate.yml` | Every 20min + manual | Runs generator → deploys to Cloudflare Pages |

### Required GitHub Secrets

| Secret | Description |
|---|---|
| `DATABASE_URL` | Full postgres DSN (`postgres://user:pass@host:5432/db`) |
| `CLOUDFLARE_API_TOKEN` | CF API token with Pages:Edit permission |
| `CLOUDFLARE_ACCOUNT_ID` | Your Cloudflare account ID |

---

## Code Conventions

- **No global state** — all dependencies passed explicitly.
- **Provider interface** — each source is an independent struct. Errors are logged to `scrape_log`, never panic.
- **DB layer** — `pgx` only, no ORM. All queries in `internal/db/queries.go`.
- **Generator** — idempotent. Safe to run multiple times; uses `os.Create` (overwrites).
- **Templates** — `html/template` with a func map. All template functions registered in `generator.New()`.

---

## Architecture Notes

```
GitHub Actions (cron)
  └─ scraper.yml → cmd/scraper → PostgreSQL (managed DB, e.g. Supabase/Neon/Railway)
  └─ generate.yml → cmd/generator → dist/ → Cloudflare Pages

All status page polling is outbound HTTP GET only.
The static site has zero server-side runtime — pure CDN delivery.
```
