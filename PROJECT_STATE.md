# wasitdown.dev — Project State Handoff
_Generated 2026-04-11. Paste this into a new Claude chat to continue work._

---

## What this project is

**wasitdown.dev** — a static site that aggregates historical incident data from 23 cloud, AI, and consumer platform status pages. Monetised via AdSense. Deployed to Cloudflare Pages. No server runtime — pure CDN.

**Repo:** `hmitsis-dev/wasitdown` (currently private, going public before launch)  
**Branch:** `main` (all work merged)  
**Go module:** `github.com/hmitsis-dev/wasitdown`  
**Go version:** 1.25 (required by pgx/v5 v5.9.1)

---

## Repository structure

```
wasitdown/
  cmd/
    scraper/main.go        — runs all scrapers once (or daemon). Entry point for cron.
    generator/main.go      — reads DB, writes dist/. Entry point for site build.
  internal/
    db/
      db.go                — pgxpool connect + RunMigrations()
      queries.go           — all SQL (pgx, no ORM): upserts, selects, uptime stats
    models/
      models.go            — Provider, Incident, IncidentUpdate, ScrapeLog, UptimeStats,
                             StatuspageResponse, GCPFeed types
    scraper/
      provider.go          — Provider interface, StatuspageScraper, AllProviders()
      runner.go            — RunAll(): loops providers, calls Scrape(), logs to scrape_log
      gcp.go               — Google Cloud JSON feed (status.cloud.google.com/incidents.json)
      azure.go             — Azure RSS feed (azure.status.microsoft/en-us/status/feed/)
      aws.go               — AWS Health RSS (health.aws.amazon.com/public/currentevents)
      teams.go             — M365 RSS/Atom (status.office365.com/api/feed)
      youtube.go           — Google consumer status JSON, filtered for YouTube incidents
      netflix.go           — HTML scrape of help.netflix.com/en/node/100649 (best-effort)
    generator/
      generator.go         — Generator.Run(): builds all pages, JSON feeds, correlation
  templates/
    base.html              — shared layout, Tailwind CDN, AdSense slots, nav
    index.html             — dashboard: provider grid, uptime table, recent incidents
    provider.html          — per-provider history + 30/90/365d uptime stats
    date.html              — cross-provider view with 2hr concurrent-window detection
    incident.html          — full incident + update timeline, JSON-LD Event schema
    compare.html           — side-by-side uptime + incident list for two providers
  static/
    css/custom.css         — minimal extras beyond Tailwind (print, focus rings, uptime bar)
    js/main.js             — vanilla JS: date picker nav, provider filter, impact filter
    robots.txt
  db/
    migrations/
      001_initial.sql      — schema + 10 cloud/AI providers seeded
      002_social_providers.sql — 13 social/consumer providers seeded
  Dockerfile.scraper       — multi-stage, alpine runtime, SCRAPER_MODE=once default
  docker-compose.yml       — postgres:16-alpine + scraper (daemon mode)
  .env.example
  .github/
    workflows/
      scraper.yml          — cron */15 * * * * → go run ./cmd/scraper
      generate.yml         — cron 5,25,45 * * * * → generator → Cloudflare Pages deploy
  CLAUDE.md                — developer guide (how to add providers, run locally, migrations)
  go.mod / go.sum
```

---

## Database schema

```sql
providers        (id, name, slug UNIQUE, status_page_url, api_url, type, created_at)
incidents        (id, provider_id, external_id, title, impact, status,
                  started_at, resolved_at, duration_minutes, created_at)
                  UNIQUE (provider_id, external_id)
incident_updates (id, incident_id, body, status, created_at)
scrape_log       (id, provider_id, scraped_at, success, error)
schema_migrations(version, applied_at)   ← migration tracker
```

**Impact values:** `none | minor | major | critical`  
**Provider types:** `statuspage | rss | custom`  
**Uptime stat:** computed via `PERCENTILE_CONT(0.5)` + `SUM(duration_minutes)` / window minutes

---

## All 23 providers

### Scraper type: `StatuspageScraper` (Atlassian /api/v2/incidents.json — zero custom code)

| Name | Slug | Status Page |
|---|---|---|
| Anthropic | `anthropic` | status.anthropic.com |
| OpenAI | `openai` | status.openai.com |
| Cloudflare | `cloudflare` | www.cloudflarestatus.com |
| GitHub | `github` | githubstatus.com |
| Vercel | `vercel` | www.vercel-status.com |
| Groq | `groq` | groqstatus.com |
| Mistral | `mistral` | mistral-status.com |
| Discord | `discord` | discordstatus.com |
| Slack | `slack` | slack-status.com |
| Zoom | `zoom` | status.zoom.us |
| Meta (Instagram/Facebook/WhatsApp/Threads) | `meta` | metastatus.com |
| X (Twitter) | `twitter` | api.twitterstat.us |
| TikTok | `tiktok` | status.tiktok.com |
| Snapchat | `snapchat` | status.snapchat.com |
| Spotify | `spotify` | status.spotify.com |
| PayPal | `paypal` | www.paypal-status.com |
| Stripe | `stripe` | status.stripe.com |

### Scraper type: custom/RSS (dedicated Go files)

| Name | Slug | Source | Notes |
|---|---|---|---|
| Google Cloud | `gcp` | status.cloud.google.com/incidents.json | Custom JSON array format |
| Azure | `azure` | azure.status.microsoft RSS | RSS, classifies impact from title text |
| AWS | `aws` | health.aws.amazon.com RSS | RSS, "RESOLVED:" prefix detection |
| Teams | `teams` | status.office365.com/api/feed | RSS+Atom dual-format parser |
| YouTube | `youtube` | status.google.com/incidents.json | Filters Google status JSON by "youtube" keyword |
| Netflix | `netflix` | help.netflix.com/en/node/100649 | HTML scrape, regex problem detection, best-effort |

