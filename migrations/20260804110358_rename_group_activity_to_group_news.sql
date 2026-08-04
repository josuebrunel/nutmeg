-- +goose Up
ALTER TABLE group_activity RENAME TO group_news;
ALTER TABLE group_news ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Fold each match's full AI-written report into its group_news row (the
-- short "match_logged" blurb it was created with), so match reports become
-- part of the news feed instead of a separate parallel table.
UPDATE group_news
SET content = match_article.content,
    model = match_article.model,
    updated_at = match_article.updated_at
FROM match_article
WHERE group_news.kind = 'match_logged'
  AND group_news.subject_id = match_article.match_id;

DROP TABLE IF EXISTS match_article CASCADE;

ALTER INDEX idx_group_activity_group_created RENAME TO idx_group_news_group_created;

-- +goose Down
ALTER INDEX idx_group_news_group_created RENAME TO idx_group_activity_group_created;

CREATE TABLE match_article (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID NOT NULL UNIQUE REFERENCES matches(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    model VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO match_article (match_id, content, model, updated_at)
SELECT subject_id, content, model, updated_at
FROM group_news
WHERE kind = 'match_logged';

ALTER TABLE group_news DROP COLUMN updated_at;
ALTER TABLE group_news RENAME TO group_activity;
