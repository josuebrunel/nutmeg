package model

import "time"

type GroupPlayer struct {
	ID       string    `db:"id"`
	GroupID  string    `db:"group_id"`
	Name     string    `db:"name"`
	Phone    *string   `db:"phone"`
	Email    *string   `db:"email"`
	Role     string    `db:"role"`
	JoinedAt time.Time `db:"joined_at"`
}
