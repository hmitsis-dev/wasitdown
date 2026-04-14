-- Migration 003: Fix broken provider URLs and remove providers with no working public API

-- Fix GitHub (missing www.)
UPDATE providers SET
    api_url = 'https://www.githubstatus.com/api/v2/incidents.json',
    status_page_url = 'https://www.githubstatus.com'
WHERE slug = 'github';

-- Fix AWS (now returns JSON, not RSS)
UPDATE providers SET type = 'custom' WHERE slug = 'aws';

-- Remove providers with no accessible public incident API
DELETE FROM incidents WHERE provider_id IN (
    SELECT id FROM providers WHERE slug IN (
        'mistral', 'slack', 'teams', 'meta', 'twitter', 'tiktok',
        'snapchat', 'youtube', 'spotify', 'netflix', 'paypal', 'stripe'
    )
);
DELETE FROM scrape_log WHERE provider_id IN (
    SELECT id FROM providers WHERE slug IN (
        'mistral', 'slack', 'teams', 'meta', 'twitter', 'tiktok',
        'snapchat', 'youtube', 'spotify', 'netflix', 'paypal', 'stripe'
    )
);
DELETE FROM providers WHERE slug IN (
    'mistral', 'slack', 'teams', 'meta', 'twitter', 'tiktok',
    'snapchat', 'youtube', 'spotify', 'netflix', 'paypal', 'stripe'
);
