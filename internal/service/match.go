package service

import (
	"context"
	"errors"
	"strings"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

type MatchRepository interface {
	CreateMatch(ctx context.Context, groupID, teamAName, teamBName string, scoreA, scoreB int, createdBy string, teamAPlayers, teamBPlayers []string, goals map[string]int) error
	ListMatchesByGroup(ctx context.Context, groupID string) ([]repository.MatchWithTeams, error)
	DeleteMatch(ctx context.Context, matchID string) error
	GetGroupLeaderboard(ctx context.Context, groupID string) ([]repository.LeaderboardEntry, error)
	GetPlayerStats(ctx context.Context, userID string) (*repository.PlayerStats, error)
	GetMatchDetail(ctx context.Context, matchID string) (*repository.MatchDetail, error)
	GetMatchPlayers(ctx context.Context, matchID string) ([]repository.MatchPlayerRow, error)
	GetMatchGoals(ctx context.Context, matchID string) (map[string]int, error)
	UpdateMatch(ctx context.Context, matchID, teamAName, teamBName string, scoreA, scoreB int, teamAPlayers, teamBPlayers []string, goals map[string]int) error
	GetGlobalStats(ctx context.Context, userID string) (*repository.GlobalStats, error)
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

	goals := parseGoals(input.GoalsInput)
	return s.repo.CreateMatch(ctx, input.GroupID, input.TeamAName, input.TeamBName, input.ScoreA, input.ScoreB, input.CreatedBy, input.TeamAPlayers, input.TeamBPlayers, goals)
}

func parseGoals(input string) map[string]int {
	goals := make(map[string]int)
	if input == "" {
		return goals
	}
	parts := strings.Split(input, ",")
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
		if c, err := parseInt(fields[2]); err == nil {
			count = c
		}
		goals[playerID] = count
	}
	return goals
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

func (s *MatchService) AuthorizeGroupAccess(ctx context.Context, groupID, userID string) error {
	g, err := s.groupRepo.GetGroup(ctx, groupID)
	if err != nil {
		return model.ErrNotFound
	}
	if g.CreatedBy != userID {
		return model.ErrNotAuthorized
	}
	return nil
}

func (s *MatchService) authorizeMatchAccess(ctx context.Context, matchID, userID string) error {
	detail, err := s.repo.GetMatchDetail(ctx, matchID)
	if err != nil {
		return model.ErrNotFound
	}
	g, err := s.groupRepo.GetGroup(ctx, detail.GroupID)
	if err != nil {
		return err
	}
	if g.CreatedBy != userID {
		return model.ErrNotAuthorized
	}
	return nil
}

func (s *MatchService) Delete(ctx context.Context, matchID, userID string) error {
	if err := s.authorizeMatchAccess(ctx, matchID, userID); err != nil {
		return err
	}
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

type UpdateMatchInput struct {
	MatchID      string
	UserID       string
	TeamAName    string
	TeamBName    string
	ScoreA       int
	ScoreB       int
	TeamAPlayers []string
	TeamBPlayers []string
	GoalsInput   string
}

type EditableMatch struct {
	MatchID      string
	GroupID      string
	TeamAName    string
	TeamBName    string
	ScoreA       int
	ScoreB       int
	TeamAPlayers []string
	TeamBPlayers []string
	Goals        map[string]int
}

func (s *MatchService) GetEditable(ctx context.Context, matchID, userID string) (*EditableMatch, error) {
	if err := s.authorizeMatchAccess(ctx, matchID, userID); err != nil {
		return nil, err
	}

	detail, err := s.repo.GetMatchDetail(ctx, matchID)
	if err != nil {
		return nil, err
	}
	players, err := s.repo.GetMatchPlayers(ctx, matchID)
	if err != nil {
		return nil, err
	}
	goals, err := s.repo.GetMatchGoals(ctx, matchID)
	if err != nil {
		return nil, err
	}

	var teamAPlayers, teamBPlayers []string
	for _, p := range players {
		if p.TeamID == detail.HomeTeamID {
			teamAPlayers = append(teamAPlayers, p.PlayerID)
		} else {
			teamBPlayers = append(teamBPlayers, p.PlayerID)
		}
	}
	if teamAPlayers == nil {
		teamAPlayers = []string{}
	}
	if teamBPlayers == nil {
		teamBPlayers = []string{}
	}
	if goals == nil {
		goals = make(map[string]int)
	}

	return &EditableMatch{
		MatchID:      matchID,
		GroupID:      detail.GroupID,
		TeamAName:    detail.TeamAName,
		TeamBName:    detail.TeamBName,
		ScoreA:       detail.ScoreA,
		ScoreB:       detail.ScoreB,
		TeamAPlayers: teamAPlayers,
		TeamBPlayers: teamBPlayers,
		Goals:        goals,
	}, nil
}

func (s *MatchService) Update(ctx context.Context, input UpdateMatchInput) error {
	if err := s.authorizeMatchAccess(ctx, input.MatchID, input.UserID); err != nil {
		return err
	}
	goals := parseGoals(input.GoalsInput)
	return s.repo.UpdateMatch(ctx, input.MatchID, input.TeamAName, input.TeamBName, input.ScoreA, input.ScoreB, input.TeamAPlayers, input.TeamBPlayers, goals)
}

func (s *MatchService) GlobalStats(ctx context.Context, userID string) (*repository.GlobalStats, error) {
	stats, err := s.repo.GetGlobalStats(ctx, userID)
	if err != nil {
		return &repository.GlobalStats{}, nil
	}
	return stats, nil
}
