package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"nutmeg/internal/model"
	"nutmeg/internal/repository"
)

// maxPlayerSigningLength caps a generated player-signing blurb — a single
// headline-style sentence, much shorter than a full match report.
const maxPlayerSigningLength = 160

// maxMatchReportLength caps a generated match report — a short headline
// plus a few paragraphs, longer-form than the one-line signing blurb.
const maxMatchReportLength = 1800

type NewsRepository interface {
	GetMemberByID(ctx context.Context, memberID string) (*model.GroupPlayer, error)
	GetMatchDetail(ctx context.Context, matchID string) (*repository.MatchDetail, error)
	GetMatchGoals(ctx context.Context, matchID string) (map[string]int, error)
	GetMatchAssists(ctx context.Context, matchID string) (map[string]int, error)
	GetMatchDefenders(ctx context.Context, matchID string) (teamADefenders, teamBDefenders []string, err error)
	SetGroupNewsContent(ctx context.Context, id, content, model string) error
	GetGroupNewsBySubject(ctx context.Context, kind, subjectID string) (*model.GroupNews, error)
}

type NewsService struct {
	repo NewsRepository
	llm  LLMGenerator
}

func NewNewsService(repo NewsRepository, llm LLMGenerator) *NewsService {
	return &NewsService{repo: repo, llm: llm}
}

// GenerateNews upgrades an already-created group_news row's deterministic
// fallback content with AI-generated copy, built strictly from real data for
// the given kind/subject — a short signing blurb for "player_added", a full
// match report for "match_logged". On any failure the row is left with its
// fallback (or prior) content untouched — same "never overwrite a working
// state with a failure" discipline as CommentaryService.Generate.
func (s *NewsService) GenerateNews(ctx context.Context, newsID, kind, subjectID string) error {
	prompt, maxLen, err := s.buildPrompt(ctx, kind, subjectID)
	if err != nil {
		return fmt.Errorf("news: build prompt: %w", err)
	}

	started := time.Now()
	raw, err := s.llm.Generate(ctx, prompt)
	if err != nil {
		slog.Debug("news: llm generate failed", "news_id", newsID, "kind", kind, "model", s.llm.Model(), "duration", time.Since(started))
		return fmt.Errorf("news: generate: %w", err)
	}
	slog.Debug("news: llm generate completed", "news_id", newsID, "kind", kind, "model", s.llm.Model(), "duration", time.Since(started), "chars", len(raw))

	content, err := validateGeneratedText(raw, maxLen)
	if err != nil {
		return fmt.Errorf("news: validate: %w", err)
	}

	if err := s.repo.SetGroupNewsContent(ctx, newsID, content, s.llm.Model()); err != nil {
		return fmt.Errorf("news: store: %w", err)
	}
	return nil
}

func (s *NewsService) buildPrompt(ctx context.Context, kind, subjectID string) (prompt string, maxLen int, err error) {
	switch kind {
	case "player_added":
		player, err := s.repo.GetMemberByID(ctx, subjectID)
		if err != nil {
			return "", 0, fmt.Errorf("fetch player: %w", err)
		}
		return buildPlayerSigningPrompt(player.Name), maxPlayerSigningLength, nil
	case "match_logged":
		match, err := s.repo.GetMatchDetail(ctx, subjectID)
		if err != nil {
			return "", 0, fmt.Errorf("fetch match: %w", err)
		}
		goals, err := s.repo.GetMatchGoals(ctx, subjectID)
		if err != nil {
			return "", 0, fmt.Errorf("fetch goals: %w", err)
		}
		assists, err := s.repo.GetMatchAssists(ctx, subjectID)
		if err != nil {
			return "", 0, fmt.Errorf("fetch assists: %w", err)
		}
		names, err := s.resolvePlayerNames(ctx, goals, assists)
		if err != nil {
			return "", 0, fmt.Errorf("resolve player names: %w", err)
		}
		teamADefenders, teamBDefenders, err := s.repo.GetMatchDefenders(ctx, subjectID)
		if err != nil {
			return "", 0, fmt.Errorf("fetch defenders: %w", err)
		}
		prompt := buildMatchReportPrompt(match.TeamAName, match.TeamBName, match.ScoreA, match.ScoreB,
			formatStatLines(goals, names, "goal"), formatStatLines(assists, names, "assist"),
			teamADefenders, teamBDefenders)
		return prompt, maxMatchReportLength, nil
	default:
		return "", 0, fmt.Errorf("unknown news kind %q", kind)
	}
}