---

## Key implementation details

### Adding a new provider (standard Atlassian Statuspage)
1. Add SQL row to a new `db/migrations/NNN_desc.sql` — runs automatically on next scraper start
2. Add `&StatuspageScraper{slug: "newslug"}` to `AllProviders()` in `internal/scraper/provider.go`
3. No other code needed

### Generator pages produced
- `/index.html` — dashboard
- `/provider/[slug]/index.html` — one per provider (23 total)
- `/date/[yyyy-mm-dd]/index.html` — one per day that has ≥1 incident
- `/incident/[id]/index.html` — one per incident row
- `/compare/[slug-vs-slug]/index.html` — all pairwise combos
- `/api/v1/recent.json`, `providers.json`, `uptime.json`

### Cross-provider correlation
`findConcurrentGroups()` in `generator.go`: groups incidents that started within a 2-hour window AND span ≥2 different providers. Displayed as a warning banner on `/date/` pages.

### Template func map (available in all templates)
`formatDate`, `formatDateTime`, `formatDateHuman`, `impactColor`, `impactBadge`,
`uptimeColor`, `formatUptime`, `domain`, `now`, `safeHTML`, `sub`, `roundFloat`, `derefTime`

### Environment variables
| Var | Used by | Default |
|---|---|---|
| `DATABASE_URL` | scraper + generator | (required) |
| `MIGRATIONS_DIR` | scraper | `db/migrations` |
| `SCRAPER_MODE` | scraper | `once` (`daemon` = poll every 15min) |
| `OUTPUT_DIR` | generator | `dist` |
| `TEMPLATES_DIR` | generator | `templates` |
| `STATIC_DIR` | generator | `static` |

---

## Running locally

```bash
# 1. Start postgres
cp .env.example .env          # set POSTGRES_PASSWORD
docker-compose up -d          # starts postgres only (scraper is daemon mode)

# 2. Run scraper (one pass, applies migrations, seeds providers)
export DATABASE_URL="postgres://wasitdown:changeme@localhost:5432/wasitdown?sslmode=disable"
go run ./cmd/scraper

# 3. Generate static site
go run ./cmd/generator
# Output → dist/

# 4. Preview
cd dist && python3 -m http.server 8080
```

---

## CI/CD

| Workflow | Trigger | Action |
|---|---|---|
| `scraper.yml` | Every 15min + manual | `go run ./cmd/scraper` (once mode) against managed DB |
| `generate.yml` | Every 20min + manual | `go run ./cmd/generator` → `dist/` → Cloudflare Pages |

### GitHub Secrets required
- `DATABASE_URL` — full Postgres DSN (Supabase / Neon / Railway recommended)
- `CLOUDFLARE_API_TOKEN` — Pages:Edit permission
- `CLOUDFLARE_ACCOUNT_ID`

### Cloudflare Pages project name
`wasitdown` (hardcoded in `generate.yml`)

---

## SEO setup per page

Every page has:
- `<title>`, `<meta name="description">`, `<link rel="canonical">`
- `og:title`, `og:description`, `og:url`, `og:type`
- `twitter:card`, `twitter:title`, `twitter:description`
- JSON-LD structured data (WebSite on index, WebPage on provider/date/compare, Event on incident)

---

## AdSense

Placeholder `ca-pub-REPLACE_WITH_YOUR_ADSENSE_ID` and `REPLACE_SLOT` appear in:
- `templates/base.html` (head script)
- `templates/index.html` (top + bottom slots)
- `templates/provider.html` (mid slot)
- `templates/date.html` (mid slot)
- `templates/incident.html` (mid slot)
- `templates/compare.html` (mid slot)

Replace both strings across all templates once AdSense is approved.

---

## Known limitations / open items

1. **Netflix scraper** — no public API. HTML-parses their help page. Very shallow history; only captures active/same-day incidents. Consider removing if data quality is poor.
2. **YouTube scraper** — no dedicated YouTube status page. Uses `status.google.com/incidents.json` and filters by "youtube" keyword. Works if Google posts YouTube incidents there; otherwise silent.
3. **Teams scraper** — Microsoft's public RSS feed for M365 health. Historical depth limited; grows as scraper accumulates data.
4. **TikTok / Snapchat** — added as Atlassian Statuspage scrapers but URLs are best-guess (`status.tiktok.com`, `status.snapchat.com`). Will log errors to `scrape_log` if the endpoints don't resolve — easy to fix by updating `api_url` in the DB.
5. **Compare pages** — generates *all* pairwise combinations (n×(n-1)/2 = 253 pages for 23 providers). Fine now, but grows quadratically with provider count.
6. **Static assets** — `static/` is copied to `dist/` in the CI step. The generator itself doesn't copy static files; that's handled by the generate workflow `cp -r static/. dist/`.
7. **Sitemap** — `robots.txt` references `/sitemap.xml` but no sitemap generator exists yet.
8. **Search** — `robots.txt` sitemap + JSON-LD cover SEO basics, but there's no client-side search page yet.

---

## Code conventions (enforce these)
- No global state — all deps passed explicitly
- Provider interface: each source is an isolated struct, errors logged to `scrape_log`, never panic
- DB: `pgx` only, no ORM, all queries in `internal/db/queries.go`
- Generator: idempotent, uses `os.Create` (overwrites)
- Templates: `html/template`, all funcs registered in `generator.New()`
- New migrations: add `NNN_description.sql`, never edit existing files
