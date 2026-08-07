package service

import (
	"strings"
	"testing"

	"nutmeg/internal/repository"
)

// fixturePlayers returns a small match roster: a and c played for the home
// team (t1), b for the away team (t2).
func fixturePlayers() []repository.MatchPlayerRow {
	return []repository.MatchPlayerRow{
		{PlayerID: "a", TeamID: "t1"},
		{PlayerID: "b", TeamID: "t2"},
		{PlayerID: "c", TeamID: "t1"},
	}
}

func TestGroupEventsByTeam(t *testing.T) {
	names := map[string]string{"a": "Ada", "b": "Bo", "c": "Cal"}
	goals := map[string]int{"a": 2, "b": 1, "c": 1}
	assists := map[string]int{"b": 2, "a": 1}

	var s NewsService
	teamAScorers, teamAAssisters, teamBScorers, teamBAssisters :=
		s.groupEventsByTeam(fixturePlayers(), "t1", goals, assists, names)

	wantAGoals := []string{"Ada (2 goals)", "Cal (1 goal)"}
	assertStringsEqual(t, "team A scorers", teamAScorers, wantAGoals)
	assertStringsEqual(t, "team A assisters", teamAAssisters, []string{"Ada (1 assist)"})
	assertStringsEqual(t, "team B scorers", teamBScorers, []string{"Bo (1 goal)"})
	assertStringsEqual(t, "team B assisters", teamBAssisters, []string{"Bo (2 assists)"})
}

func TestGroupEventsByTeamEmptySide(t *testing.T) {
	var s NewsService
	teamAScorers, teamAAssisters, teamBScorers, teamBAssisters :=
		s.groupEventsByTeam(fixturePlayers(), "t1", map[string]int{}, map[string]int{}, map[string]string{})
	assertStringsEqual(t, "team A scorers", teamAScorers, nil)
	assertStringsEqual(t, "team A assisters", teamAAssisters, nil)
	assertStringsEqual(t, "team B scorers", teamBScorers, nil)
	assertStringsEqual(t, "team B assisters", teamBAssisters, nil)
}

func TestFormatTeamOffense(t *testing.T) {
	got := formatTeamOffense("Shirts", []string{"Ada (2 goals)"}, []string{"Ada (1 assist)"})
	if got != "Shirts (goals: Ada (2 goals); assists: Ada (1 assist))" {
		t.Fatalf("formatTeamOffense = %q", got)
	}

	got = formatTeamOffense("Skins", nil, nil)
	if got != "Skins (goals: none scored; assists: none recorded)" {
		t.Fatalf("formatTeamOffense empty = %q", got)
	}
}

func TestBuildMatchReportPromptAttributesTeams(t *testing.T) {
	prompt := buildMatchReportPrompt("Shirts", "Skins", 4, 3,
		[]string{"Ada (2 goals)", "Cal (1 goal)"}, []string{"Ada (1 assist)"},
		[]string{"Bo (1 goal)"}, nil, nil, nil)

	for _, want := range []string{
		"Final score: Shirts 4 - 3 Skins",
		"Shirts (goals: Ada (2 goals), Cal (1 goal); assists: Ada (1 assist))",
		"Skins (goals: Bo (1 goal); assists: none recorded)",
		"belongs to exactly one team",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestBuildMatchReportPromptCleanSheet(t *testing.T) {
	prompt := buildMatchReportPrompt("Shirts", "Skins", 4, 0,
		[]string{"Ada (4 goals)"}, nil, nil, nil,
		[]string{"Cal"}, []string{"Bo"})

	for _, want := range []string{
		"Shirts's defense (Cal) kept a clean sheet",
		"Skins (goals: none scored; assists: none recorded)",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, "Skins's defense") {
		t.Errorf("Skins' defense must not be credited a clean sheet when Shirts scored %d", 4)
	}
}

func assertStringsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}
}