// resolvePlayerNames looks up the display name of every player appearing in
// goals or assists — team rosters are small (pickup soccer), so one
// GetMemberByID call per unique id is simpler than adding a batch query.
func (s *NewsService) resolvePlayerNames(ctx context.Context, goals, assists map[string]int) (map[string]string, error) {
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
// noun[s])" lines, e.g. "Chris (2 goals)" — sorted by count descending then
// name, so the prompt's most notable performers come first.
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

// buildPlayerSigningPrompt is pure — no invented facts, only the player's
// real name is known at this point. Styled as sports-journalism transfer
// news (a "signing" announcement) rather than a chatty group-chat welcome.
func buildPlayerSigningPrompt(playerName string) string {
	return fmt.Sprintf(`You are a sports journalist writing a short transfer-news blurb for a pickup soccer group's website, in the style of a real transfer-announcement headline (e.g. "BREAKING: Club completes signing of Player").

Write one short, punchy sentence announcing this player's arrival to the group like a genuine signing announcement. Do not invent any stat, event, position, or piece of history about them — all that's known is their name.

Player: %s

Write only the announcement itself, nothing else - no preamble, no quotation marks.`, playerName)
}

// buildMatchReportPrompt is pure — built strictly from the real score,
// scorers, assisters, and defenders, no invented players or events.
func buildMatchReportPrompt(teamAName, teamBName string, scoreA, scoreB int, scorers, assisters, teamADefenders, teamBDefenders []string) string {
	scorerLine := "No goals were scored by anyone listed."
	if len(scorers) > 0 {
		scorerLine = "Goal scorers: " + strings.Join(scorers, ", ")
	}
	assisterLine := "No assists were recorded."
	if len(assisters) > 0 {
		assisterLine = "Assists: " + strings.Join(assisters, ", ")
	}

	defenseLine := "No clean sheets this match."
	var cleanSheets []string
	if scoreB == 0 && len(teamADefenders) > 0 {
		cleanSheets = append(cleanSheets, teamAName+"'s defense ("+strings.Join(teamADefenders, ", ")+") kept a clean sheet")
	}
	if scoreA == 0 && len(teamBDefenders) > 0 {
		cleanSheets = append(cleanSheets, teamBName+"'s defense ("+strings.Join(teamBDefenders, ", ")+") kept a clean sheet")
	}
	if len(cleanSheets) > 0 {
		defenseLine = strings.Join(cleanSheets, ". ") + "."
	}

	return fmt.Sprintf(`You are a satirical sports journalist writing a funny, over-the-top "match report" for a casual pickup soccer group's website.

Write a short punchy headline on its own first line, then 2-4 short paragraphs of exaggerated, comedic sports-journalism prose recapping this match. Base every claim ONLY on the real data below — do not invent any player, event, or stat beyond what's listed. If no goals or assists are listed for a side, do not invent any. A clean sheet is a real defensive achievement — when one is listed below, give the defenders credit for it, don't only focus on goal scorers. Keep it good-natured banter between friends: never cruel, never about anything other than their soccer performance.

Final score: %s %d - %d %s
%s
%s
%s

Write only the headline and article itself, nothing else - no preamble, no quotation marks.`,
		teamAName, scoreA, scoreB, teamBName, scorerLine, assisterLine, defenseLine)
}

// MatchReportRegenerationCooldown is the minimum time between
// regenerations of a single match's report — same rationale as
// CommentaryRegenerationCooldown.
const MatchReportRegenerationCooldown = 10 * time.Minute

// LastGeneratedAt returns when the match's report was last (re)generated,
// or nil if none has been created yet.
func (s *NewsService) LastGeneratedAt(ctx context.Context, matchID string) (*time.Time, error) {
	n, err := s.repo.GetGroupNewsBySubject(ctx, "match_logged", matchID)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, nil
	}
	return &n.UpdatedAt, nil
}

// CanRegenerate reports whether enough time has passed since the match's
// report was last generated to allow another regeneration; if not, it also
// reports how much longer the caller needs to wait.
func (s *NewsService) CanRegenerate(ctx context.Context, matchID string) (bool, time.Duration, error) {
	last, err := s.LastGeneratedAt(ctx, matchID)
	if err != nil {
		return false, 0, err
	}
	if last == nil {
		return true, 0, nil
	}
	elapsed := time.Since(*last)
	if elapsed >= MatchReportRegenerationCooldown {
		return true, 0, nil
	}
	return false, MatchReportRegenerationCooldown - elapsed, nil
}
