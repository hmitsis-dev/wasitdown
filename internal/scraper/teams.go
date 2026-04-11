package scraper

// TeamsScraper scrapes the Microsoft 365 service health public RSS feed.
//
// Microsoft does not publish a machine-readable incident history API without
// admin credentials. The best available public signal is the RSS feed at
// https://status.office365.com/api/feed — it reflects the same information
// shown on the public M365 Service Health page.
//
// Each RSS item may represent a Teams-specific incident or a broader M365
// service event. The title is used to infer Teams relevance; all items are
// ingested regardless since Teams depends on the whole M365 stack.

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// m365RSS matches the Microsoft 365 service health Atom/RSS feed.
type m365RSS struct {
	XMLName xml.Name    `xml:"rss"`
	Channel m365Channel `xml:"channel"`
}

// m365Atom matches the Atom variant Microsoft sometimes serves.
type m365Atom struct {
	XMLName xml.Name    `xml:"feed"`
	Entries []m365Entry `xml:"entry"`
}

type m365Channel struct {
	Items []m365Item `xml:"item"`
}

type m365Item struct {
	GUID        string `xml:"guid"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Link        string `xml:"link"`
}

type m365Entry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Updated string `xml:"updated"`
	Link    struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
}

// TeamsScraper handles the Microsoft 365 / Teams status feed.
type TeamsScraper struct{}

func (t *TeamsScraper) Slug() string { return "teams" }

func (t *TeamsScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, p.APIURL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	// Try RSS first, then Atom.
	items, err := parseM365Feed(body)
	if err != nil {
		return fmt.Errorf("parse M365 feed: %w", err)
	}

	for _, item := range items {
		if item.pubDate == "" && item.title == "" {
			continue
		}
		pubTime, parseErr := parseRSSDate(item.pubDate)
		if parseErr != nil {
			slog.Warn("teams: bad pubDate", "date", item.pubDate, "err", parseErr)
			continue
		}

		externalID := item.guid
		if externalID == "" {
			externalID = item.link
		}

		status := inferM365Status(item.title, item.description)
		impact := classifyImpactFromTitle(item.title)

		inc := &models.Incident{
			ProviderID: p.ID,
			ExternalID: externalID,
			Title:      sanitizeM365Title(item.title),
			Impact:     impact,
			Status:     status,
			StartedAt:  pubTime,
		}
		incID, err := db.UpsertIncident(ctx, pool, inc)
		if err != nil {
			slog.Error("upsert teams incident", "guid", externalID, "err", err)
			continue
		}

		body := item.description
		if body == "" {
			body = item.title
		}
		if body != "" {
			upd := &models.IncidentUpdate{
				IncidentID: incID,
				Body:       body,
				Status:     status,
				CreatedAt:  pubTime,
			}
			if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
				slog.Error("upsert teams update", "err", err)
			}
		}
	}
	return nil
}

// m365FeedItem is a normalised item from either RSS or Atom format.
type m365FeedItem struct {
	guid        string
	title       string
	description string
	pubDate     string
	link        string
}

func parseM365Feed(data []byte) ([]m365FeedItem, error) {
	// Try RSS.
	var rss m365RSS
	if err := xml.Unmarshal(data, &rss); err == nil && len(rss.Channel.Items) > 0 {
		items := make([]m365FeedItem, len(rss.Channel.Items))
		for i, it := range rss.Channel.Items {
			items[i] = m365FeedItem{
				guid:        it.GUID,
				title:       it.Title,
				description: it.Description,
				pubDate:     it.PubDate,
				link:        it.Link,
			}
		}
		return items, nil
	}

	// Try Atom.
	var atom m365Atom
	if err := xml.Unmarshal(data, &atom); err == nil && len(atom.Entries) > 0 {
		items := make([]m365FeedItem, len(atom.Entries))
		for i, e := range atom.Entries {
			items[i] = m365FeedItem{
				guid:        e.ID,
				title:       e.Title,
				description: e.Summary,
				pubDate:     e.Updated,
				link:        e.Link.Href,
			}
		}
		return items, nil
	}

	return nil, fmt.Errorf("feed is neither valid RSS nor Atom (len=%d)", len(data))
}

// inferM365Status inspects M365 title/body text for status signals.
func inferM365Status(title, body string) string {
	combined := strings.ToLower(title + " " + body)
	if strings.Contains(combined, "restored") ||
		strings.Contains(combined, "resolved") ||
		strings.Contains(combined, "mitigated") {
		return "resolved"
	}
	return "investigating"
}

// sanitizeM365Title strips common M365 RSS boilerplate prefixes.
func sanitizeM365Title(title string) string {
	prefixes := []string{
		"[Resolved] ",
		"[Mitigated] ",
		"[Investigating] ",
		"[Post-incident review] ",
	}
	for _, p := range prefixes {
		title = strings.TrimPrefix(title, p)
	}
	return strings.TrimSpace(title)
}
