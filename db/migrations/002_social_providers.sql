-- Migration 002: Social & consumer platform providers
--
-- Status page type notes:
--   statuspage  = Atlassian Statuspage (/api/v2/incidents.json)
--   rss         = RSS/Atom feed
--   custom      = proprietary format, handled by dedicated scraper

INSERT INTO providers (name, slug, status_page_url, api_url, type) VALUES
    -- Communication & collaboration
    ('Discord',       'discord',   'https://discordstatus.com',              'https://discordstatus.com/api/v2/incidents.json',              'statuspage'),
    ('Slack',         'slack',     'https://slack-status.com',               'https://slack-status.com/api/v2/incidents.json',               'statuspage'),
    ('Zoom',          'zoom',      'https://status.zoom.us',                 'https://status.zoom.us/api/v2/incidents.json',                 'statuspage'),
    -- Microsoft Teams — public M365 service health RSS feed
    ('Teams',         'teams',     'https://status.office365.com',           'https://status.office365.com/api/feed',                        'rss'),

    -- Social platforms
    -- Meta covers Instagram, Facebook, WhatsApp, and Threads — all on one status page
    ('Meta (Instagram / Facebook / WhatsApp / Threads)',
                      'meta',      'https://metastatus.com',                 'https://metastatus.com/api/v2/incidents.json',                 'statuspage'),
    ('X (Twitter)',   'twitter',   'https://api.twitterstat.us',             'https://api.twitterstat.us/api/v2/incidents.json',             'statuspage'),
    ('TikTok',        'tiktok',    'https://status.tiktok.com',              'https://status.tiktok.com/api/v2/incidents.json',              'statuspage'),
    ('Snapchat',      'snapchat',  'https://status.snapchat.com',            'https://status.snapchat.com/api/v2/incidents.json',            'statuspage'),
    -- YouTube: no dedicated public API; scraper uses Google's consumer status feed
    ('YouTube',       'youtube',   'https://status.google.com',              'https://status.google.com/products/youtube',                   'custom'),

    -- Streaming
    ('Spotify',       'spotify',   'https://status.spotify.com',             'https://status.spotify.com/api/v2/incidents.json',             'statuspage'),
    -- Netflix: no official public incident API; scraper polls their status help page
    ('Netflix',       'netflix',   'https://help.netflix.com/en/node/100649','https://help.netflix.com/en/node/100649',                      'custom'),

    -- Payments
    ('PayPal',        'paypal',    'https://www.paypal-status.com',          'https://www.paypal-status.com/api/v2/incidents.json',          'statuspage'),
    ('Stripe',        'stripe',    'https://status.stripe.com',              'https://status.stripe.com/api/v2/incidents.json',              'statuspage')
ON CONFLICT (slug) DO NOTHING;
