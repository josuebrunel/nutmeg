package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

// maxArticleLength caps a generated match article — a short headline plus
// a few paragraphs, longer-form than the one-line group-activity blurb and
// the 1-3 sentence player roast.
const maxArticleLength = 1800

type MatchArticleRepository interface {
	GetMatchDetail(ctx context.Context, matchID string) (*repository.MatchDetail, error)
	GetMatchGoals(ctx context.Context, matchID string) (map[string]int, error)
	GetMatchAssists(ctx context.Context, matchID string) (map[string]int, error)
	GetMatchPlayers(ctx context.Context, matchID string) ([]repository.MatchPlayerRow, error)
	GetMemberByID(ctx context.Context, memberID string) (*model.GroupPlayer, error)
	GetMatchArticle(ctx context.Context, matchID string) (*model.MatchArticle, error)
	SetMatchArticleContent(ctx context.Context, id, content, model string) error
}

type MatchArticleService struct {
	repo MatchArticleRepository
	llm  LLMGenerator
}

func NewMatchArticleService(repo MatchArticleRepository, llm LLMGenerator) *MatchArticleService {
	return &MatchArticleService{repo: repo, llm: llm}
}

// GenerateArticle builds a fresh news article for a match from its real
// score, goal scorers, and assist providers, validates it, and — only if
// it passes — updates the article row in place. On any failure (LLM
// unreachable, empty/invalid output), the existing content is left
// untouched, same discipline as CommentaryService.Generate.
func (s *MatchArticleService) GenerateArticle(ctx context.Context, articleID, matchID string) error {
	match, err := s.repo.GetMatchDetail(ctx, matchID)
	if err != nil {
		return fmt.Errorf("match article: fetch match: %w", err)
	}

	goals, err := s.repo.GetMatchGoals(ctx, matchID)
	if err != nil {
		return fmt.Errorf("match article: fetch goals: %w", err)
	}
	assists, err := s.repo.GetMatchAssists(ctx, matchID)
	if err != nil {
		return fmt.Errorf("match article: fetch assists: %w", err)
	}

	names, err := s.resolvePlayerNames(ctx, goals, assists)
	if err != nil {
		return fmt.Errorf("match article: resolve player names: %w", err)
	}

	prompt := buildMatchArticlePrompt(match.TeamAName, match.TeamBName, match.ScoreA, match.ScoreB,
		formatStatLines(goals, names, "goal"), formatStatLines(assists, names, "assist"))

	raw, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("match article: generate: %w", err)
	}

	content, err := validateGeneratedText(raw, maxArticleLength)
	if err != nil {
		return fmt.Errorf("match article: validate: %w", err)
	}

	if err := s.repo.SetMatchArticleContent(ctx, articleID, content, s.llm.Model()); err != nil {
		return fmt.Errorf("match article: store: %w", err)
	}
	return nil
}

// resolvePlayerNames looks up the display name of every player appearing
// in goals or assists — team rosters are small (pickup soccer), so one
// GetMemberByID call per unique id is simpler than adding a batch query.
func (s *MatchArticleService) resolvePlayerNames(ctx context.Context, goals, assists map[string]int) (map[string]string, error) {
	names := make(map[string]string)
	for playerID := range goals {
		if _, ok := names[playerID]; ok {
			continue
		}
		member, err := s.repo.GetMemberByID(ctx, playerID)
		if err != nil {
			return nil, err
		}
		names[playerID] = member.Name
	}
	for playerID := range assists {
		if _, ok := names[playerID]; ok {
			continue
		}
		member, err := s.repo.GetMemberByID(ctx, playerID)
		if err != nil {
			return nil, err
		}
		names[playerID] = member.Name
	}
	return names, nil
}

// formatStatLines turns a playerID -> count map into sorted "Name (n
// noun[s])" lines, e.g. "Chris (2 goals)" — sorted by count descending
// then name, so the prompt's most notable performers come first.
func formatStatLines(counts map[string]int, names map[string]string, noun string) []string {
	type row struct {
		name  string
		count int
	}
	rows := make([]row, 0, len(counts))
	for playerID, count := range counts {
		rows = append(rows, row{name: names[playerID], count: count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].name < rows[j].name
	})
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		plural := noun + "s"
		if r.count == 1 {
			plural = noun
		}
		lines = append(lines, fmt.Sprintf("%s (%d %s)", r.name, r.count, plural))
	}
	return lines
}

// buildMatchArticlePrompt is pure — built strictly from the real score,
// scorers, and assisters, no invented players or events.
func buildMatchArticlePrompt(teamAName, teamBName string, scoreA, scoreB int, scorers, assisters []string) string {
	scorerLine := "No goals were scored by anyone listed."
	if len(scorers) > 0 {
		scorerLine = "Goal scorers: " + strings.Join(scorers, ", ")
	}
	assisterLine := "No assists were recorded."
	if len(assisters) > 0 {
		assisterLine = "Assists: " + strings.Join(assisters, ", ")
	}

	return fmt.Sprintf(`You are a satirical sports journalist writing a funny, over-the-top "match report" for a casual pickup soccer group's website.

Write a short punchy headline on its own first line, then 2-4 short paragraphs of exaggerated, comedic sports-journalism prose recapping this match. Base every claim ONLY on the real data below — do not invent any player, event, or stat beyond what's listed. If no goals or assists are listed for a side, do not invent any. Keep it good-natured banter between friends: never cruel, never about anything other than their soccer performance.

Final score: %s %d - %d %s
%s
%s

Write only the headline and article itself, nothing else - no preamble, no quotation marks.`,
		teamAName, scoreA, scoreB, teamBName, scorerLine, assisterLine)
}

// MatchArticleRegenerationCooldown is the minimum time between
// regenerations of a single match's article — same rationale as
// CommentaryRegenerationCooldown.
const MatchArticleRegenerationCooldown = 10 * time.Minute

// LastGeneratedAt returns when the match's article was last (re)generated,
// or nil if none has been created yet.
func (s *MatchArticleService) LastGeneratedAt(ctx context.Context, matchID string) (*time.Time, error) {
	a, err := s.repo.GetMatchArticle(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, nil
	}
	return &a.UpdatedAt, nil
}

// CanRegenerate reports whether enough time has passed since the match's
// article was last generated to allow another regeneration; if not, it
// also reports how much longer the caller needs to wait.
func (s *MatchArticleService) CanRegenerate(ctx context.Context, matchID string) (bool, time.Duration, error) {
	last, err := s.LastGeneratedAt(ctx, matchID)
	if err != nil {
		return false, 0, err
	}
	if last == nil {
		return true, 0, nil
	}
	elapsed := time.Since(*last)
	if elapsed >= MatchArticleRegenerationCooldown {
		return true, 0, nil
	}
	return false, MatchArticleRegenerationCooldown - elapsed, nil
}
