# wasitdown.dev — Full Project Handoff

This document is a complete briefing for continuing work on this project.
Read it top to bottom before suggesting anything.

---

## What This Is

**wasitdown.dev** — a historical incident tracker for major cloud and AI providers.
It shows uptime stats, full incident timelines, cross-provider outage correlation, and a "Chaos Buddies" feature that surfaces which providers tend to go down at the same time.

No server-side runtime. The site is 100% static HTML generated from a PostgreSQL database and deployed to Cloudflare Pages.

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 (module: `github.com/hmitsis-dev/wasitdown`) |
| Database | PostgreSQL 16 |
| DB driver | `pgx/v5` — no ORM |
| Templating | Go `html/template` |
| Styling | Tailwind CSS via CDN |
| Local serving | nginx:alpine |
| Containerisation | Docker + Docker Compose v2 |
| Hosting (prod) | Cloudflare Pages |
| Scheduling (prod) | GitHub Actions cron |

---

## Repository Layout

```
cmd/
  scraper/main.go       — scraper entrypoint (daemon or once mode)
  generator/main.go     — static site generator entrypoint
internal/
  db/
    db.go               — pgxpool connection helper
    queries.go          — all SQL queries (no ORM)
  models/
    models.go           — shared structs (Provider, Incident, UptimeStats, ChaosPair, …)
  scraper/
    provider.go         — scraper registry and all provider implementations
    scraper.go          — scraping loop / daemon
  generator/
    generator.go        — site generation logic, template rendering
db/
  migrations/
    001_initial.sql     — schema + seed for 10 core providers
    002_social_providers.sql — seed for 13 social/consumer providers
templates/
  base.html             — shared layout (header, footer, nav)
  index.html            — dashboard (chaos buddies, provider grid, uptime table, recent incidents)
  provider.html         — per-provider incident history + uptime stats
  incident.html         — single incident timeline
  date.html             — all incidents on a given day
  compare.html          — side-by-side provider comparison
static/                 — CSS, JS, robots.txt, favicon
nginx.conf              — local nginx config (try_files + cache headers)
docker-compose.yml      — postgres + scraper + generator + nginx
.env                    — local config (not committed)
.github/
  workflows/
    scraper.yml         — runs scraper every 15min via GitHub Actions
    generate.yml        — runs generator every 20min + deploys to Cloudflare Pages
```

---

## Database Schema

```sql
providers       (id, name, slug, status_page_url, api_url, type, created_at)
incidents       (id, provider_id, external_id, title, impact, status,
                 started_at, resolved_at, duration_minutes, created_at)
incident_updates(id, incident_id, body, status, created_at)
scrape_log      (id, provider_id, scraped_at, success, error)
```

Key indexes: `incidents(started_at DESC)`, `incidents(provider_id)`, `incidents(status)`.

---

## Providers Tracked (23 total)

Anthropic, OpenAI, Cloudflare, GitHub, Vercel, GCP, Azure, AWS, Groq, Mistral,
Discord, Slack, Stripe, Zoom, Spotify, Netflix, Meta, PayPal, Snapchat,
Teams (M365), TikTok, Twitter/X, YouTube.

Scraper types: `statuspage` (Atlassian API), `rss`, `custom` (GCP, YouTube/Google).

---

## Features Built

### Dashboard (`/`)
- **Chaos Buddies** — top of page. SQL self-join finds provider pairs whose incidents started within 2 hours of each other. Shows top 8 pairs with ≥2 co-occurrences. Hidden if no data yet.
- **Provider Status Grid** — 30-day uptime cards for all providers. Client-side filter input (no page reload).
- **Uptime Overview** — table with 30/90/365-day uptime %, incident count (90d), avg duration. All columns sortable by clicking headers (JS, no framework).
- **Recent Incidents** — last 100 incidents across all providers.

### Provider pages (`/provider/{slug}/`)
Full incident history list, three uptime stat cards (30/90/365d), Live Status external link, compare quick-links.

### Incident pages (`/incident/{id}/`)
Full timeline with all status updates.

### Date pages (`/date/YYYY-MM-DD/`)
All incidents on a given day, with concurrent-outage groups highlighted.

### Compare pages (`/compare/{slug-a}-vs-{slug-b}/`)
Side-by-side uptime and incident list for any two providers. All pairs generated at build time.

### JSON API
- `/api/v1/providers.json`
- `/api/v1/recent.json`
- `/api/v1/uptime.json`

---

## Environment Variables

| Variable | Default | Where used |
|---|---|---|
| `DATABASE_URL` | `postgres://wasitdown:changeme@localhost:5432/wasitdown?sslmode=disable` | scraper + generator |
| `POSTGRES_PASSWORD` | `changeme` | docker-compose postgres service |
| `OUTPUT_DIR` | `dist` | generator |
| `TEMPLATES_DIR` | `templates` | generator |
| `STATIC_DIR` | `static` | generator |
| `ADS_ENABLED` | `false` | generator — set `true` in prod to render AdSense slots |
| `MIGRATIONS_DIR` | `db/migrations` | scraper (runs migrations on startup) |
| `SCRAPER_MODE` | `once` | scraper — `once` exits after one pass, `daemon` polls every 15min |

---

## How to Run Locally

