package generator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"strings"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

const siteDomain = "https://wasitdown.dev"

// Config holds generator settings.
type Config struct {
	OutputDir       string
	TemplatesDir    string
	StaticDir       string
	AdsEnabled      bool
	GAMeasurementID string // e.g. "G-XXXXXXXXXX"; empty = analytics disabled
}

// Generator builds the static site from the database.
type Generator struct {
	cfg       Config
	pool      *pgxpool.Pool
	templates map[string]*template.Template
}

// New creates a Generator and parses all templates.
func New(pool *pgxpool.Pool, cfg Config) (*Generator, error) {
	funcMap := template.FuncMap{
		"formatDate":     func(t time.Time) string { return t.UTC().Format("2006-01-02") },
		"formatDateTime": func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04 UTC") },
		"formatDateHuman": func(t time.Time) string {
			return t.UTC().Format("January 2, 2006")
		},
		"impactColor": func(impact models.Impact) string {
			switch impact {
			case models.ImpactCritical:
				return "red"
			case models.ImpactMajor:
				return "orange"
			case models.ImpactMinor:
				return "yellow"
			default:
				return "gray"
			}
		},
		"impactBadge": func(impact models.Impact) string {
			switch impact {
			case models.ImpactCritical:
				return "bg-red-100 text-red-800 border border-red-300"
			case models.ImpactMajor:
				return "bg-orange-100 text-orange-800 border border-orange-300"
			case models.ImpactMinor:
				return "bg-yellow-100 text-yellow-800 border border-yellow-300"
			default:
				return "bg-gray-100 text-gray-600 border border-gray-200"
			}
		},
		"uptimeColor": func(pct float64) string {
			switch {
			case pct >= 99.9:
				return "text-green-600"
			case pct >= 99.0:
				return "text-yellow-600"
			case pct >= 95.0:
				return "text-orange-600"
			default:
				return "text-red-600"
			}
		},
		"formatUptime": func(pct float64) string {
			return fmt.Sprintf("%.3f%%", pct)
		},
		"adsEnabled":      func() bool   { return cfg.AdsEnabled },
		"analyticsID":     func() string { return cfg.GAMeasurementID },
		"lower":      strings.ToLower,
		"domain":     func() string { return siteDomain },
		"now":    func() time.Time { return time.Now().UTC() },
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) //nolint:gosec
		},
		"sub": func(a, b int) int { return a - b },
		"roundFloat": func(f float64) int { return int(math.Round(f)) },
		"derefTime": func(t *time.Time) time.Time {
			if t == nil {
				return time.Time{}
			}
			return *t
		},
	}

	// Parse base.html once, then clone it per page so each page gets its own
	// isolated template namespace. This prevents {{define "head-extra"}} in one
	// page from overwriting another's definition (Go templates share a single
	// namespace when parsed together via ParseGlob).
	basePath := filepath.Join(cfg.TemplatesDir, "base.html")
	base, err := template.New("base.html").Funcs(funcMap).ParseFiles(basePath)
	if err != nil {
		return nil, fmt.Errorf("parse base template: %w", err)
	}

	pages := []string{"index.html", "provider.html", "date.html", "incident.html", "compare.html", "privacy.html"}
	templates := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		t, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone base for %s: %w", page, err)
		}
		if _, err := t.ParseFiles(filepath.Join(cfg.TemplatesDir, page)); err != nil {
			return nil, fmt.Errorf("parse %s: %w", page, err)
		}
		templates[page] = t
	}

	return &Generator{cfg: cfg, pool: pool, templates: templates}, nil
}

