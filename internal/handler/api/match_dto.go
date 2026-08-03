package api

import (
	"fmt"
	"strings"
	"time"

	"nutmeg/internal/repository"
	"nutmeg/internal/service"
)

type MatchResponse struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	TeamAName string    `json:"team_a_name"`
	TeamBName string    `json:"team_b_name"`
	ScoreA    int       `json:"score_a"`
	ScoreB    int       `json:"score_b"`
	PlayedAt  time.Time `json:"played_at"`
}

func toMatchResponse(m repository.MatchWithTeams) MatchResponse {
	return MatchResponse{
		ID:        m.ID,
		GroupID:   m.GroupID,
		TeamAName: m.TeamAName,
		TeamBName: m.TeamBName,
		ScoreA:    m.ScoreA,
		ScoreB:    m.ScoreB,
		PlayedAt:  m.PlayedAt,
	}
}

func toMatchResponses(matches []repository.MatchWithTeams) []MatchResponse {
	out := make([]MatchResponse, len(matches))
	for i, m := range matches {
		out[i] = toMatchResponse(m)
	}
	return out
}

// MatchDetailResponse is the full per-player breakdown of a single match —
// returned by Create/Get/Update, whereas MatchResponse (the list shape) only
// carries team-level scores.
type MatchDetailResponse struct {
	MatchID      string         `json:"match_id"`
	GroupID      string         `json:"group_id"`
	TeamAName    string         `json:"team_a_name"`
	TeamBName    string         `json:"team_b_name"`
	ScoreA       int            `json:"score_a"`
	ScoreB       int            `json:"score_b"`
	TeamAPlayers []string       `json:"team_a_players"`
	TeamBPlayers []string       `json:"team_b_players"`
	Goals        map[string]int `json:"goals"`
	Assists      map[string]int `json:"assists"`
	PlayedAt     time.Time      `json:"played_at"`
}

func toMatchDetailResponse(m *service.EditableMatch) MatchDetailResponse {
	return MatchDetailResponse{
		MatchID:      m.MatchID,
		GroupID:      m.GroupID,
		TeamAName:    m.TeamAName,
		TeamBName:    m.TeamBName,
		ScoreA:       m.ScoreA,
		ScoreB:       m.ScoreB,
		TeamAPlayers: m.TeamAPlayers,
		TeamBPlayers: m.TeamBPlayers,
		Goals:        m.Goals,
		Assists:      m.Assists,
		PlayedAt:     m.PlayedAt,
	}
}

// MatchWriteRequest is the request body for both logging a new match and
// editing an existing one. Goals/Assists key by player id, unlike the HTML
// form's bespoke "playerID:team:count" encoding (see encodeTally below).
type MatchWriteRequest struct {
	TeamAName    string         `json:"team_a_name"`
	TeamBName    string         `json:"team_b_name"`
	ScoreA       int            `json:"score_a"`
	ScoreB       int            `json:"score_b"`
	TeamAPlayers []string       `json:"team_a_players"`
	TeamBPlayers []string       `json:"team_b_players"`
	Goals        map[string]int `json:"goals,omitempty"`
	Assists      map[string]int `json:"assists,omitempty"`
	PlayedAt     *time.Time     `json:"played_at,omitempty"`
}

// encodeTally converts a clean player-id -> count map into the
// "playerID:team:count,..." string service.MatchService.Create/Update
// expect (an HTML-form artifact — see internal/service/match.go's
// parseTally) — keeps the service layer untouched while giving API callers
// a normal JSON map instead.
func encodeTally(counts map[string]int, teamAPlayers, teamBPlayers []string) string {
	teamOf := make(map[string]string, len(teamAPlayers)+len(teamBPlayers))
	for _, pid := range teamAPlayers {
		teamOf[pid] = "a"
	}
	for _, pid := range teamBPlayers {
		teamOf[pid] = "b"
	}

	parts := make([]string, 0, len(counts))
	for pid, count := range counts {
		if count <= 0 {
			continue
		}
		team, ok := teamOf[pid]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s:%d", pid, team, count))
	}
	return strings.Join(parts, ",")
}
