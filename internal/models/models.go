package models

import "time"

type ProviderType string

const (
	ProviderTypeStatuspage ProviderType = "statuspage"
	ProviderTypeRSS        ProviderType = "rss"
	ProviderTypeCustom     ProviderType = "custom"
)

type Impact string

const (
	ImpactNone     Impact = "none"
	ImpactMinor    Impact = "minor"
	ImpactMajor    Impact = "major"
	ImpactCritical Impact = "critical"
)

type Provider struct {
	ID            int          `json:"id"`
	Name          string       `json:"name"`
	Slug          string       `json:"slug"`
	StatusPageURL string       `json:"status_page_url"`
	APIURL        string       `json:"api_url"`
	Type          ProviderType `json:"type"`
	CreatedAt     time.Time    `json:"created_at"`
}

type Incident struct {
	ID              int        `json:"id"`
	ProviderID      int        `json:"provider_id"`
	ProviderName    string     `json:"provider_name,omitempty"`
	ProviderSlug    string     `json:"provider_slug,omitempty"`
	ExternalID      string     `json:"external_id"`
	Title           string     `json:"title"`
	Impact          Impact     `json:"impact"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	Updates         []IncidentUpdate `json:"updates,omitempty"`
}

func (i Incident) ImpactColor() string {
	switch i.Impact {
	case ImpactMinor:
		return "yellow"
	case ImpactMajor:
		return "orange"
	case ImpactCritical:
		return "red"
	default:
		return "gray"
	}
}

func (i Incident) IsResolved() bool {
	return i.ResolvedAt != nil
}

type IncidentUpdate struct {
	ID         int       `json:"id"`
	IncidentID int       `json:"incident_id"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type ScrapeLog struct {
	ID         int       `json:"id"`
	ProviderID int       `json:"provider_id"`
	ScrapedAt  time.Time `json:"scraped_at"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// UptimeStats holds computed uptime data for a provider over a window.
type UptimeStats struct {
	ProviderID      int     `json:"provider_id"`
	ProviderSlug    string  `json:"provider_slug"`
	ProviderName    string  `json:"provider_name"`
	Days            int     `json:"days"`
	UptimePct       float64 `json:"uptime_pct"`
	IncidentCount   int     `json:"incident_count"`
	AvgDuration     float64 `json:"avg_duration_minutes"`
	MedianDuration  float64 `json:"median_duration_minutes"`
	LongestDuration int     `json:"longest_duration_minutes"`
}

// ChaosPair holds two providers that have had concurrent incidents and how often.
type ChaosPair struct {
	SlugA  string `json:"slug_a"`
	NameA  string `json:"name_a"`
	SlugB  string `json:"slug_b"`
	NameB  string `json:"name_b"`
	Count  int    `json:"count"`
}

// DateCorrelation holds cross-provider incident data for a given date.
type DateCorrelation struct {
	Date      string     `json:"date"`
	Incidents []Incident `json:"incidents"`
	// Groups of incidents within a 2hr window across multiple providers.
	ConcurrentGroups [][]Incident `json:"concurrent_groups,omitempty"`
}

// StatuspageResponse matches the Atlassian Statuspage API.
type StatuspageResponse struct {
	Incidents []StatuspageIncident `json:"incidents"`
}

type StatuspageIncident struct {
	ID                 string                    `json:"id"`
	Name               string                    `json:"name"`
	Impact             string                    `json:"impact"`
	Status             string                    `json:"status"`
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
	ResolvedAt         *time.Time                `json:"resolved_at"`
	IncidentUpdates    []StatuspageIncidentUpdate `json:"incident_updates"`
	Shortlink          string                    `json:"shortlink"`
}

type StatuspageIncidentUpdate struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// GCPIncident matches the Google Cloud status JSON feed format.
type GCPFeed struct {
	Incidents []GCPIncident `json:"items"`
}

type GCPIncident struct {
	ID          string        `json:"id"`
	ExternalID  string        `json:"external-desc"`
	Description string        `json:"description"`
	Severity    string        `json:"severity"`
	Begin       time.Time     `json:"begin"`
	End         *time.Time    `json:"end,omitempty"`
	StatusTime  time.Time     `json:"status-time"`
	Updates     []GCPUpdate   `json:"updates"`
	CurrentlyAffected []string `json:"currently-affected-locations"`
}

type GCPUpdate struct {
	Created     time.Time `json:"created"`
	Text        string    `json:"text"`
	When        time.Time `json:"when"`
}