// Run generates the complete static site. Safe to call multiple times.
func (g *Generator) Run(ctx context.Context) error {
	if err := os.MkdirAll(g.cfg.OutputDir, 0o755); err != nil {
		return err
	}

	providers, err := db.GetAllProviders(ctx, g.pool)
	if err != nil {
		return fmt.Errorf("load providers: %w", err)
	}

	stats30, err := db.GetUptimeStats(ctx, g.pool, 30)
	if err != nil {
		return fmt.Errorf("uptime 30d: %w", err)
	}
	stats90, err := db.GetUptimeStats(ctx, g.pool, 90)
	if err != nil {
		return fmt.Errorf("uptime 90d: %w", err)
	}
	stats365, err := db.GetUptimeStats(ctx, g.pool, 365)
	if err != nil {
		return fmt.Errorf("uptime 365d: %w", err)
	}

	recent, err := db.GetRecentIncidents(ctx, g.pool, 100)
	if err != nil {
		return fmt.Errorf("recent incidents: %w", err)
	}

	chaosPairs, err := db.GetChaosPairs(ctx, g.pool, 8)
	if err != nil {
		return fmt.Errorf("chaos pairs: %w", err)
	}

	// Build uptime lookup maps keyed by provider slug.
	statsMap := func(stats []models.UptimeStats) map[string]models.UptimeStats {
		m := make(map[string]models.UptimeStats)
		for _, s := range stats {
			m[s.ProviderSlug] = s
		}
		return m
	}
	m30 := statsMap(stats30)
	m90 := statsMap(stats90)
	m365 := statsMap(stats365)

	if err := g.copyStatic(); err != nil {
		return fmt.Errorf("copy static: %w", err)
	}

	if err := g.generateIndex(ctx, providers, m30, m90, m365, recent, chaosPairs); err != nil {
		return err
	}

	steps := []func(context.Context, []models.Provider, map[string]models.UptimeStats, map[string]models.UptimeStats, map[string]models.UptimeStats, []models.Incident) error{
		g.generateProviderPages,
		g.generateDatePages,
		g.generateIncidentPages,
		g.generateComparePages,
		g.generateJSON,
	}

	for _, step := range steps {
		if err := step(ctx, providers, m30, m90, m365, recent); err != nil {
			return err
		}
	}

	if err := g.generateStaticPages(); err != nil {
		return err
	}
	if err := g.generateSitemap(ctx, providers); err != nil {
		return fmt.Errorf("sitemap: %w", err)
	}
	if err := g.generateFeed(ctx); err != nil {
		return fmt.Errorf("feed: %w", err)
	}

	slog.Info("site generation complete", "output", g.cfg.OutputDir)
	return nil
}

// --- Page data structs ---

type indexData struct {
	Title       string
	Description string
	Canonical   string
	Providers   []models.Provider
	Recent      []models.Incident
	ChaosPairs  []models.ChaosPair
	Stats30     map[string]models.UptimeStats
	Stats90     map[string]models.UptimeStats
	Stats365    map[string]models.UptimeStats
	Generated   time.Time
}

type providerData struct {
	Title       string
	Description string
	Canonical   string
	Provider    models.Provider
	Incidents   []models.Incident
	Stats30     models.UptimeStats
	Stats90     models.UptimeStats
	Stats365    models.UptimeStats
	Generated   time.Time
}

type dateData struct {
	Title            string
	Description      string
	Canonical        string
	Date             time.Time
	Incidents        []models.Incident
	ConcurrentGroups [][]models.Incident
	Generated        time.Time
}

type incidentData struct {
	Title       string
	Description string
	Canonical   string
	Incident    models.Incident
	Generated   time.Time
}

type staticData struct {
	Title       string
	Description string
	Canonical   string
	Generated   time.Time
}

type compareData struct {
	Title       string
	Description string
	Canonical   string
	ProviderA   models.Provider
	ProviderB   models.Provider
	Stats30A    models.UptimeStats
	Stats30B    models.UptimeStats
	Stats90A    models.UptimeStats
	Stats90B    models.UptimeStats
	IncidentsA  []models.Incident
	IncidentsB  []models.Incident
	Generated   time.Time
}

// --- Generators ---

func (g *Generator) generateIndex(
	ctx context.Context,
	providers []models.Provider,
	m30, m90, m365 map[string]models.UptimeStats,
	recent []models.Incident,
	chaosPairs []models.ChaosPair,
) error {
	data := indexData{
		Title:       "Was It Down? — Cloud & AI Provider Incident History",
		Description: "Track historical incidents across AWS, Cloudflare, OpenAI, Anthropic, GitHub, Vercel, GCP, Azure and more. See uptime stats, cross-provider outages, and full incident timelines.",
		Canonical:   siteDomain + "/",
		Providers:   providers,
		Recent:      recent,
		ChaosPairs:  chaosPairs,
		Stats30:     m30,
		Stats90:     m90,
		Stats365:    m365,
		Generated:   time.Now().UTC(),
	}
	return g.render("index.html", filepath.Join(g.cfg.OutputDir, "index.html"), data)
}

