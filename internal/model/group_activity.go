package model

import "time"

// GroupActivity is one entry in a group's activity feed (a new player
// joining, a match being logged). Unlike PlayerCommentary, this is
// append-only — every event gets its own row, none ever superseded.
type GroupActivity struct {
	ID        string    `db:"id"`
	GroupID   string    `db:"group_id"`
	Kind      string    `db:"kind"`
	SubjectID string    `db:"subject_id"`
	Content   string    `db:"content"`
	Model     *string   `db:"model"`
	CreatedAt time.Time `db:"created_at"`
}
