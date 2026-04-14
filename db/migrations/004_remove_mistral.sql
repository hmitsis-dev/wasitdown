-- Migration 004: Remove Mistral (status.mistral.ai is a custom Nuxt app with no JSON API)
DELETE FROM incidents WHERE provider_id = (SELECT id FROM providers WHERE slug = 'mistral');
DELETE FROM scrape_log WHERE provider_id = (SELECT id FROM providers WHERE slug = 'mistral');
DELETE FROM providers WHERE slug = 'mistral';
