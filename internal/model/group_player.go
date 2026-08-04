package model

import "time"

// Roster roles, stored on GroupPlayer.Role / group_players.role.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type GroupPlayer struct {
	ID       string    `db:"id"`
	GroupID  string    `db:"group_id"`
	Name     string    `db:"name"`
	Phone    *string   `db:"phone"`
	Email    *string   `db:"email"`
	Role     string    `db:"role"`
	Slug     *string   `db:"slug"`
	JoinedAt time.Time `db:"joined_at"`
}

// PublicPath returns the identifier this player's own public profile link
// should be built from — see Group.PublicPath.
func (p *GroupPlayer) PublicPath() string {
	if p.Slug != nil && *p.Slug != "" {
		return *p.Slug
	}
	return p.ID
}
