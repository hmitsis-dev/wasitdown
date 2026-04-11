# wasitdown.dev — Project State
_Paste this into a new Claude Code session to continue work. Last updated 2026-04-11._

---

## What this project is

**wasitdown.dev** — a static site that aggregates historical incident data from 23 cloud, AI,
and consumer platform status pages. Monetised via AdSense. Deployed to Cloudflare Pages.
Zero server-side runtime — pure CDN.

**Repo:** `hmitsis-dev/wasitdown` (private → going public at launch)
**Branch:** `main` — all work is here
**Go module:** `github.com/hmitsis-dev/wasitdown`
**Go version:** `1.25` (required by `pgx/v5 v5.9.1`)

---

## Full File Map

```
cmd/
  scraper/main.go         — entry: runs all scrapers once or daemon
  generator/main.go       — entry: reads DB, writes dist/

internal/
  db/
    db.go                 — pgxpool connect + RunMigrations()
    queries.go            — all SQL (pgx, no ORM): upserts, GetChaosPairs, uptime stats
  models/
    models.go             — Provider, Incident, IncidentUpdate, ScrapeLog,
                            UptimeStats, ChaosPair, StatuspageResponse, GCPFeed
  scraper/
    provider.go           — Provider interface, StatuspageScraper, AllProviders()
    runner.go             — RunAll(): loops providers, logs every result to scrape_log
    gcp.go                — Google Cloud JSON (status.cloud.google.com/incidents.json)
    azure.go              — Azure RSS feed
    aws.go                — AWS Health RSS
    teams.go              — M365 RSS/Atom dual-format parser
    youtube.go            — Google consumer status JSON, filtered for YouTube
    netflix.go            — HTML scrape of Netflix help page (best-effort)
  generator/
    generator.go          — Generator.Run(): all page types, sitemap, static copy

templates/
  base.html               — layout, Tailwind CDN, custom.css, adsEnabled/analyticsID guards
  index.html              — dashboard: chaos buddies, provider grid + filter, sortable
                            uptime table, recent incidents
  provider.html           — per-provider: 30/90/365d uptime cards, full incident list
  incident.html           — single incident + update timeline, JSON-LD Event schema
  date.html               — all incidents on a day, concurrent-window correlation banner
  compare.html            — side-by-side uptime + incident lists for any two providers
  privacy.html            — privacy policy (GA4, AdSense, Cloudflare CDN, GDPR)

static/
  css/custom.css          — print styles, focus rings, uptime bar
  js/main.js              — vanilla JS: provider filter, impact filter, date nav
  robots.txt

db/
  migrations/
    001_initial.sql       — schema + 10 cloud/AI providers seeded
    002_social_providers.sql — 13 social/consumer providers seeded

Dockerfile.scraper        — Go 1.25-alpine multi-stage; SCRAPER_MODE=once default
Dockerfile.generator      — Go 1.25-alpine multi-stage; copies templates/ + static/
docker-compose.yml        — PRODUCTION: postgres only, bound to 127.0.0.1:5432
docker-compose.local.yml  — LOCAL DEV: postgres + scraper + generator + nginx on :8080
nginx.conf                — try_files for clean URLs, cache headers for assets/HTML

.env.example
.gitignore                — excludes .env, dist/, AI tool configs
.grimoire                 — AI tool config (do not commit changes here)

.github/
  workflows/
    scraper.yml           — COMMENTED OUT (activate when prod DB is ready)
    generate.yml          — COMMENTED OUT (activate when prod DB + CF Pages are ready)
  ISSUE_TEMPLATE/
    bug_report.md
    feature_request.md
    provider_request.md

README.md                 — public-facing readme
SETUP.md                  — full VPS + GitHub Actions + Cloudflare Pages setup guide
CONTRIBUTING.md           — contributor guide (add providers, template rules, query rules)
SECURITY.md               — responsible disclosure policy
HANDOFF.md                — comprehensive developer briefing
PROJECT_STATE.md          — this file
CLAUDE.md                 — developer guide (in .gitignore — not committed)
```

