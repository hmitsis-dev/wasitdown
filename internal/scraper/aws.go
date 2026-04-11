package scraper

import (
	"context"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// awsRSS matches the AWS Health RSS feed.
type awsRSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel awsChannel `xml:"channel"`
}

type awsChannel struct {
	Items []awsItem `xml:"item"`
}

type awsItem struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Link        string `xml:"link"`
}

// AWSScraper scrapes the AWS Health RSS feed.
type AWSScraper struct{}

func (a *AWSScraper) Slug() string { return "aws" }

func (a *AWSScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, p.APIURL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	var feed awsRSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return fmt.Errorf("parse rss: %w", err)
	}

	for _, item := range feed.Channel.Items {
		pubTime, err := parseRSSDate(item.PubDate)
		if err != nil {
			slog.Warn("aws: bad pubDate", "date", item.PubDate, "err", err)
			pubTime = time.Now().UTC()
		}

		externalID := item.GUID
		if externalID == "" {
			// Derive stable ID from title+date.
			h := md5.Sum([]byte(item.Title + item.PubDate))
			externalID = fmt.Sprintf("%x", h)
		}

		// AWS titles look like: "RESOLVED: [us-east-1] EC2 - Increased API Error Rates"
		status := "resolved"
		if !strings.HasPrefix(strings.ToUpper(item.Title), "RESOLVED") {
			status = "investigating"
		}
		impact := classifyImpactFromTitle(item.Title)

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
			slog.Error("upsert aws incident", "guid", externalID, "err", err)
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
				slog.Error("upsert aws update", "err", err)
			}
		}
	}
	return nil
}
