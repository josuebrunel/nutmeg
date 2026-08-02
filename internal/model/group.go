package model

import "time"

type Group struct {
	ID          string    `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	CreatedBy   string    `db:"created_by"`
	Slug        *string   `db:"slug"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// PublicPath returns the identifier this group's own public-facing links
// (leaderboard, player profiles) should be built from — its slug once it
// has one, falling back to its id for groups created before the slug
// column existed.
func (g *Group) PublicPath() string {
	if g.Slug != nil && *g.Slug != "" {
		return *g.Slug
	}
	return g.ID
}
