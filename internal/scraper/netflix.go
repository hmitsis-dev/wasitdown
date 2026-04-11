package scraper

// NetflixScraper fetches Netflix streaming service status.
//
// Netflix does not publish a machine-readable incident API. Their status
// information is surfaced at https://help.netflix.com/en/node/100649 as
// rendered HTML. This scraper:
//
//  1. Fetches the raw HTML with a normal HTTP GET.
//  2. Looks for known status signal patterns in the HTML body.
//  3. If the page indicates a known problem ("We're aware of…", "Some members
//     may be experiencing…"), it synthesises an incident record.
//  4. If no signal is found, nothing is written — silence means all-clear.
//
// Because there is no unique incident ID exposed, the scraper derives a stable
// external ID by hashing the page date + headline so re-runs don't duplicate
// rows. All synthesised incidents are marked as "investigating" until a
// resolved signal appears on the same page, at which point they stay resolved
// permanently (idempotent upsert).
//
// Limitation: Netflix's status page gives almost no historical depth — at most
// it reflects the current day. If the goal is history, Netflix outage dates
// will remain sparse until the scraper has been running continuously.

import (
	"context"
	"crypto/md5"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// netflixStatusURL is the public Netflix service status help page.
const netflixStatusURL = "https://help.netflix.com/en/node/100649"

// Patterns that indicate an active or recent Netflix incident in the page HTML.
var (
	netflixProblemRe = regexp.MustCompile(`(?i)(we('re| are) (aware|experiencing)|some members may|streaming (issues|problems)|service (disruption|outage)|unable to (stream|watch|play)|playback (issue|error)|connection (problem|error))`)
	netflixResolveRe = regexp.MustCompile(`(?i)(resolved|restored|back to normal|working (as expected|normally))`)

	// Strips HTML tags so we're matching plain text.
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
)

// NetflixScraper handles the Netflix streaming status page.
type NetflixScraper struct{}

func (n *NetflixScraper) Slug() string { return "netflix" }

func (n *NetflixScraper) Scrape(ctx context.Context, pool *pgxpool.Pool, p models.Provider) error {
	body, err := httpGet(ctx, netflixStatusURL)
	if err != nil {
		return fmt.Errorf("fetch netflix status page: %w", err)
	}

	text := htmlTagRe.ReplaceAllString(string(body), " ")
	text = strings.Join(strings.Fields(text), " ") // collapse whitespace

	if !netflixProblemRe.MatchString(text) {
		// No incident signal detected — all good.
		slog.Info("netflix: no incident signal on status page")
		return nil
	}

	// Determine status.
	status := "investigating"
	if netflixResolveRe.MatchString(text) {
		status = "resolved"
	}

	// Extract a short headline (up to 200 chars around the first match).
	headline := extractNetflixHeadline(text)

	// Derive a stable external ID: md5(date + first 64 chars of headline).
	today := time.Now().UTC().Format("2006-01-02")
	key := today + headline[:min(64, len(headline))]
	h := md5.Sum([]byte(key))
	externalID := fmt.Sprintf("netflix-%s-%x", today, h[:4])

	inc := &models.Incident{
		ProviderID: p.ID,
		ExternalID: externalID,
		Title:      headline,
		Impact:     models.ImpactMajor, // consumer-facing outages are always felt widely
		Status:     status,
		StartedAt:  time.Now().UTC().Truncate(time.Hour), // best approximation
	}
	if status == "resolved" {
		t := time.Now().UTC()
		inc.ResolvedAt = &t
	}

	incID, err := db.UpsertIncident(ctx, pool, inc)
	if err != nil {
		return fmt.Errorf("upsert netflix incident: %w", err)
	}

	upd := &models.IncidentUpdate{
		IncidentID: incID,
		Body:       fmt.Sprintf("Detected via Netflix status page (%s). Snippet: %s", netflixStatusURL, headline),
		Status:     status,
		CreatedAt:  time.Now().UTC(),
	}
	if err := db.UpsertIncidentUpdate(ctx, pool, upd); err != nil {
		slog.Error("upsert netflix update", "err", err)
	}

	slog.Info("netflix: incident recorded", "status", status, "headline", headline)
	return nil
}

// extractNetflixHeadline grabs the first ~180 chars around the problem keyword.
func extractNetflixHeadline(text string) string {
	loc := netflixProblemRe.FindStringIndex(text)
	if loc == nil {
		return "Netflix service disruption detected"
	}
	start := loc[0] - 40
	if start < 0 {
		start = 0
	}
	end := loc[1] + 140
	if end > len(text) {
		end = len(text)
	}
	snippet := strings.TrimSpace(text[start:end])
	// Trim at sentence boundary if possible.
	if i := strings.IndexAny(snippet[40:], ".!?"); i >= 0 {
		snippet = snippet[:40+i+1]
	}
	return snippet
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
