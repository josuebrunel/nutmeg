package model

import "time"

type Team struct {
	ID        string    `db:"id"`
	GroupID   string    `db:"group_id"`
	Name      string    `db:"name"`
	Color     *string   `db:"color"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
