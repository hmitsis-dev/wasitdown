package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/hmitsis-dev/wasitdown/internal/db"
	"github.com/hmitsis-dev/wasitdown/internal/generator"
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

	outputDir := "dist"
	if v := os.Getenv("OUTPUT_DIR"); v != "" {
		outputDir = v
	}
	templatesDir := "templates"
	if v := os.Getenv("TEMPLATES_DIR"); v != "" {
		templatesDir = v
	}
	staticDir := "static"
	if v := os.Getenv("STATIC_DIR"); v != "" {
		staticDir = v
	}

	cfg := generator.Config{
		OutputDir:       outputDir,
		TemplatesDir:    templatesDir,
		StaticDir:       staticDir,
		AdsEnabled:      os.Getenv("ADS_ENABLED") == "true",
		GAMeasurementID: os.Getenv("GA_MEASUREMENT_ID"),
	}

	gen, err := generator.New(pool, cfg)
	if err != nil {
		slog.Error("init generator", "err", err)
		os.Exit(1)
	}

	if err := gen.Run(ctx); err != nil {
		slog.Error("generate site", "err", err)
		os.Exit(1)
	}
	slog.Info("done", "output", outputDir)
}
