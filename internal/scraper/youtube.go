package scraper

// YouTubeScraper fetches YouTube-specific incidents from Google's public
// status infrastructure.
//
// Google does not publish a separate Atlassian Statuspage for YouTube. Instead,
// YouTube outages are reported on https://status.google.com which covers all
// consumer Google products. Google exposes this data as a JSON array — the same
// format as the GCP incidents feed but hosted at a different domain.
//
// The scraper fetches the feed and filters for entries that mention YouTube
// (by product name or service name). Non-YouTube GCP incidents are skipped
// so this provider doesn't duplicate the existing GCP scraper.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// googleStatusFeed matches the JSON format served by status.google.com.
// It is structurally similar to the GCP feed but uses different field names.
type googleStatusFeed []googleStatusItem

type googleStatusItem struct {
	// Both naming conventions appear in practice.
	ID          string             `json:"id"`
	ExternalID  string             `json:"external-desc"`
	Description string             `json:"description"`
	Service     string             `json:"service_name"`
	Severity    string             `json:"severity"`
	Begin       googleTime         `json:"begin"`
	End         *googleTime        `json:"end"`
	Updates     []googleStatusUpd  `json:"updates"`
	Products    []string           `json:"affected_products"`
}

type googleStatusUpd struct {
	Created googleTime `json:"created"`
	Text    string     `json:"text"`
}

// googleTime handles Google's non-standard epoch-millisecond timestamps as
// well as RFC3339 strings transparently.
type googleTime struct {
	time.Time
}

func (g *googleTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		return nil
	}
	// Try RFC3339 first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		g.Time = t
		return nil
	}
	// Some Google feeds embed epoch-milliseconds as a number string.
	var ms int64
	if err := json.Unmarshal(b, &ms); err == nil {
		g.Time = time.UnixMilli(ms).UTC()
		return nil
	}
	// Additional formats seen in the wild.
	for _, f := range []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05 MST",
	} {
		if t, err := time.Parse(f, s); err == nil {
			g.Time = t.UTC()
			return nil
		}
	}
	return fmt.Errorf("googleTime: cannot parse %q", s)
}

// YouTubeScraper handles the Google consumer status page for YouTube.
type YouTubeScraper struct{}

func (y *YouTubeScraper) Slug() string { return "youtube" }

func (y *YouTubeScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	// Google's consumer status JSON — same schema root as GCP but a different host.
	// We derive the incidents endpoint from the status page URL.
	apiURL := "https://status.google.com/incidents.json"

	body, err := httpGet(ctx, apiURL)
	if err != nil {
		return fmt.Errorf("fetch google status: %w", err)
	}

	var feed googleStatusFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return fmt.Errorf("parse google status: %w", err)
	}

	ingested := 0
	for _, item := range feed {
		if !isYouTubeIncident(item) {
			continue
		}

		externalID := item.ID
		if externalID == "" {
			externalID = item.ExternalID
		}
		if externalID == "" {
			continue
		}

		title := item.Description
		if title == "" {
			title = "YouTube service disruption"
		}

		var endTime *time.Time
		if item.End != nil && !item.End.IsZero() {
			t := item.End.Time
			endTime = &t
		}

		status := "resolved"
		if endTime == nil {
			status = "investigating"
		}

		impact := mapGoogleSeverity(item.Severity)
		dur := durationMinutes(item.Begin.Time, endTime)

		inc := &models.Incident{
			ProviderID:      p.ID,
			ExternalID:      externalID,
			Title:           title,
			Impact:          impact,
			Status:          status,
			StartedAt:       item.Begin.Time,
			ResolvedAt:      endTime,
			DurationMinutes: dur,
		}
		incID, err := db.UpsertIncident(ctx, pool, inc)
		if err != nil {
			slog.Error("upsert youtube incident", "id", externalID, "err", err)
			continue
		}

		for _, u := range item.Updates {
			upd := &models.IncidentUpdate{
				IncidentID: incID,
				Body:       u.Text,
				Status:     status,
				CreatedAt:  u.Created.Time,
			}
			if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
				slog.Error("upsert youtube update", "err", err)
			}
		}
		ingested++
	}

	slog.Info("youtube: ingested incidents", "count", ingested, "total_in_feed", len(feed))
	return nil
}

// isYouTubeIncident returns true when an item is YouTube-related.
func isYouTubeIncident(item googleStatusItem) bool {
	needle := "youtube"
	if strings.Contains(strings.ToLower(item.Service), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(item.Description), needle) {
		return true
	}
	for _, prod := range item.Products {
		if strings.Contains(strings.ToLower(prod), needle) {
			return true
		}
	}
	return false
}

// mapGoogleSeverity converts Google severity strings to our canonical Impact.
func mapGoogleSeverity(s string) models.Impact {
	switch strings.ToLower(s) {
	case "high", "critical":
		return models.ImpactMajor
	case "medium":
		return models.ImpactMinor
	default:
		return models.ImpactNone
	}
}
