-- +goose Up
CREATE TABLE IF NOT EXISTS player_commentary (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_player_id UUID NOT NULL REFERENCES group_players(id) ON DELETE CASCADE,
    match_id UUID REFERENCES matches(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    model VARCHAR(100) NOT NULL,
    superseded BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_player_commentary_group_player_id ON player_commentary(group_player_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_player_commentary_active ON player_commentary(group_player_id) WHERE NOT superseded;

-- +goose Down
DROP TABLE IF EXISTS player_commentary CASCADE;
