package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

// streakLookback caps how many past matches the streak calculation walks,
// per the spec — keeps the query cheap regardless of how long a group has
// been active.
const streakLookback = 10

// maxRoastLength caps generated commentary — long enough for a few
// sentences of trash talk, short enough that a rambling local model gets
// trimmed rather than displayed in full.
const maxRoastLength = 700

type CommentaryRepository interface {
	GetMemberByID(ctx context.Context, memberID string) (*model.GroupPlayer, error)
	GetPlayerStats(ctx context.Context, memberID string) (*repository.PlayerStats, error)
	GetPlayerMatchHistory(ctx context.Context, memberID string, limit int) ([]repository.PlayerMatchResult, error)
	GetGroupLeaderboard(ctx context.Context, groupID string, sortBy string) ([]repository.LeaderboardEntry, error)
	GetActivePlayerCommentary(ctx context.Context, groupPlayerID string) (*repository.PlayerCommentary, error)
	ReplacePlayerCommentary(ctx context.Context, groupPlayerID string, matchID *string, content, model string) error
}

// LLMGenerator is satisfied by *llm.Client — declared here, not imported,
// so the service can be unit-tested against a fake without a real Ollama
// instance running.
type LLMGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Model() string
}

type CommentaryService struct {
	repo CommentaryRepository
	llm  LLMGenerator
}

func NewCommentaryService(repo CommentaryRepository, llm LLMGenerator) *CommentaryService {
	return &CommentaryService{repo: repo, llm: llm}
}

// Generate builds a fresh roast for a player from their real stats and
// match history, validates it, and — only if it passes — atomically
// supersedes any existing active commentary with the new entry. On any
// failure (LLM unreachable, empty/invalid output), existing data is left
// untouched, so a bad generation never overwrites a working one.
func (s *CommentaryService) Generate(ctx context.Context, groupPlayerID string, matchID *string) error {
	member, err := s.repo.GetMemberByID(ctx, groupPlayerID)
	if err != nil {
		return fmt.Errorf("commentary: fetch member: %w", err)
	}

	stats, err := s.repo.GetPlayerStats(ctx, groupPlayerID)
	if err != nil {
		return fmt.Errorf("commentary: fetch stats: %w", err)
	}

	history, err := s.repo.GetPlayerMatchHistory(ctx, groupPlayerID, streakLookback)
	if err != nil {
		return fmt.Errorf("commentary: fetch history: %w", err)
	}

	leaderboard, err := s.repo.GetGroupLeaderboard(ctx, member.GroupID, "")
	if err != nil {
		return fmt.Errorf("commentary: fetch leaderboard: %w", err)
	}
	isTopScorer := repository.TopScorerID(leaderboard) == groupPlayerID
	isTopPasser := repository.TopPasserID(leaderboard) == groupPlayerID
	isTopDefender := repository.TopDefenderID(leaderboard) == groupPlayerID

	prompt := s.BuildPrompt(member.Name, member.Position, *stats, history, isTopScorer, isTopPasser, isTopDefender)

	started := time.Now()
	raw, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		slog.Debug("commentary: llm generate failed", "group_player_id", groupPlayerID, "model", s.llm.Model(), "duration", time.Since(started))
		return fmt.Errorf("commentary: generate: %w", err)
	}
	slog.Debug("commentary: llm generate completed", "group_player_id", groupPlayerID, "model", s.llm.Model(), "duration", time.Since(started), "chars", len(raw))

	content, err := validateRoast(raw)
	if err != nil {
		return fmt.Errorf("commentary: validate: %w", err)
	}

	if err := s.repo.ReplacePlayerCommentary(ctx, groupPlayerID, matchID, content, s.llm.Model()); err != nil {
		return fmt.Errorf("commentary: store: %w", err)
	}

	return nil
}

// LastGeneratedAt returns when the player's current active commentary was
// created, or nil if none exists yet — used to enforce the regeneration
// cooldown.
func (s *CommentaryService) LastGeneratedAt(ctx context.Context, groupPlayerID string) (*time.Time, error) {
	pc, err := s.repo.GetActivePlayerCommentary(ctx, groupPlayerID)
	if err != nil {
		return nil, err
	}
	if pc == nil {
		return nil, nil
	}
	return &pc.CreatedAt, nil
}

