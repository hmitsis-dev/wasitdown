package db

import (
	"context"
	"fmt"
	"time"

	"github.com/hmitsis-dev/wasitdown/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetAllProviders returns every provider.
func GetAllProviders(ctx context.Context, pool *pgxpool.Pool) ([]models.Provider, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, slug, status_page_url, api_url, type, created_at FROM providers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var out []models.Provider
	for rows.Next() {
		var p models.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.StatusPageURL, &p.APIURL, &p.Type, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProviderBySlug fetches a single provider by slug.
func GetProviderBySlug(ctx context.Context, pool *pgxpool.Pool, slug string) (*models.Provider, error) {
	var p models.Provider
	err := pool.QueryRow(ctx,
		`SELECT id, name, slug, status_page_url, api_url, type, created_at FROM providers WHERE slug=$1`, slug,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.StatusPageURL, &p.APIURL, &p.Type, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertIncident inserts or updates an incident record. Returns the incident ID.
func UpsertIncident(ctx context.Context, pool *pgxpool.Pool, inc *models.Incident) (int, error) {
	var id int
	err := pool.QueryRow(ctx, `
		INSERT INTO incidents (provider_id, external_id, title, impact, status, started_at, resolved_at, duration_minutes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider_id, external_id) DO UPDATE
		  SET title=$3, impact=$4, status=$5, started_at=$6, resolved_at=$7, duration_minutes=$8
		RETURNING id`,
		inc.ProviderID, inc.ExternalID, inc.Title, inc.Impact, inc.Status,
		inc.StartedAt, inc.ResolvedAt, inc.DurationMinutes,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert incident %s: %w", inc.ExternalID, err)
	}
	return id, nil
}

// UpsertIncidentUpdate inserts or updates an incident update (idempotent by incident_id+created_at).
func UpsertIncidentUpdate(ctx context.Context, pool *pgxpool.Pool, u *models.IncidentUpdate) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO incident_updates (incident_id, body, status, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`,
		u.IncidentID, u.Body, u.Status, u.CreatedAt,
	)
	return err
}

// LogScrape writes a scrape result entry.
func LogScrape(ctx context.Context, pool *pgxpool.Pool, providerID int, success bool, errMsg string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO scrape_log (provider_id, scraped_at, success, error) VALUES ($1, NOW(), $2, $3)`,
		providerID, success, errMsg,
	)
	return err
}

// GetTodayIncidents returns:
//   - Unresolved incidents started within the last 7 days (avoids stale "live" entries
//     from scrapers that never set resolved_at, e.g. AWS)
//   - Any incident that started today (UTC), resolved or not
//
// Active incidents come first, then resolved, both newest-first within each group.
func GetTodayIncidents(ctx context.Context, pool *pgxpool.Pool) ([]models.Incident, error) {
	todayUTC := time.Now().UTC().Truncate(24 * time.Hour)
	staleCutoff := time.Now().UTC().AddDate(0, 0, -7)
	rows, err := pool.Query(ctx, `
		SELECT i.id, i.provider_id, p.name, p.slug, i.external_id, i.title, i.impact,
		       i.status, i.started_at, i.resolved_at, i.duration_minutes, i.created_at
		FROM incidents i
		JOIN providers p ON p.id = i.provider_id
		WHERE (i.resolved_at IS NULL AND i.started_at >= $2)
		   OR i.started_at >= $1
		ORDER BY i.resolved_at NULLS FIRST, i.started_at DESC`, todayUTC, staleCutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIncidents(rows)
}

// GetRecentIncidents returns incidents newer than cutoff, newest first, across all providers.
func GetRecentIncidents(ctx context.Context, pool *pgxpool.Pool, limit int) ([]models.Incident, error) {
	rows, err := pool.Query(ctx, `
		SELECT i.id, i.provider_id, p.name, p.slug, i.external_id, i.title, i.impact,
		       i.status, i.started_at, i.resolved_at, i.duration_minutes, i.created_at
		FROM incidents i
		JOIN providers p ON p.id = i.provider_id
		ORDER BY i.started_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIncidents(rows)
}

// GetIncidentsByProvider returns all incidents for a given provider, newest first.
func GetIncidentsByProvider(ctx context.Context, pool *pgxpool.Pool, providerID int) ([]models.Incident, error) {
	rows, err := pool.Query(ctx, `
		SELECT i.id, i.provider_id, p.name, p.slug, i.external_id, i.title, i.impact,
		       i.status, i.started_at, i.resolved_at, i.duration_minutes, i.created_at
		FROM incidents i
		JOIN providers p ON p.id = i.provider_id
		WHERE i.provider_id = $1
		ORDER BY i.started_at DESC`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIncidents(rows)
}

// GetIncidentsByDate returns incidents that started on a given calendar date (UTC).
func GetIncidentsByDate(ctx context.Context, pool *pgxpool.Pool, date time.Time) ([]models.Incident, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	rows, err := pool.Query(ctx, `
		SELECT i.id, i.provider_id, p.name, p.slug, i.external_id, i.title, i.impact,
		       i.status, i.started_at, i.resolved_at, i.duration_minutes, i.created_at
		FROM incidents i
		JOIN providers p ON p.id = i.provider_id
		WHERE i.started_at >= $1 AND i.started_at < $2
		ORDER BY i.started_at ASC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIncidents(rows)
}

// GetIncidentByID returns a full incident with updates.
func GetIncidentByID(ctx context.Context, pool *pgxpool.Pool, id int) (*models.Incident, error) {
	var inc models.Incident
	err := pool.QueryRow(ctx, `
		SELECT i.id, i.provider_id, p.name, p.slug, i.external_id, i.title, i.impact,
		       i.status, i.started_at, i.resolved_at, i.duration_minutes, i.created_at
		FROM incidents i
		JOIN providers p ON p.id = i.provider_id
		WHERE i.id = $1`, id,
	).Scan(
		&inc.ID, &inc.ProviderID, &inc.ProviderName, &inc.ProviderSlug,
		&inc.ExternalID, &inc.Title, &inc.Impact, &inc.Status,
		&inc.StartedAt, &inc.ResolvedAt, &inc.DurationMinutes, &inc.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx,
		`SELECT id, incident_id, body, status, created_at FROM incident_updates WHERE incident_id=$1 ORDER BY created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u models.IncidentUpdate
		if err := rows.Scan(&u.ID, &u.IncidentID, &u.Body, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		inc.Updates = append(inc.Updates, u)
	}
	return &inc, rows.Err()
}

// GetAllIncidentIDs returns all incident IDs for the generator to iterate over.
func GetAllIncidentIDs(ctx context.Context, pool *pgxpool.Pool) ([]int, error) {
	rows, err := pool.Query(ctx, `SELECT id FROM incidents ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetDistinctIncidentDates returns every UTC date that has at least one incident.
func GetDistinctIncidentDates(ctx context.Context, pool *pgxpool.Pool) ([]time.Time, error) {
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT date_trunc('day', started_at AT TIME ZONE 'UTC') AS d FROM incidents ORDER BY d DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// GetUptimeStats computes uptime stats for all providers over the given window.
// Overlapping incidents are merged before summing downtime so that simultaneous
// regional outages (common for transparent providers like Cloudflare) are not
// double-counted. Uses PostgreSQL 14+ range_agg on tstzrange.
func GetUptimeStats(ctx context.Context, pool *pgxpool.Pool, days int) ([]models.UptimeStats, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	rows, err := pool.Query(ctx, `
		WITH base AS (
			SELECT p.id, p.slug, p.name,
			       COUNT(i.id) AS incident_count,
			       COALESCE(AVG(i.duration_minutes) FILTER (WHERE i.duration_minutes IS NOT NULL), 0) AS avg_dur,
			       COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY i.duration_minutes)
			           FILTER (WHERE i.duration_minutes IS NOT NULL), 0) AS median_dur,
			       COALESCE(MAX(i.duration_minutes), 0) AS max_dur,
			       -- Only major/critical incidents count toward downtime.
			       -- Minor degradations and maintenance (impact=none/minor) are
			       -- included in incident counts but do not reduce the uptime %.
			       range_agg(tstzrange(i.started_at, i.resolved_at))
			           FILTER (WHERE i.resolved_at IS NOT NULL
			               AND i.resolved_at > i.started_at
			               AND i.impact IN ('major', 'critical')) AS merged
			FROM providers p
			LEFT JOIN incidents i ON i.provider_id = p.id AND i.started_at >= $1
			GROUP BY p.id, p.slug, p.name
		)
		SELECT id, slug, name, incident_count, avg_dur, median_dur, max_dur,
		       COALESCE(
		           (SELECT SUM(EXTRACT(EPOCH FROM (upper(r) - lower(r)))/60)
		            FROM unnest(merged) AS r),
		           0
		       ) AS total_outage_min
		FROM base
		ORDER BY name`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totalMinutes := float64(days) * 24 * 60
	var out []models.UptimeStats
	for rows.Next() {
		var s models.UptimeStats
		var totalOutage float64
		if err := rows.Scan(&s.ProviderID, &s.ProviderSlug, &s.ProviderName,
			&s.IncidentCount, &s.AvgDuration, &s.MedianDuration, &s.LongestDuration, &totalOutage); err != nil {
			return nil, err
		}
		s.Days = days
		uptime := 1.0 - (totalOutage / totalMinutes)
		if uptime < 0 {
			uptime = 0
		}
		s.UptimePct = uptime * 100
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetChaosPairs returns provider pairs whose incidents overlapped within a 2-hour
// window most often. Only pairs with at least 2 co-occurrences are included.
func GetChaosPairs(ctx context.Context, pool *pgxpool.Pool, limit int) ([]models.ChaosPair, error) {
	rows, err := pool.Query(ctx, `
		SELECT p1.name, p1.slug, p2.name, p2.slug, COUNT(*) AS cnt
		FROM incidents i1
		JOIN incidents i2
		  ON i2.provider_id > i1.provider_id
		 AND ABS(EXTRACT(EPOCH FROM (i2.started_at - i1.started_at))) <= 7200
		JOIN providers p1 ON p1.id = i1.provider_id
		JOIN providers p2 ON p2.id = i2.provider_id
		GROUP BY p1.name, p1.slug, p2.name, p2.slug
		HAVING COUNT(*) >= 2
		ORDER BY cnt DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("chaos pairs: %w", err)
	}
	defer rows.Close()
	var out []models.ChaosPair
	for rows.Next() {
		var p models.ChaosPair
		if err := rows.Scan(&p.NameA, &p.SlugA, &p.NameB, &p.SlugB, &p.Count); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanIncidents(rows pgx.Rows) ([]models.Incident, error) {
	var out []models.Incident
	for rows.Next() {
		var inc models.Incident
		if err := rows.Scan(
			&inc.ID, &inc.ProviderID, &inc.ProviderName, &inc.ProviderSlug,
			&inc.ExternalID, &inc.Title, &inc.Impact, &inc.Status,
			&inc.StartedAt, &inc.ResolvedAt, &inc.DurationMinutes, &inc.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, inc)
	}
	return out, rows.Err()
}
