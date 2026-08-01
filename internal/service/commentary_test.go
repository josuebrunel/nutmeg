package service

import (
	"testing"

	"nutmeg/internal/assert"
	"nutmeg/internal/repository"
)

func TestValidateRoast(t *testing.T) {
	t.Run("empty input is rejected", func(t *testing.T) {
		_, err := validateRoast("   ")
		assert.ErrIs(t, err, errEmptyGeneration)
	})

	t.Run("blocklisted content is rejected", func(t *testing.T) {
		_, err := validateRoast("you should kys honestly")
		assert.ErrIs(t, err, errBlockedGeneration)
	})

	t.Run("well-formed input passes through unchanged", func(t *testing.T) {
		got, err := validateRoast("  Three goals this season and zero of them on target. Impressive, in a way.  ")
		assert.NoErr(t, err)
		assert.Eq(t, got, "Three goals this season and zero of them on target. Impressive, in a way.")
	})

	t.Run("overlong input is truncated at a sentence boundary", func(t *testing.T) {
		long := "This is the first sentence of a roast that goes on forever. " +
			"Here is a second sentence that keeps padding the length out further and further and further and further and further and further and further and further. " +
			"And a third one that should never survive truncation."
		got, err := validateRoast(long)
		assert.NoErr(t, err)
		assert.True(t, len(got) <= maxRoastLength)
		assert.True(t, got[len(got)-1] == '.')
	})
}

func TestStreaks(t *testing.T) {
	homeTeam, awayTeam := "team-home", "team-away"
	newestFirst := func(rows ...repository.PlayerMatchResult) []repository.PlayerMatchResult { return rows }

	t.Run("scoreless streak stops at the first match with a goal", func(t *testing.T) {
		history := newestFirst(
			repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 1, AwayScore: 1, GoalsScored: 0},
			repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 2, AwayScore: 0, GoalsScored: 0},
			repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 3, AwayScore: 1, GoalsScored: 2}, // scored here
			repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 1, AwayScore: 0, GoalsScored: 0},
		)
		assert.EqN(t, scorelessStreak(history), 2)
	})

	t.Run("losing streak stops at the first non-loss", func(t *testing.T) {
		history := newestFirst(
			// player on away team, away loses both times (home > away)
			repository.PlayerMatchResult{TeamID: awayTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 2, AwayScore: 0},
			repository.PlayerMatchResult{TeamID: awayTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 3, AwayScore: 1},
			repository.PlayerMatchResult{TeamID: awayTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 1, AwayScore: 1}, // draw breaks it
			repository.PlayerMatchResult{TeamID: awayTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 0, AwayScore: 5},
		)
		assert.EqN(t, losingStreak(history), 2)
	})

	t.Run("no streak when the most recent match wasn't a loss or scoreless", func(t *testing.T) {
		history := newestFirst(
			repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 3, AwayScore: 1, GoalsScored: 1},
		)
		assert.EqN(t, scorelessStreak(history), 0)
		assert.EqN(t, losingStreak(history), 0)
	})
}

func TestMatchResult(t *testing.T) {
	homeTeam, awayTeam := "team-home", "team-away"

	cases := []struct {
		name string
		m    repository.PlayerMatchResult
		want string
	}{
		{"home team win", repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 3, AwayScore: 1}, "win"},
		{"home team loss", repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 1, AwayScore: 3}, "loss"},
		{"away team win", repository.PlayerMatchResult{TeamID: awayTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 0, AwayScore: 2}, "win"},
		{"away team loss", repository.PlayerMatchResult{TeamID: awayTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 2, AwayScore: 0}, "loss"},
		{"draw", repository.PlayerMatchResult{TeamID: homeTeam, HomeTeamID: homeTeam, AwayTeamID: awayTeam, HomeScore: 2, AwayScore: 2}, "draw"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Eq(t, matchResult(tc.m), tc.want)
		})
	}
}

func TestBuildPrompt_NoInventedStats(t *testing.T) {
	s := &CommentaryService{}
	stats := repository.PlayerStats{MatchesPlayed: 5, Wins: 2, Draws: 1, Losses: 2, Goals: 3, Assists: 1}
	prompt := s.BuildPrompt("Chris", stats, nil)

	assert.StrContains(t, prompt, "Chris")
	assert.StrContains(t, prompt, "2 wins, 1 draws, 2 losses")
	assert.StrContains(t, prompt, "Goals: 3")
	assert.StrContains(t, prompt, "Assists: 1")
	assert.StrContains(t, prompt, "Do not invent")
	assert.StrContains(t, prompt, "minutes played")
}
