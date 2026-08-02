package service

import (
	"context"
	"fmt"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

// maxActivityLength caps generated group-news blurbs — these are one-line
// announcements, much shorter than a player roast.
const maxActivityLength = 160

type ActivityRepository interface {
	GetMemberByID(ctx context.Context, memberID string) (*model.GroupPlayer, error)
	GetMatchDetail(ctx context.Context, matchID string) (*repository.MatchDetail, error)
	SetGroupActivityContent(ctx context.Context, id, content, model string) error
}

type ActivityService struct {
	repo ActivityRepository
	llm  LLMGenerator
}

func NewActivityService(repo ActivityRepository, llm LLMGenerator) *ActivityService {
	return &ActivityService{repo: repo, llm: llm}
}

// GenerateNews upgrades an already-created group_activity row's
// deterministic fallback content with a short AI-generated blurb, built
// strictly from real data for the given kind/subject. On any failure the
// row is left with its fallback content untouched — same "never overwrite
// a working state with a failure" discipline as CommentaryService.Generate.
func (s *ActivityService) GenerateNews(ctx context.Context, activityID, kind, subjectID string) error {
	prompt, err := s.buildPrompt(ctx, kind, subjectID)
	if err != nil {
		return fmt.Errorf("activity: build prompt: %w", err)
	}

	raw, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("activity: generate: %w", err)
	}

	content, err := validateGeneratedText(raw, maxActivityLength)
	if err != nil {
		return fmt.Errorf("activity: validate: %w", err)
	}

	if err := s.repo.SetGroupActivityContent(ctx, activityID, content, s.llm.Model()); err != nil {
		return fmt.Errorf("activity: store: %w", err)
	}
	return nil
}

func (s *ActivityService) buildPrompt(ctx context.Context, kind, subjectID string) (string, error) {
	switch kind {
	case "player_added":
		player, err := s.repo.GetMemberByID(ctx, subjectID)
		if err != nil {
			return "", fmt.Errorf("fetch player: %w", err)
		}
		return buildPlayerAddedPrompt(player.Name), nil
	case "match_logged":
		match, err := s.repo.GetMatchDetail(ctx, subjectID)
		if err != nil {
			return "", fmt.Errorf("fetch match: %w", err)
		}
		return buildMatchLoggedPrompt(match.TeamAName, match.TeamBName, match.ScoreA, match.ScoreB), nil
	default:
		return "", fmt.Errorf("unknown activity kind %q", kind)
	}
}

// buildPlayerAddedPrompt is pure — no invented facts, only the player's
// real name is known at this point.
func buildPlayerAddedPrompt(playerName string) string {
	return fmt.Sprintf(`You are an upbeat announcer posting group news for a casual pickup soccer group chat.

Write a short, fun one-liner (1 sentence, no more) welcoming a new player who just joined the group. Do not invent any stat, event, or piece of history about them — all that's known is their name.

Player: %s

Write only the announcement itself, nothing else - no preamble, no quotation marks.`, playerName)
}

// buildMatchLoggedPrompt is pure — built strictly from the real final
// score, no invented players or events.
func buildMatchLoggedPrompt(teamAName, teamBName string, scoreA, scoreB int) string {
	return fmt.Sprintf(`You are an upbeat announcer posting group news for a casual pickup soccer group chat.

Write a short, fun one-liner (1 sentence, no more) announcing a match result. Base it ONLY on the real score below — do not invent any player, event, or detail beyond what's listed.

%s %d - %d %s

Write only the announcement itself, nothing else - no preamble, no quotation marks.`, teamAName, scoreA, scoreB, teamBName)
}
