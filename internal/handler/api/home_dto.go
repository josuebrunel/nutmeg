package api

import "nutmeg/internal/repository"

type GlobalStatsResponse struct {
	TotalMatches int `json:"total_matches"`
	TotalGoals   int `json:"total_goals"`
	TotalAssists int `json:"total_assists"`
	TotalDraws   int `json:"total_draws"`
	TotalPlayers int `json:"total_players"`
}

func toGlobalStatsResponse(s *repository.GlobalStats) GlobalStatsResponse {
	return GlobalStatsResponse{
		TotalMatches: s.TotalMatches,
		TotalGoals:   s.TotalGoals,
		TotalAssists: s.TotalAssists,
		TotalDraws:   s.TotalDraws,
		TotalPlayers: s.TotalPlayers,
	}
}

type DashboardResponse struct {
	Groups []GroupResponse     `json:"groups"`
	Stats  GlobalStatsResponse `json:"stats"`
}
