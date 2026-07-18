package model

import "time"

type Match struct {
	ID         string    `db:"id"`
	GroupID    string    `db:"group_id"`
	HomeTeamID string    `db:"home_team_id"`
	AwayTeamID string    `db:"away_team_id"`
	HomeScore  int       `db:"home_score"`
	AwayScore  int       `db:"away_score"`
	PlayedAt   time.Time `db:"played_at"`
	Notes      *string   `db:"notes"`
	CreatedBy  string    `db:"created_by"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}
