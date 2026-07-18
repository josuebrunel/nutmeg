package service

import (
	"context"
	"errors"
	"strings"

	"nutmeg/internal/repository"
)

type MatchRepository interface {
	CreateMatch(ctx context.Context, groupID, teamAName, teamBName string, scoreA, scoreB int, createdBy string, teamAPlayers, teamBPlayers []string, goals map[string]int) error
	ListMatchesByGroup(ctx context.Context, groupID string) ([]repository.MatchWithTeams, error)
	DeleteMatch(ctx context.Context, matchID string) error
	GetGroupLeaderboard(ctx context.Context, groupID string) ([]repository.LeaderboardEntry, error)
	GetPlayerStats(ctx context.Context, userID string) (*repository.PlayerStats, error)
}

type MatchService struct {
	repo      MatchRepository
	groupRepo GroupRepository
}

func NewMatchService(repo MatchRepository, groupRepo GroupRepository) *MatchService {
	return &MatchService{repo: repo, groupRepo: groupRepo}
}

type CreateMatchInput struct {
	GroupID      string
	TeamAName    string
	TeamBName    string
	ScoreA       int
	ScoreB       int
	CreatedBy    string
	TeamAPlayers []string
	TeamBPlayers []string
	GoalsInput   string
}

func (s *MatchService) Create(ctx context.Context, input CreateMatchInput) error {
	if input.TeamAName == "" || input.TeamBName == "" {
		return errors.New("team names are required")
	}
	if len(input.TeamAPlayers) == 0 || len(input.TeamBPlayers) == 0 {
		return errors.New("each team must have at least one player")
	}
	if input.ScoreA < 0 || input.ScoreB < 0 {
		return errors.New("scores cannot be negative")
	}

	goals := make(map[string]int)
	if input.GoalsInput != "" {
		parts := strings.Split(input.GoalsInput, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			fields := strings.Split(part, ":")
			if len(fields) < 3 {
				continue
			}
			playerID := strings.TrimSpace(fields[0])
			count := 1
			if len(fields) >= 3 {
				// Format: playerId:team:count
				// We just use playerId and count, ignore team letter
				if c, err := parseInt(fields[2]); err == nil {
					count = c
				}
			}
			goals[playerID] = count
		}
	}

	return s.repo.CreateMatch(ctx, input.GroupID, input.TeamAName, input.TeamBName, input.ScoreA, input.ScoreB, input.CreatedBy, input.TeamAPlayers, input.TeamBPlayers, goals)
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func (s *MatchService) ListByGroup(ctx context.Context, groupID string) ([]repository.MatchWithTeams, error) {
	matches, err := s.repo.ListMatchesByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if matches == nil {
		return []repository.MatchWithTeams{}, nil
	}
	return matches, nil
}

func (s *MatchService) Delete(ctx context.Context, matchID, userID string) error {
	return s.repo.DeleteMatch(ctx, matchID)
}

func (s *MatchService) GetLeaderboard(ctx context.Context, groupID string) ([]repository.LeaderboardEntry, error) {
	entries, err := s.repo.GetGroupLeaderboard(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		return []repository.LeaderboardEntry{}, nil
	}
	return entries, nil
}

func (s *MatchService) GetPlayerStats(ctx context.Context, userID string) (*repository.PlayerStats, error) {
	stats, err := s.repo.GetPlayerStats(ctx, userID)
	if err != nil {
		return &repository.PlayerStats{}, nil
	}
	return stats, nil
}
