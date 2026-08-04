-- +goose Up
ALTER TABLE group_players ADD COLUMN position VARCHAR(2) DEFAULT 'M'
    CHECK (position IS NULL OR position IN ('GK', 'D', 'M', 'A'));
UPDATE group_players SET position = 'M' WHERE position IS NULL;

-- +goose Down
ALTER TABLE group_players DROP COLUMN position;