func (g *Generator) generateProviderPages(
	ctx context.Context,
	providers []models.Provider,
	m30, m90, m365 map[string]models.UptimeStats,
	_ []models.Incident,
) error {
	for _, p := range providers {
		incidents, err := db.GetIncidentsByProvider(ctx, g.pool, p.ID)
		if err != nil {
			slog.Error("provider incidents", "slug", p.Slug, "err", err)
			continue
		}
		dir := filepath.Join(g.cfg.OutputDir, "provider", p.Slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		data := providerData{
			Title:       fmt.Sprintf("%s Incident History & Uptime — Was It Down?", p.Name),
			Description: fmt.Sprintf("Complete incident history for %s. View uptime statistics, outage timelines, and status updates.", p.Name),
			Canonical:   fmt.Sprintf("%s/provider/%s", siteDomain, p.Slug),
			Provider:    p,
			Incidents:   incidents,
			Stats30:     m30[p.Slug],
			Stats90:     m90[p.Slug],
			Stats365:    m365[p.Slug],
			Generated:   time.Now().UTC(),
		}
		if err := g.render("provider.html", filepath.Join(dir, "index.html"), data); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) generateDatePages(
	ctx context.Context,
	_ []models.Provider,
	_, _, _ map[string]models.UptimeStats,
	_ []models.Incident,
) error {
	dates, err := db.GetDistinctIncidentDates(ctx, g.pool)
	if err != nil {
		return fmt.Errorf("get dates: %w", err)
	}
	for _, d := range dates {
		incidents, err := db.GetIncidentsByDate(ctx, g.pool, d)
		if err != nil {
			slog.Error("date incidents", "date", d, "err", err)
			continue
		}
		concurrent := findConcurrentGroups(incidents, 2*time.Hour)
		dateStr := d.UTC().Format("2006-01-02")
		dir := filepath.Join(g.cfg.OutputDir, "date", dateStr)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		data := dateData{
			Title:            fmt.Sprintf("Cloud Incidents on %s — Was It Down?", d.UTC().Format("January 2, 2006")),
			Description:      fmt.Sprintf("All cloud and AI provider incidents on %s. Cross-provider correlation and outage timeline.", d.UTC().Format("January 2, 2006")),
			Canonical:        fmt.Sprintf("%s/date/%s", siteDomain, dateStr),
			Date:             d,
			Incidents:        incidents,
			ConcurrentGroups: concurrent,
			Generated:        time.Now().UTC(),
		}
		if err := g.render("date.html", filepath.Join(dir, "index.html"), data); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) generateIncidentPages(
	ctx context.Context,
	_ []models.Provider,
	_, _, _ map[string]models.UptimeStats,
	_ []models.Incident,
) error {
	ids, err := db.GetAllIncidentIDs(ctx, g.pool)
	if err != nil {
		return fmt.Errorf("get incident IDs: %w", err)
	}
	for _, id := range ids {
		inc, err := db.GetIncidentByID(ctx, g.pool, id)
		if err != nil || inc == nil {
			slog.Error("fetch incident", "id", id, "err", err)
			continue
		}
		dir := filepath.Join(g.cfg.OutputDir, "incident", fmt.Sprintf("%d", id))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		data := incidentData{
			Title:       fmt.Sprintf("%s — %s Incident | Was It Down?", inc.Title, inc.ProviderName),
			Description: fmt.Sprintf("Full timeline for %s incident on %s: %s", inc.ProviderName, inc.StartedAt.UTC().Format("January 2, 2006"), inc.Title),
			Canonical:   fmt.Sprintf("%s/incident/%d", siteDomain, id),
			Incident:    *inc,
			Generated:   time.Now().UTC(),
		}
		if err := g.render("incident.html", filepath.Join(dir, "index.html"), data); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) generateComparePages(
	ctx context.Context,
	providers []models.Provider,
	m30, m90, _ map[string]models.UptimeStats,
	_ []models.Incident,
) error {
	// Generate pairs for adjacent providers (avoids n^2 explosion).
	// In practice, add more explicit pairs as needed.
	pairs := [][2]int{}
	for i := 0; i < len(providers); i++ {
		for j := i + 1; j < len(providers); j++ {
			pairs = append(pairs, [2]int{i, j})
		}
	}
	for _, pair := range pairs {
		a, b := providers[pair[0]], providers[pair[1]]
		incA, err := db.GetIncidentsByProvider(ctx, g.pool, a.ID)
		if err != nil {
			continue
		}
		incB, err := db.GetIncidentsByProvider(ctx, g.pool, b.ID)
		if err != nil {
			continue
		}
		slug := fmt.Sprintf("%s-vs-%s", a.Slug, b.Slug)
		dir := filepath.Join(g.cfg.OutputDir, "compare", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		data := compareData{
			Title:       fmt.Sprintf("%s vs %s Uptime Comparison — Was It Down?", a.Name, b.Name),
			Description: fmt.Sprintf("Side-by-side uptime and incident comparison between %s and %s.", a.Name, b.Name),
			Canonical:   fmt.Sprintf("%s/compare/%s", siteDomain, slug),
			ProviderA:   a,
			ProviderB:   b,
			Stats30A:    m30[a.Slug],
			Stats30B:    m30[b.Slug],
			Stats90A:    m90[a.Slug],
			Stats90B:    m90[b.Slug],
			IncidentsA:  incA,
			IncidentsB:  incB,
			Generated:   time.Now().UTC(),
		}
		if err := g.render("compare.html", filepath.Join(dir, "index.html"), data); err != nil {
			return err
		}
	}
	return nil
}

// generateJSON writes machine-readable JSON data feeds for API consumers.
func (g *Generator) generateJSON(
	ctx context.Context,
	providers []models.Provider,
	m30, _, _ map[string]models.UptimeStats,
	recent []models.Incident,
) error {
	apiDir := filepath.Join(g.cfg.OutputDir, "api", "v1")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(apiDir, "providers.json"), providers); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(apiDir, "recent.json"), recent); err != nil {
		return err
	}

	statsOut := make(map[string]models.UptimeStats)
	for k, v := range m30 {
		statsOut[k] = v
	}
	if err := writeJSON(filepath.Join(apiDir, "uptime.json"), statsOut); err != nil {
		return err
	}
	return nil
}

func (g *Generator) generateStaticPages() error {
	pages := []struct {
		tmpl      string
		out       string
		title     string
		desc      string
		canonical string
	}{
		{
			tmpl:      "privacy.html",
			out:       filepath.Join(g.cfg.OutputDir, "privacy", "index.html"),
			title:     "Privacy Policy — wasitdown.dev",
			desc:      "Privacy policy for wasitdown.dev — what data we collect, cookies, advertising, and GDPR information.",
			canonical: siteDomain + "/privacy/",
		},
	}
	for _, p := range pages {
		if err := os.MkdirAll(filepath.Dir(p.out), 0o755); err != nil {
			return err
		}
		data := staticData{
			Title:       p.title,
			Description: p.desc,
			Canonical:   p.canonical,
			Generated:   time.Now().UTC(),
		}
		if err := g.render(p.tmpl, p.out, data); err != nil {
			return err
		}
	}
	return nil
}

// generateSitemap writes dist/sitemap.xml covering all generated URLs.
func (g *Generator) generateSitemap(ctx context.Context, providers []models.Provider) error {
	type url struct {
		Loc        string  `xml:"loc"`
		ChangeFreq string  `xml:"changefreq,omitempty"`
		Priority   float64 `xml:"priority,omitempty"`
	}
	type urlset struct {
		XMLName xml.Name `xml:"urlset"`
		Xmlns   string   `xml:"xmlns,attr"`
		URLs    []url    `xml:"url"`
	}

	urls := []url{
		{Loc: siteDomain + "/", ChangeFreq: "hourly", Priority: 1.0},
		{Loc: siteDomain + "/privacy/", ChangeFreq: "monthly", Priority: 0.3},
	}

	for _, p := range providers {
		urls = append(urls, url{
			Loc:        fmt.Sprintf("%s/provider/%s/", siteDomain, p.Slug),
			ChangeFreq: "hourly",
			Priority:   0.8,
		})
	}

	dates, err := db.GetDistinctIncidentDates(ctx, g.pool)
	if err != nil {
		return fmt.Errorf("get dates: %w", err)
	}
	for _, d := range dates {
		urls = append(urls, url{
			Loc:        fmt.Sprintf("%s/date/%s/", siteDomain, d.UTC().Format("2006-01-02")),
			ChangeFreq: "weekly",
			Priority:   0.5,
		})
	}

	ids, err := db.GetAllIncidentIDs(ctx, g.pool)
	if err != nil {
		return fmt.Errorf("get incident IDs: %w", err)
	}
	for _, id := range ids {
		urls = append(urls, url{
			Loc:      fmt.Sprintf("%s/incident/%d/", siteDomain, id),
			Priority: 0.6,
		})
	}

	for i := 0; i < len(providers); i++ {
		for j := i + 1; j < len(providers); j++ {
			urls = append(urls, url{
				Loc:        fmt.Sprintf("%s/compare/%s-vs-%s/", siteDomain, providers[i].Slug, providers[j].Slug),
				ChangeFreq: "daily",
				Priority:   0.4,
			})
		}
	}

	out := urlset{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}

	f, err := os.Create(filepath.Join(g.cfg.OutputDir, "sitemap.xml"))
	if err != nil {
		return err
	}

	if _, err := f.WriteString(xml.Header); err != nil {
		_ = f.Close()
		return err
	}
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(out); err != nil {
		_ = f.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// generateFeed writes dist/feed.xml — an Atom feed of the 50 most recent incidents.
func (g *Generator) generateFeed(ctx context.Context) error {
	type feedLink struct {
		Rel  string `xml:"rel,attr,omitempty"`
		Type string `xml:"type,attr,omitempty"`
		Href string `xml:"href,attr"`
	}
	type feedEntry struct {
		XMLName   xml.Name   `xml:"entry"`
		Title     string     `xml:"title"`
		Link      feedLink   `xml:"link"`
		ID        string     `xml:"id"`
		Updated   string     `xml:"updated"`
		Summary   string     `xml:"summary"`
	}
	type feed struct {
		XMLName  xml.Name    `xml:"feed"`
		XMLNS    string      `xml:"xmlns,attr"`
		Title    string      `xml:"title"`
		Subtitle string      `xml:"subtitle"`
		Link     []feedLink  `xml:"link"`
		ID       string      `xml:"id"`
		Updated  string      `xml:"updated"`
		Entries  []feedEntry `xml:"entry"`
	}

	incidents, err := db.GetRecentIncidents(ctx, g.pool, 50)
	if err != nil {
		return err
	}

	entries := make([]feedEntry, 0, len(incidents))
	for _, inc := range incidents {
		providerSlug := inc.ProviderSlug
		url := fmt.Sprintf("%s/incident/%s/%s/", siteDomain, providerSlug, inc.ExternalID)
		summary := fmt.Sprintf("[%s] %s — impact: %s, status: %s",
			inc.ProviderName, inc.Title, inc.Impact, inc.Status)
		entries = append(entries, feedEntry{
			Title:   fmt.Sprintf("%s: %s", inc.ProviderName, inc.Title),
			Link:    feedLink{Rel: "alternate", Href: url},
			ID:      url,
			Updated: inc.StartedAt.UTC().Format(time.RFC3339),
			Summary: summary,
		})
	}

	updated := time.Now().UTC().Format(time.RFC3339)
	if len(entries) > 0 {
		updated = entries[0].Updated
	}

	out := feed{
		XMLNS:    "http://www.w3.org/2005/Atom",
		Title:    "wasitdown.dev — Recent Incidents",
		Subtitle: "Historical cloud & AI provider incident tracker",
		Link: []feedLink{
			{Rel: "self", Type: "application/atom+xml", Href: siteDomain + "/feed.xml"},
			{Rel: "alternate", Type: "text/html", Href: siteDomain},
		},
		ID:      siteDomain + "/feed.xml",
		Updated: updated,
		Entries: entries,
	}

	path := filepath.Join(g.cfg.OutputDir, "feed.xml")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<?xml-stylesheet type="text/xsl" href="/feed.xsl"?>` + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	enc := xml.NewEncoder(f)
	enc.Indent("", "  ")
	if err := enc.Encode(out); err != nil {
		_ = f.Close()
		return err
	}
	if err := enc.Close(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// copyStatic copies the static/ directory tree into dist/ so assets are served.
func (g *Generator) copyStatic() error {
	return filepath.WalkDir(g.cfg.StaticDir, func(src string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(g.cfg.StaticDir, src)
		if err != nil {
			return err
		}
		dst := filepath.Join(g.cfg.OutputDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

// --- helpers ---

func (g *Generator) render(tmplName, outPath string, data any) error {
	t, ok := g.templates[tmplName]
	if !ok {
		return fmt.Errorf("template not found: %s", tmplName)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	if err := t.ExecuteTemplate(f, tmplName, data); err != nil {
		_ = f.Close()
		return fmt.Errorf("render %s: %w", tmplName, err)
	}
	return f.Close()
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// findConcurrentGroups groups incidents that started within `window` of each other
// AND span multiple providers.
func findConcurrentGroups(incidents []models.Incident, window time.Duration) [][]models.Incident {
	if len(incidents) < 2 {
		return nil
	}
	sorted := make([]models.Incident, len(incidents))
	copy(sorted, incidents)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartedAt.Before(sorted[j].StartedAt)
	})

	var groups [][]models.Incident
	used := make([]bool, len(sorted))

	for i := range sorted {
		if used[i] {
			continue
		}
		group := []models.Incident{sorted[i]}
		providers := map[int]bool{sorted[i].ProviderID: true}
		for j := i + 1; j < len(sorted); j++ {
			if used[j] {
				continue
			}
			if sorted[j].StartedAt.Sub(sorted[i].StartedAt) <= window {
				if !providers[sorted[j].ProviderID] {
					group = append(group, sorted[j])
					providers[sorted[j].ProviderID] = true
					used[j] = true
				}
			}
		}
		if len(group) >= 2 {
			used[i] = true
			groups = append(groups, group)
		}
	}
	return groups
}
