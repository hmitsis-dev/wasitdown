package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GCPScraper handles the Google Cloud status JSON feed.
type GCPScraper struct{}

func (g *GCPScraper) Slug() string { return "gcp" }

func (g *GCPScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, p.APIURL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	// GCP feed is a JSON array at the top level.
	var items []models.GCPIncident
	if err := json.Unmarshal(body, &items); err != nil {
		// Try wrapped format.
		var feed models.GCPFeed
		if err2 := json.Unmarshal(body, &feed); err2 != nil {
			return fmt.Errorf("parse gcp feed: %w (array: %v)", err2, err)
		}
		items = feed.Incidents
	}

	for _, gi := range items {
		impact := models.ImpactMinor
		switch gi.Severity {
		case "high", "critical":
			impact = models.ImpactMajor
		case "low":
			impact = models.ImpactNone
		}

		status := "resolved"
		if gi.End == nil {
			status = "investigating"
		}

		externalID := gi.ID
		if externalID == "" {
			externalID = gi.ExternalID
		}

		dur := durationMinutes(gi.Begin, gi.End)
		inc := &models.Incident{
			ProviderID:      p.ID,
			ExternalID:      externalID,
			Title:           gi.Description,
			Impact:          impact,
			Status:          status,
			StartedAt:       gi.Begin,
			ResolvedAt:      gi.End,
			DurationMinutes: dur,
		}
		incID, err := db.UpsertIncident(ctx, pool, inc)
		if err != nil {
			slog.Error("upsert gcp incident", "external_id", externalID, "err", err)
			continue
		}

		for _, u := range gi.Updates {
			upd := &models.IncidentUpdate{
				IncidentID: incID,
				Body:       u.Text,
				Status:     status,
				CreatedAt:  u.Created,
			}
			if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
				slog.Error("upsert gcp update", "err", err)
			}
		}
	}
	return nil
}