```bash
# 1. Start everything
docker compose up -d

# 2. Open the site
open http://localhost:8080

# 3. Force a fresh scrape + regenerate
docker compose up scraper     # scrapes once
docker compose up generator   # regenerates site
```

---

## GitHub Actions Workflows (already written)

**`scraper.yml`** — runs `go run ./cmd/scraper` every 15 minutes.
Needs secret: `DATABASE_URL`

**`generate.yml`** — runs `go run ./cmd/generator` every 20 minutes, then deploys `dist/` to Cloudflare Pages.
Needs secrets: `DATABASE_URL`, `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`

`ADS_ENABLED` is NOT currently set in the workflow — add it as a secret or env var when AdSense is configured.

---

## What Still Needs to Be Done (Next Steps)

### 1. Managed PostgreSQL

The scraper and generator both need a persistent `DATABASE_URL` accessible from GitHub Actions.
Recommended options (all have free tiers):

- **Neon** (`neon.tech`) — serverless Postgres, generous free tier, connection pooling built in. Best fit for this project because the scraper runs in short bursts.
- **Supabase** — Postgres + dashboard UI, good for inspecting data. Slightly heavier.
- **Railway** — simple, good DX, small monthly free allowance.

Steps:
1. Create a project on your chosen provider.
2. Get the connection string (DSN). It will look like `postgres://user:pass@host:5432/dbname?sslmode=require`.
3. Add it as a GitHub Actions secret named `DATABASE_URL`.
4. The scraper runs `db/migrations/` automatically on first start — schema and seed data will be applied.

### 2. Cloudflare Pages

The `generate.yml` workflow deploys to Cloudflare Pages using `cloudflare/pages-action`.

Steps:
1. Create a **Cloudflare account** at `cloudflare.com` if you don't have one.
2. Go to **Workers & Pages → Create → Pages → Connect to Git** — but since deployment is via API (not Git integration), instead go to **Workers & Pages → Create → Pages → Direct Upload** to create the project first. Name it `wasitdown` (must match `projectName` in `generate.yml`).
3. Create a **Cloudflare API token**: Profile → API Tokens → Create Token → use "Edit Cloudflare Workers" template, scope to your account.
4. Add to GitHub Actions secrets:
   - `CLOUDFLARE_API_TOKEN` — the token you just created
   - `CLOUDFLARE_ACCOUNT_ID` — found on the right sidebar of your Cloudflare dashboard

### 3. Custom Domain

Once Cloudflare Pages is deploying:
1. In Cloudflare Pages → your project → Custom Domains → Add domain → `wasitdown.dev`
2. If the domain is also on Cloudflare (recommended), DNS is configured automatically.
3. SSL is automatic via Cloudflare.

### 4. GitHub Repository Secrets Summary

Go to your repo → Settings → Secrets and variables → Actions → New repository secret:

| Secret name | Value |
|---|---|
| `DATABASE_URL` | Your managed Postgres DSN (with `sslmode=require`) |
| `CLOUDFLARE_API_TOKEN` | Cloudflare API token (Pages:Edit scope) |
| `CLOUDFLARE_ACCOUNT_ID` | Your Cloudflare account ID |

Optional (when AdSense is set up):
| `ADS_ENABLED` | `true` |

### 5. AdSense

The templates have `REPLACE_WITH_YOUR_ADSENSE_ID` and `REPLACE_SLOT` placeholders.
When you have a real AdSense publisher ID:
1. Replace `ca-pub-REPLACE_WITH_YOUR_ADSENSE_ID` in `templates/base.html` and both `index.html` / `provider.html` ad slot blocks.
2. Replace `REPLACE_SLOT` with your actual ad slot IDs.
3. Set `ADS_ENABLED=true` in your prod environment / GitHub Actions secret.

### 6. Cron Timing Note

The two GitHub Actions workflows are staggered:
- Scraper: every 15 minutes (`*/15 * * * *`)
- Generator: at :05, :25, :45 past each hour (`5,25,45 * * * *`)

This means the generator always runs ~5 minutes after a scrape pass. No coordination needed.

### 7. Potential Improvements (for future sessions)

- **Search page** — full-text incident search across all providers
- **RSS/Atom feed** — `/feed.xml` so users can subscribe to incidents
- **Alert emails / webhooks** — notify when a new incident is scraped (would require a small server component or a third-party service like Resend + a GitHub Actions step)
- **Historical charts** — uptime trend over time per provider (data is in the DB already)
- **Incident search by keyword** — useful for finding all "database" or "API latency" incidents
- **Sitemap.xml** — already referenced in `robots.txt`? Worth generating for SEO
- **Better Twitter/X status URL** — `https://twitterstat.us` may not always be up; verify
- **Mistral / TikTok / Meta status URLs** — some may need updating as those providers change their status infrastructure

---

## Known Constraints

- The generator is stateless and idempotent — safe to run as many times as needed.
- Compare pages are generated for **all pairs** of providers (n²/2). With 23 providers that's 253 pages — fine, but worth knowing if providers grow significantly.
- Tailwind CSS is loaded from CDN. In production this is fine for a static site, but it means each page loads ~350 KB of CSS. Consider a build step with Tailwind CLI if performance becomes a concern.
- The scraper runs `MIGRATIONS_DIR` on every startup — this is safe because all migrations use `IF NOT EXISTS` / `ON CONFLICT DO NOTHING`.
