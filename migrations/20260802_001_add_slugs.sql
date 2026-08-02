-- +goose Up
ALTER TABLE groups ADD COLUMN IF NOT EXISTS slug VARCHAR(160);
ALTER TABLE group_players ADD COLUMN IF NOT EXISTS slug VARCHAR(160);

-- Backfill existing rows with a plain slugified name (e.g. "chris") where
-- that's unique, falling back to name + first 8 chars of id (e.g.
-- "chris-6b7505a9") only for rows whose base slug collides with another
-- row's — mirrors internal/repository.setSlugIfEmpty's clean-first,
-- suffix-on-collision scheme in raw SQL since Goose can't call Go code.
-- One-time duplication for this backfill only, not ongoing debt.
WITH base AS (
    SELECT id, trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')) AS base_slug
    FROM groups
),
dupes AS (
    SELECT base_slug FROM base GROUP BY base_slug HAVING COUNT(*) > 1
)
UPDATE groups g SET slug = CASE
        WHEN b.base_slug = '' OR b.base_slug IN (SELECT base_slug FROM dupes)
            THEN b.base_slug || '-' || substr(g.id::text, 1, 8)
        ELSE b.base_slug
    END
FROM base b
WHERE b.id = g.id AND g.slug IS NULL;

WITH base AS (
    SELECT id, group_id, trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g')) AS base_slug
    FROM group_players
),
dupes AS (
    SELECT group_id, base_slug FROM base GROUP BY group_id, base_slug HAVING COUNT(*) > 1
)
UPDATE group_players gp SET slug = CASE
        WHEN b.base_slug = '' OR (b.group_id, b.base_slug) IN (SELECT group_id, base_slug FROM dupes)
            THEN b.base_slug || '-' || substr(gp.id::text, 1, 8)
        ELSE b.base_slug
    END
FROM base b
WHERE b.id = gp.id AND gp.slug IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_groups_slug ON groups(slug) WHERE slug IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_group_players_group_slug ON group_players(group_id, slug) WHERE slug IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_group_players_group_slug;
DROP INDEX IF EXISTS uq_groups_slug;
ALTER TABLE group_players DROP COLUMN IF EXISTS slug;
ALTER TABLE groups DROP COLUMN IF EXISTS slug;
