package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/scraper"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		slog.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	migrationsDir := "db/migrations"
	if v := os.Getenv("MIGRATIONS_DIR"); v != "" {
		migrationsDir = v
	}

	if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
		slog.Error("run migrations", "err", err)
		os.Exit(1)
	}

	// Determine run mode: "once" (default for cron) or "daemon" (polls every 15min).
	mode := os.Getenv("SCRAPER_MODE")
	if mode == "" {
		mode = "once"
	}

	slog.Info("scraper starting", "mode", mode)

	switch mode {
	case "daemon":
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		// Run immediately on startup.
		scraper.RunAll(ctx, pool)
		for {
			select {
			case <-ticker.C:
				scraper.RunAll(ctx, pool)
			case <-ctx.Done():
				return
			}
		}
	default:
		// "once" — run a single pass and exit (suitable for cron).
		scraper.RunAll(ctx, pool)
		slog.Info("scraper done")
	}
}