---

## Database Schema

```sql
providers        (id, name, slug UNIQUE, status_page_url, api_url,
                  type CHECK('statuspage'|'rss'|'custom'), created_at)

incidents        (id, provider_id → providers, external_id, title,
                  impact CHECK('none'|'minor'|'major'|'critical'),
                  status, started_at, resolved_at, duration_minutes, created_at)
                  UNIQUE (provider_id, external_id)

incident_updates (id, incident_id → incidents ON DELETE CASCADE,
                  body, status, created_at)

scrape_log       (id, provider_id, scraped_at, success, error)

schema_migrations(version PK, applied_at)   — migration tracker
```

Indexes: `incidents(started_at DESC)`, `incidents(provider_id)`, `incidents(status)`

---

## All 23 Providers

### Atlassian Statuspage — `StatuspageScraper` (no custom code per provider)

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

### Custom scrapers (dedicated Go files)

| Name | Slug | Source | Notes |
|---|---|---|---|
| Google Cloud | `gcp` | status.cloud.google.com/incidents.json | Custom JSON array |
| Azure | `azure` | azure.status.microsoft RSS | RSS, title-based impact classification |
| AWS | `aws` | health.aws.amazon.com RSS | RSS, "RESOLVED:" prefix detection |
| Teams | `teams` | status.office365.com/api/feed | RSS+Atom dual parser |
| YouTube | `youtube` | status.google.com/incidents.json | Filters Google JSON by "youtube" keyword |
| Netflix | `netflix` | help.netflix.com/en/node/100649 | HTML scrape, regex problem detection |

---

## Generator — What It Builds

Every `Generator.Run()` call produces:

| Output | Path |
|---|---|
| Dashboard | `/index.html` |
| Per-provider | `/provider/{slug}/index.html` × 23 |
| Per-day | `/date/YYYY-MM-DD/index.html` × 1 per day with incidents |
| Per-incident | `/incident/{id}/index.html` × 1 per incident row |
| Compare | `/compare/{a}-vs-{b}/index.html` × all pairs (253 with 23 providers) |
| Privacy policy | `/privacy/index.html` |
| JSON feeds | `/api/v1/recent.json`, `providers.json`, `uptime.json` |
| Sitemap | `/sitemap.xml` |
| Static assets | `/css/custom.css`, `/js/main.js`, `/robots.txt` (copied by generator) |

**Template system:** `base.html` is cloned once per page type so `{{define "head-extra"}}` and
`{{define "content"}}` are isolated per page — prevents Go template namespace collisions.

**Template functions available:** `formatDate`, `formatDateTime`, `formatDateHuman`,
`impactColor`, `impactBadge`, `uptimeColor`, `formatUptime`, `adsEnabled`, `analyticsID`,
`lower`, `domain`, `now`, `safeHTML`, `sub`, `roundFloat`, `derefTime`

**Chaos Buddies query** (`GetChaosPairs`): SQL self-join on `incidents` finding provider
pairs whose `started_at` values are within 7200 seconds of each other. Requires ≥2 co-occurrences.

---

## Environment Variables

| Variable | Default | Binary |
|---|---|---|
| `DATABASE_URL` | _(required)_ | both |
| `POSTGRES_PASSWORD` | `changeme` | docker-compose |
| `MIGRATIONS_DIR` | `db/migrations` | scraper |
| `SCRAPER_MODE` | `once` | scraper — `daemon` polls every 15 min |
| `OUTPUT_DIR` | `dist` | generator |
| `TEMPLATES_DIR` | `templates` | generator |
| `STATIC_DIR` | `static` | generator |
| `ADS_ENABLED` | `false` | generator — `true` renders AdSense `<ins>` slots |
| `GA_MEASUREMENT_ID` | _(empty)_ | generator — `G-XXXXXXXXXX` enables GA4 snippet |

---

## Running Locally

