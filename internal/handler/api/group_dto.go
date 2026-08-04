package api

import (
	"time"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type GroupResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toGroupResponse(g *model.Group) GroupResponse {
	return GroupResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		CreatedBy:   g.CreatedBy,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}

func toGroupResponses(groups []*model.Group) []GroupResponse {
	out := make([]GroupResponse, len(groups))
	for i, g := range groups {
		out[i] = toGroupResponse(g)
	}
	return out
}

type GroupCreateRequest struct {
	Name string `json:"name"`
}

type GroupUpdateRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type MemberResponse struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Phone    *string   `json:"phone,omitempty"`
	Email    *string   `json:"email,omitempty"`
	Role     string    `json:"role"`
	Position *string   `json:"position,omitempty"`
	JoinedAt time.Time `json:"joined_at"`
}

func toMemberResponse(m repository.MemberInfo) MemberResponse {
	return MemberResponse{ID: m.ID, Name: m.Name, Phone: m.Phone, Email: m.Email, Role: m.Role, Position: m.Position, JoinedAt: m.JoinedAt}
}

func toMemberResponses(members []repository.MemberInfo) []MemberResponse {
	out := make([]MemberResponse, len(members))
	for i, m := range members {
		out[i] = toMemberResponse(m)
	}
	return out
}

// MemberAddRequest adds one member (Names has one entry) or several at once
// (Names has more than one) — mirroring the HTML roster form's
// comma-separated "name" field, which only accepts phone/email for a
// single addition since they can't be shared across people.
type MemberAddRequest struct {
	Names    []string `json:"names"`
	Phone    *string  `json:"phone,omitempty"`
	Email    *string  `json:"email,omitempty"`
	Position *string  `json:"position,omitempty"`
}

type MemberUpdateRequest struct {
	Name     string  `json:"name"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Position *string `json:"position,omitempty"`
}

type MemberImportRow struct {
	Name     string `json:"name"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	Position string `json:"position,omitempty"`
}

type MemberImportRequest struct {
	Rows []MemberImportRow `json:"rows"`
}

type MemberImportResponse struct {
	Imported int `json:"imported"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

type JoinRequestResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func toJoinRequestResponse(r repository.JoinRequestInfo) JoinRequestResponse {
	return JoinRequestResponse{ID: r.ID, Name: r.Name, Email: r.Email, CreatedAt: r.CreatedAt}
}

func toJoinRequestResponses(reqs []repository.JoinRequestInfo) []JoinRequestResponse {
	out := make([]JoinRequestResponse, len(reqs))
	for i, r := range reqs {
		out[i] = toJoinRequestResponse(r)
	}
	return out
}

type LeaderboardEntryResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Matches int    `json:"matches"`
	Wins    int    `json:"wins"`
	Draws   int    `json:"draws"`
	Losses  int    `json:"losses"`
	Goals   int    `json:"goals"`
	Assists int    `json:"assists"`
}

func toLeaderboardResponses(entries []repository.LeaderboardEntry) []LeaderboardEntryResponse {
	out := make([]LeaderboardEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = LeaderboardEntryResponse{
			ID:      e.MemberID,
			Name:    e.Name,
			Matches: e.Matches,
			Wins:    e.Wins,
			Draws:   e.Draws,
			Losses:  e.Losses,
			Goals:   e.Goals,
			Assists: e.Assists,
		}
	}
	return out
}

// PlayerProfileResponse is the public, read-only view of a single player's
// stats — the JSON equivalent of the public player-profile HTML page.
type PlayerProfileResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	GroupID       string  `json:"group_id"`
	MatchesPlayed int     `json:"matches_played"`
	Wins          int     `json:"wins"`
	Draws         int     `json:"draws"`
	Losses        int     `json:"losses"`
	Goals         int     `json:"goals"`
	Assists       int     `json:"assists"`
	Commentary    *string `json:"commentary,omitempty"`
}
