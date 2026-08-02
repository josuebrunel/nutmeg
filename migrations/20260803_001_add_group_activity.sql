-- +goose Up
CREATE TABLE IF NOT EXISTS group_activity (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    kind VARCHAR(30) NOT NULL CHECK (kind IN ('player_added', 'match_logged')),
    subject_id UUID NOT NULL,
    content TEXT NOT NULL,
    model VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_group_activity_group_created ON group_activity(group_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS group_activity CASCADE;
