package model

import "time"

type Player struct {
	ID          string    `db:"id"`
	Email       string    `db:"email"`
	DisplayName string    `db:"display_name"`
	UserID      *string   `db:"user_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