// CommentaryRegenerationCooldown is the minimum time between regenerations
// of a single player's commentary — real inference cost on a
// memory-constrained self-hosted box, so this is deliberately conservative.
const CommentaryRegenerationCooldown = 10 * time.Minute

// CanRegenerate reports whether enough time has passed since the player's
// last commentary generation to allow another one; if not, it also
// reports how much longer the caller needs to wait.
func (s *CommentaryService) CanRegenerate(ctx context.Context, groupPlayerID string) (bool, time.Duration, error) {
	last, err := s.LastGeneratedAt(ctx, groupPlayerID)
	if err != nil {
		return false, 0, err
	}
	if last == nil {
		return true, 0, nil
	}
	elapsed := time.Since(*last)
	if elapsed >= CommentaryRegenerationCooldown {
		return true, 0, nil
	}
	return false, CommentaryRegenerationCooldown - elapsed, nil
}

// BuildPrompt constructs the roast prompt strictly from derived stats,
// streak data, and group-standing facts — nothing about a player is ever
// invented. position is nil when the player hasn't set one; the
// clean-sheet line only appears for goalkeepers/defenders, since that's the
// stat that's actually their job.
func (s *CommentaryService) BuildPrompt(playerName string, position *string, stats repository.PlayerStats, history []repository.PlayerMatchResult, isTopScorer, isTopPasser, isTopDefender bool) string {
	scoreless := scorelessStreak(history)
	losing := losingStreak(history)

	streakLine := "No notable losing or scoreless streak right now."
	switch {
	case scoreless >= 3:
		streakLine = fmt.Sprintf("They haven't scored in their last %d matches.", scoreless)
	case losing >= 2:
		streakLine = fmt.Sprintf("They're on a %d-match losing streak.", losing)
	}

	var titles []string
	if isTopScorer {
		titles = append(titles, "They are currently the group's top scorer.")
	}
	if isTopPasser {
		titles = append(titles, "They are currently the group's top assist provider.")
	}
	if isTopDefender {
		titles = append(titles, "They are currently the group's top defender by clean sheets.")
	}
	titleLine := strings.Join(titles, " ")

	defenseLine := ""
	if position != nil && (*position == model.PositionGK || *position == model.PositionD) {
		if stats.CleanSheets > 0 {
			defenseLine = fmt.Sprintf("They play %s and have kept %d clean sheet(s) in %d matches.", *position, stats.CleanSheets, stats.MatchesPlayed)
		} else {
			defenseLine = fmt.Sprintf("They play %s and haven't kept a single clean sheet yet in %d matches.", *position, stats.MatchesPlayed)
		}
	}

	return fmt.Sprintf(`You are a savage but good-natured trash-talking commentator for a casual pickup soccer group chat.

Write a short, funny roast (3-5 sentences, no more) about this player, based ONLY on the real stats below. Do not invent any stat, event, or piece of history that isn't listed here. Never mention minutes played or time on the pitch - that data doesn't exist for this app. Keep it good-natured banter between friends: never cruel, never about anything other than their soccer performance.

Player: %s
Matches played: %d
Record: %d wins, %d draws, %d losses
Goals: %d
Assists: %d
%s
%s
%s

Write only the roast itself, nothing else - no preamble, no quotation marks.`,
		playerName, stats.MatchesPlayed, stats.Wins, stats.Draws, stats.Losses, stats.Goals, stats.Assists, defenseLine, streakLine, titleLine)
}

// scorelessStreak counts consecutive most-recent matches (history is
// newest-first) with zero goals for this player, stopping at the first
// match they scored in.
func scorelessStreak(history []repository.PlayerMatchResult) int {
	n := 0
	for _, m := range history {
		if m.GoalsScored > 0 {
			break
		}
		n++
	}
	return n
}

// losingStreak counts consecutive most-recent losses, stopping at the
// first non-loss.
func losingStreak(history []repository.PlayerMatchResult) int {
	n := 0
	for _, m := range history {
		if m.Result() != "loss" {
			break
		}
		n++
	}
	return n
}

// validateRoast delegates to the shared generation validator (see
// llmsafety.go) with the roast-specific length cap.
func validateRoast(text string) (string, error) {
	return validateGeneratedText(text, maxRoastLength)
}
