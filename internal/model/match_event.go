package model

import "time"

type MatchEvent struct {
	ID         string    `db:"id"`
	MatchID    string    `db:"match_id"`
	TeamID     string    `db:"team_id"`
	ScorerID   string    `db:"scorer_id"`
	AssisterID *string   `db:"assister_id"`
	Minute     *int      `db:"minute"`
	CreatedAt  time.Time `db:"created_at"`
}