```bash
# 1. Configure
cp .env.example .env          # defaults work as-is for local dev

# 2. Start postgres
docker compose -f docker-compose.local.yml up -d postgres

# 3. Scrape (applies migrations + seeds providers on first run)
docker compose -f docker-compose.local.yml run --rm scraper

# 4. Generate site
docker compose -f docker-compose.local.yml run --rm generator

# 5. Serve at http://localhost:8080
docker compose -f docker-compose.local.yml up -d web
```

Or start everything at once and wait ~30s for scraper + generator to finish:
```bash
docker compose -f docker-compose.local.yml up -d
```

---

## Production Architecture (pending setup)

```
GitHub Actions cron
  scraper.yml   → every 15 min → go run ./cmd/scraper  → managed PostgreSQL
  generate.yml  → every 20 min → go run ./cmd/generator → dist/ → Cloudflare Pages
```

**Both workflows are currently commented out.** To activate:
1. Provision a managed Postgres (Neon recommended — serverless, good free tier)
2. Create Cloudflare Pages project named `wasitdown`
3. Add three GitHub Actions secrets: `DATABASE_URL`, `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`
4. In both workflow files, uncomment everything
5. **Update `go-version: "1.22"` → `"1.25"` in both workflows before enabling** (currently stale)
6. The `cp -r static/. dist/` step in `generate.yml` is also redundant — generator handles it via `copyStatic()`

Full walkthrough: `SETUP.md`

---

## AdSense & Analytics Setup

Templates are ready — just need real IDs:

1. **AdSense:** Replace `ca-pub-REPLACE_WITH_YOUR_ADSENSE_ID` and `REPLACE_SLOT` in:
   - `templates/base.html` (script tag)
   - `templates/index.html` (2 slots)
   - `templates/provider.html`, `date.html`, `incident.html`, `compare.html` (1 slot each)
   Then set `ADS_ENABLED=true` in your GitHub Actions env / secret.

2. **Google Analytics:** Set `GA_MEASUREMENT_ID=G-XXXXXXXXXX` in GitHub Actions env.
   The snippet is already in `base.html` behind `{{if analyticsID}}`.

---

## Adding a New Provider

**Standard Atlassian Statuspage (no new code):**
1. Add SQL row to `db/migrations/002_social_providers.sql` (or a new `003_*.sql` file):
   ```sql
   INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
     ('Name', 'slug', 'https://status.example.com',
      'https://status.example.com/api/v2/incidents.json', 'statuspage')
   ON CONFLICT (slug) DO NOTHING;
   ```
2. Add `&StatuspageScraper{slug: "slug"}` to `AllProviders()` in `internal/scraper/provider.go`
3. Re-run scraper then generator

**Custom format:** Create `internal/scraper/{slug}.go` implementing `Provider` interface
(`Slug() string`, `Scrape(ctx, pool, models.Provider) error`), then add to `AllProviders()`.

---

## Code Conventions

- No global state — all deps passed explicitly
- Every error logged to `scrape_log` via `db.LogScrape()` — never panic
- All DB queries in `internal/db/queries.go` — pgx only, no ORM
- Generator is idempotent — `os.Create` overwrites, safe to run many times
- New migrations: add `NNN_description.sql`, never edit applied files
- Ad slots always wrapped in `{{if adsEnabled}}...{{end}}`

---

## Known Issues / Open Items

| Issue | File | Fix |
|---|---|---|
| `go-version: "1.22"` in CI workflows | `.github/workflows/*.yml` | Change to `"1.25"` when uncommenting |
| Redundant `cp -r static/. dist/` in `generate.yml` | same | Remove — generator handles it |
| AdSense placeholders | all templates | Replace when approved |
| Netflix has no public API | `internal/scraper/netflix.go` | HTML scrape only; will be empty until an incident is live |
| TikTok/Snapchat API URLs unverified | `002_social_providers.sql` | Update `api_url` in DB if 404s appear in `scrape_log` |
| 253 compare pages at 23 providers | `generator.go` | Grows as O(n²) — fine for now |
| No 404.html | `dist/` | Referenced in `nginx.conf`; generator doesn't create it yet |
