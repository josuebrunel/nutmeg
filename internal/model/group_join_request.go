package model

import "time"

type JoinRequest struct {
	ID        string    `db:"id"`
	GroupID   string    `db:"group_id"`
	UserID    string    `db:"user_id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
