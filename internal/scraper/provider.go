package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Provider defines the interface every status source must implement.
type Provider interface {
	// Slug returns the unique identifier matching the DB slug column.
	Slug() string
	// Scrape fetches incidents and upserts them into the DB. Returns error on failure.
	Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error
}

// httpGet performs a GET with a 15s timeout and returns the body bytes.
func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "wasitdown-scraper/1.0 (+https://wasitdown.dev)")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// normalizeImpact maps provider-specific impact strings to our canonical set.
func normalizeImpact(raw string) models.Impact {
	switch raw {
	case "critical":
		return models.ImpactCritical
	case "major":
		return models.ImpactMajor
	case "minor":
		return models.ImpactMinor
	default:
		return models.ImpactNone
	}
}

// durationMinutes computes the integer minutes between two times, returning nil if end is nil.
func durationMinutes(start time.Time, end *time.Time) *int {
	if end == nil {
		return nil
	}
	mins := int(math.Round(end.Sub(start).Minutes()))
	if mins < 0 {
		mins = 0
	}
	return &mins
}

// --- Statuspage scraper (handles Anthropic, OpenAI, Cloudflare, GitHub, Vercel, Groq, Mistral) ---

type StatuspageScraper struct {
	slug string
}

func (s *StatuspageScraper) Slug() string { return s.slug }

func (s *StatuspageScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, p.APIURL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	var resp models.StatuspageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	for _, si := range resp.Incidents {
		dur := durationMinutes(si.CreatedAt, si.ResolvedAt)
		inc := &models.Incident{
			ProviderID:      p.ID,
			ExternalID:      si.ID,
			Title:           si.Name,
			Impact:          normalizeImpact(si.Impact),
			Status:          si.Status,
			StartedAt:       si.CreatedAt,
			ResolvedAt:      si.ResolvedAt,
			DurationMinutes: dur,
		}
		incID, err := db.UpsertIncident(ctx, pool, inc)
		if err != nil {
			slog.Error("upsert incident", "provider", s.slug, "external_id", si.ID, "err", err)
			continue
		}
		for _, u := range si.IncidentUpdates {
			upd := &models.IncidentUpdate{
				IncidentID: incID,
				Body:       u.Body,
				Status:     u.Status,
				CreatedAt:  u.CreatedAt,
			}
			if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
				slog.Error("upsert update", "provider", s.slug, "err", err)
			}
		}
	}
	return nil
}

// AllProviders returns the complete list of scrapers, one per data source.
// To add a new Atlassian Statuspage provider, just add a StatuspageScraper
// entry here and seed its row in db/migrations/. No other code needed.
func AllProviders() []Provider {
	return []Provider{
		// --- Cloud & AI ---
		&StatuspageScraper{slug: "anthropic"},
		&StatuspageScraper{slug: "openai"},
		&StatuspageScraper{slug: "cloudflare"},
		&StatuspageScraper{slug: "github"},
		&StatuspageScraper{slug: "vercel"},
		&StatuspageScraper{slug: "groq"},
		&GCPScraper{},
		&AzureScraper{},
		&AWSScraper{},

		// --- Communication & collaboration ---
		&StatuspageScraper{slug: "discord"},
		&StatuspageScraper{slug: "zoom"},
	}
}
