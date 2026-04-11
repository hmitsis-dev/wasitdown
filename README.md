# wasitdown.dev

> Was it down? Historical cloud & AI provider incident tracker.

Aggregates incident history from public status pages across major cloud and AI providers. A scraper collects data into PostgreSQL, and a generator builds a fully static site deployed to Cloudflare Pages — no server-side runtime.

---

## What it tracks

AWS, Azure, Google Cloud, Anthropic, OpenAI, Cloudflare, GitHub, Vercel, Stripe, Zoom, and more (23 providers total). View incidents by provider, by date, or compare providers side by side.

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
- **Ads toggle** — AdSense slots controlled by `ADS_ENABLED` env var; off by default

---

## Architecture

```
GitHub Actions (cron)
  └─ every 15min → scraper → PostgreSQL (Supabase / Neon / Railway)
  └─ every 20min → generator → dist/ → Cloudflare Pages
```

All status page polling is outbound HTTP GET only. The site is pure static HTML — no backend at runtime.

---

## Running Locally

**Prerequisites:** Docker & Docker Compose, Go 1.22+

### 1. Clone and configure

```bash
git clone https://github.com/hmitsis-dev/wasitdown
cd wasitdown
cp .env.example .env   # or edit .env directly — defaults work for local dev
```

### 2. Start all services

```bash
docker compose up -d
```

This starts PostgreSQL, the scraper (daemon mode, polls every 15 min), the generator (runs once then exits), and nginx on port 8080.

### 3. Open the site

```
http://localhost:8080
```

The generator runs automatically on startup. To regenerate manually after schema or template changes:

```bash
docker compose up generator
```

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_PASSWORD` | `changeme` | PostgreSQL password |
| `DATABASE_URL` | _(local DSN)_ | Full Postgres connection string |
| `OUTPUT_DIR` | `dist` | Generator output directory |
| `TEMPLATES_DIR` | `templates` | HTML template directory |
| `STATIC_DIR` | `static` | Static assets directory |
| `ADS_ENABLED` | `false` | Set to `true` to render AdSense slots |

---

## Adding a Provider

1. **Add a DB row** in `db/migrations/001_initial.sql` (or a new migration file):

```sql
INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
  ('My Provider', 'myprovider', 'https://status.myprovider.com',
   'https://status.myprovider.com/api/v2/incidents.json', 'statuspage')
ON CONFLICT (slug) DO NOTHING;
```

2. **Wire up the scraper** — for standard Atlassian Statuspage providers, add the slug to `AllProviders()` in `internal/scraper/provider.go`:

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

For production, also set `ADS_ENABLED=true` in your GitHub Actions environment if you want AdSense to render.

Push to `main` — the GitHub Actions workflows handle the rest.

---

## Tech Stack

- **Go 1.22+** — scraper + static site generator
- **PostgreSQL** — incident storage (managed DB recommended)
- **Tailwind CSS** (CDN) — styling
- **nginx** — local static file serving
- **Cloudflare Pages** — production hosting
- **GitHub Actions** — cron scheduling

---

## License

See [LICENSE](LICENSE).
