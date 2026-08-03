package model

import "time"

// MatchArticle is the AI-generated funny news article for a single match —
// one row per match, updated in place on regeneration (unlike
// PlayerCommentary, which supersedes instead of updating).
type MatchArticle struct {
	ID        string    `db:"id"`
	MatchID   string    `db:"match_id"`
	Content   string    `db:"content"`
	Model     *string   `db:"model"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
