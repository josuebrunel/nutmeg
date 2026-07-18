package model

import "time"

type GroupPlayer struct {
	ID       string    `db:"id"`
	GroupID  string    `db:"group_id"`
	UserID   string    `db:"user_id"`
	Role     string    `db:"role"`
	JoinedAt time.Time `db:"joined_at"`
}

type GroupPlayerWithUser struct {
	GroupPlayer
	Email     string `db:"email"`
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
}
