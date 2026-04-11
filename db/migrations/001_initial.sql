-- Migration 001: Initial schema

CREATE TABLE IF NOT EXISTS providers (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    status_page_url TEXT NOT NULL,
    api_url     TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('statuspage', 'rss', 'custom')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incidents (
    id                  SERIAL PRIMARY KEY,
    provider_id         INTEGER NOT NULL REFERENCES providers(id),
    external_id         TEXT NOT NULL,
    title               TEXT NOT NULL,
    impact              TEXT NOT NULL DEFAULT 'none',
    status              TEXT NOT NULL DEFAULT 'investigating',
    started_at          TIMESTAMPTZ NOT NULL,
    resolved_at         TIMESTAMPTZ,
    duration_minutes    INTEGER,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider_id, external_id)
);

CREATE TABLE IF NOT EXISTS incident_updates (
    id          SERIAL PRIMARY KEY,
    incident_id INTEGER NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    body        TEXT NOT NULL,
    status      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scrape_log (
    id          SERIAL PRIMARY KEY,
    provider_id INTEGER NOT NULL REFERENCES providers(id),
    scraped_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success     BOOLEAN NOT NULL,
    error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_incidents_provider_id ON incidents(provider_id);
CREATE INDEX IF NOT EXISTS idx_incidents_started_at ON incidents(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incident_updates_incident_id ON incident_updates(incident_id);
CREATE INDEX IF NOT EXISTS idx_scrape_log_provider_id ON scrape_log(provider_id);
CREATE INDEX IF NOT EXISTS idx_scrape_log_scraped_at ON scrape_log(scraped_at DESC);

-- Seed providers
INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
    ('Anthropic',   'anthropic',  'https://status.anthropic.com',            'https://status.anthropic.com/api/v2/incidents.json',    'statuspage'),
    ('OpenAI',      'openai',     'https://status.openai.com',               'https://status.openai.com/api/v2/incidents.json',       'statuspage'),
    ('Cloudflare',  'cloudflare', 'https://www.cloudflarestatus.com',        'https://www.cloudflarestatus.com/api/v2/incidents.json','statuspage'),
    ('GitHub',      'github',     'https://githubstatus.com',                'https://githubstatus.com/api/v2/incidents.json',         'statuspage'),
    ('Vercel',      'vercel',     'https://www.vercel-status.com',           'https://www.vercel-status.com/api/v2/incidents.json',    'statuspage'),
    ('Google Cloud','gcp',        'https://status.cloud.google.com',         'https://status.cloud.google.com/incidents.json',         'custom'),
    ('Azure',       'azure',      'https://status.azure.com',                'https://azure.status.microsoft/en-us/status/feed/',      'rss'),
    ('AWS',         'aws',        'https://health.aws.amazon.com',           'https://health.aws.amazon.com/public/currentevents',     'rss'),
    ('Groq',        'groq',       'https://groqstatus.com',                  'https://groqstatus.com/api/v2/incidents.json',           'statuspage'),
    ('Mistral',     'mistral',    'https://mistral-status.com',              'https://mistral-status.com/api/v2/incidents.json',       'statuspage')
ON CONFLICT (slug) DO NOTHING;
