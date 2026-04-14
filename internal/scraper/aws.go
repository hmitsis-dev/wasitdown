package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"
	"unicode/utf16"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// awsEvent matches the JSON objects in the AWS Health public current-events feed.
// The endpoint (https://health.aws.amazon.com/public/currentevents) returns a
// UTF-16 BE JSON array with a BOM. Each object represents one open or recent event.
type awsEvent struct {
	ARN          string         `json:"arn"`
	Date         string         `json:"date"` // Unix epoch seconds as string
	RegionName   string         `json:"region_name"`
	Status       string         `json:"status"` // "1" = open/active, others may vary
	Service      string         `json:"service"`
	ServiceName  string         `json:"service_name"`
	Summary      string         `json:"summary"`
	EventLog     []awsEventLog  `json:"event_log"`
}

type awsEventLog struct {
	Summary string `json:"summary"`
	Message string `json:"message"`
}

// AWSScraper scrapes the AWS Health public JSON feed.
type AWSScraper struct{}

func (a *AWSScraper) Slug() string { return "aws" }

func (a *AWSScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, p.APIURL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	text, err := decodeUTF16(body)
	if err != nil {
		return fmt.Errorf("decode utf-16: %w", err)
	}

	var events []awsEvent
	if err := json.Unmarshal([]byte(text), &events); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	for _, ev := range events {
		epochSecs, err := strconv.ParseInt(ev.Date, 10, 64)
		if err != nil {
			slog.Warn("aws: bad date", "date", ev.Date)
			epochSecs = time.Now().Unix()
		}
		startedAt := time.Unix(epochSecs, 0).UTC()

		// status "1" = open/active in this feed
		status := "investigating"
		impact := classifyImpactFromTitle(ev.Summary)

		title := ev.Summary
		if ev.ServiceName != "" && ev.RegionName != "" {
			title = fmt.Sprintf("[%s] %s - %s", ev.RegionName, ev.ServiceName, ev.Summary)
		}

		inc := &models.Incident{
			ProviderID: p.ID,
			ExternalID: ev.ARN,
			Title:      title,
			Impact:     impact,
			Status:     status,
			StartedAt:  startedAt,
		}
		incID, err := db.UpsertIncident(ctx, pool, inc)
		if err != nil {
			slog.Error("upsert aws incident", "arn", ev.ARN, "err", err)
			continue
		}

		for _, log := range ev.EventLog {
			body := log.Message
			if body == "" {
				body = log.Summary
			}
			if body == "" {
				continue
			}
			upd := &models.IncidentUpdate{
				IncidentID: incID,
				Body:       body,
				Status:     status,
				CreatedAt:  startedAt,
			}
			if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
				slog.Error("upsert aws update", "err", err)
			}
		}
	}
	return nil
}

// decodeUTF16 converts a UTF-16 byte slice (with or without BOM) to a UTF-8 string.
func decodeUTF16(b []byte) (string, error) {
	if len(b) < 2 {
		return string(b), nil
	}

	var order string
	start := 0
	switch {
	case b[0] == 0xFF && b[1] == 0xFE:
		order = "le"
		start = 2
	case b[0] == 0xFE && b[1] == 0xFF:
		order = "be"
		start = 2
	default:
		// No BOM — return as-is (likely already UTF-8)
		return string(b), nil
	}

	b = b[start:]
	if len(b)%2 != 0 {
		return "", fmt.Errorf("odd byte count for UTF-16 data")
	}

	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		if order == "le" {
			u16[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		} else {
			u16[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		}
	}
	return string(utf16.Decode(u16)), nil
}
