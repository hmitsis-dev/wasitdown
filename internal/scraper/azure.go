package scraper

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// azureRSS matches the Azure status RSS feed structure.
type azureRSS struct {
	XMLName xml.Name      `xml:"rss"`
	Channel azureChannel  `xml:"channel"`
}

type azureChannel struct {
	Items []azureItem `xml:"item"`
}

type azureItem struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Link        string `xml:"link"`
}

// AzureScraper parses the Azure status RSS feed.
type AzureScraper struct{}

func (a *AzureScraper) Slug() string { return "azure" }

func (a *AzureScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, p.APIURL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	var feed azureRSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return fmt.Errorf("parse rss: %w", err)
	}

	for _, item := range feed.Channel.Items {
		pubTime, err := parseRSSDate(item.PubDate)
		if err != nil {
			slog.Warn("azure: bad pubDate", "date", item.PubDate, "err", err)
			pubTime = time.Now().UTC()
		}

		externalID := item.GUID
		if externalID == "" {
			externalID = item.Link
		}

		impact := classifyImpactFromTitle(item.Title)
		status := "resolved"
		if strings.Contains(strings.ToLower(item.Title), "investigating") ||
			strings.Contains(strings.ToLower(item.Description), "investigating") {
			status = "investigating"
		}

		inc := &models.Incident{
			ProviderID: p.ID,
			ExternalID: externalID,
			Title:      item.Title,
			Impact:     impact,
			Status:     status,
			StartedAt:  pubTime,
		}
		incID, err := db.UpsertIncident(ctx, pool, inc)
		if err != nil {
			slog.Error("upsert azure incident", "guid", externalID, "err", err)
			continue
		}

		if item.Description != "" {
			upd := &models.IncidentUpdate{
				IncidentID: incID,
				Body:       item.Description,
				Status:     status,
				CreatedAt:  pubTime,
			}
			if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
				slog.Error("upsert azure update", "err", err)
			}
		}
	}
	return nil
}

// parseRSSDate tries multiple common RSS date formats.
func parseRSSDate(s string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as RSS date", s)
}

// classifyImpactFromTitle makes a rough guess at impact from free-form title text.
func classifyImpactFromTitle(title string) models.Impact {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "outage") || strings.Contains(lower, "unavailable"):
		return models.ImpactMajor
	case strings.Contains(lower, "degraded") || strings.Contains(lower, "performance"):
		return models.ImpactMinor
	case strings.Contains(lower, "advisory") || strings.Contains(lower, "warning"):
		return models.ImpactNone
	default:
		return models.ImpactMinor
	}
}
