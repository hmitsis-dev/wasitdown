package scraper

import (
	"context"
	"log/slog"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunAll fetches all providers from the DB, matches them to their Scraper
// implementation, runs each scraper, and logs the result. Never panics.
func RunAll(ctx context.Context, pool *pgxpool.Pool) {
	providers, err := db.GetAllProviders(ctx, pool)
	if err != nil {
		slog.Error("load providers", "err", err)
		return
	}

	scrapers := make(map[string]Provider, len(AllProviders()))
	for _, s := range AllProviders() {
		scrapers[s.Slug()] = s
	}

	for _, p := range providers {
		scraper, ok := scrapers[p.Slug]
		if !ok {
			slog.Warn("no scraper for provider", "slug", p.Slug)
			continue
		}

		slog.Info("scraping", "provider", p.Slug)
		scrapeErr := scraper.Scrape(ctx, pool, p)

		errMsg := ""
		success := scrapeErr == nil
		if scrapeErr != nil {
			errMsg = scrapeErr.Error()
			slog.Error("scrape failed", "provider", p.Slug, "err", scrapeErr)
		} else {
			slog.Info("scrape ok", "provider", p.Slug)
		}

		if logErr := db.LogScrape(ctx, pool, p.ID, success, errMsg); logErr != nil {
			slog.Error("log scrape", "provider", p.Slug, "err", logErr)
		}
	}
}
