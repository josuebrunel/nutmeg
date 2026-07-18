package model

import "time"

type GroupPlayer struct {
	ID       string    `db:"id"`
	GroupID  string    `db:"group_id"`
	PlayerID string    `db:"player_id"`
	Role     string    `db:"role"`
	JoinedAt time.Time `db:"joined_at"`
}

type GroupPlayerWithPlayer struct {
	GroupPlayer
	Email       string `db:"email"`
	DisplayName string `db:"display_name"`
}
