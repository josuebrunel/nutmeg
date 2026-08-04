package model

import "time"

// GroupNews is one entry in a group's public news feed (a new player
// joining, a match report). "player_added" rows are append-only — every
// event gets its own row, none ever superseded. A "match_logged" row is
// updated in place instead, as its content is upgraded from a fallback line
// to the full AI-generated match report, and again on manual regeneration —
// the same discipline the old, now-merged MatchArticle followed.
type GroupNews struct {
	ID        string    `db:"id"`
	GroupID   string    `db:"group_id"`
	Kind      string    `db:"kind"`
	SubjectID string    `db:"subject_id"`
	Content   string    `db:"content"`
	Model     *string   `db:"model"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
