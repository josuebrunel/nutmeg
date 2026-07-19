package repository

import (
	"context"
	"os"
	"testing"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/scan"

	"nutmeg/internal/database"
	"nutmeg/migrations"
)

func openTestDB(t *testing.T) *Repository {
	t.Helper()

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@db:5432/nutmeg?sslmode=disable"
	}

	db, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(db, migrations.FS); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return New(bob.NewDB(db))
}

var (
	testGroupID  = "a0000000-0000-0000-0000-000000000001"
	testAliceID  = "a0000000-0000-0000-0000-000000000002"
	testBobID    = "a0000000-0000-0000-0000-000000000003"
	testCarolID  = "a0000000-0000-0000-0000-000000000004"
	testCreator  = "a0000000-0000-0000-0000-0000000000ff"
)

func TestGetGroupLeaderboard(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	t.Run("empty group", func(t *testing.T) {
		entries, err := repo.GetGroupLeaderboard(ctx, "00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("with match data", func(t *testing.T) {
		cleanup := seedForLeaderboard(t, repo, testGroupID, testAliceID, testBobID, testCarolID)
		t.Cleanup(cleanup)

		entries, err := repo.GetGroupLeaderboard(ctx, testGroupID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(entries) == 0 {
			t.Fatal("expected non-empty leaderboard")
		}

		var alice, bob, carol LeaderboardEntry
		for _, e := range entries {
			switch e.Name {
			case "Alice":
				alice = e
			case "Bob":
				bob = e
			case "Carol":
				carol = e
			}
		}

		if alice.Name == "" {
			t.Fatal("Alice not found in leaderboard")
		}
		if alice.Matches != 1 {
			t.Fatalf("Alice matches: want 1, got %d", alice.Matches)
		}
		if alice.Wins != 1 {
			t.Fatalf("Alice wins: want 1, got %d", alice.Wins)
		}
		if alice.Losses != 0 {
			t.Fatalf("Alice losses: want 0, got %d", alice.Losses)
		}
		if alice.Goals != 2 {
			t.Fatalf("Alice goals: want 2, got %d", alice.Goals)
		}

		if bob.Name == "" {
			t.Fatal("Bob not found in leaderboard")
		}
		if bob.Matches != 1 {
			t.Fatalf("Bob matches: want 1, got %d", bob.Matches)
		}
		if bob.Wins != 0 {
			t.Fatalf("Bob wins: want 0, got %d", bob.Wins)
		}
		if bob.Losses != 1 {
			t.Fatalf("Bob losses: want 1, got %d", bob.Losses)
		}
		if bob.Goals != 0 {
			t.Fatalf("Bob goals: want 0, got %d", bob.Goals)
		}

		if carol.Name == "" {
			t.Fatal("Carol not found in leaderboard")
		}
		if carol.Matches != 1 {
			t.Fatalf("Carol matches: want 1, got %d", carol.Matches)
		}
		if carol.Goals != 0 {
			t.Fatalf("Carol goals: want 0, got %d", carol.Goals)
		}
	})
}

func TestGetGlobalStats(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	t.Run("no groups", func(t *testing.T) {
		stats, err := repo.GetGlobalStats(ctx, "00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.TotalMatches != 0 || stats.TotalGoals != 0 || stats.TotalPlayers != 0 {
			t.Fatalf("expected all zeros, got %+v", stats)
		}
	})

	t.Run("with data", func(t *testing.T) {
		cleanup := seedForLeaderboard(t, repo, testGroupID, testAliceID, testBobID, testCarolID)
		t.Cleanup(cleanup)

		stats, err := repo.GetGlobalStats(ctx, testCreator)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stats.TotalMatches != 1 {
			t.Fatalf("TotalMatches: want 1, got %d", stats.TotalMatches)
		}
		if stats.TotalGoals != 2 {
			t.Fatalf("TotalGoals: want 2, got %d", stats.TotalGoals)
		}
		if stats.TotalPlayers != 3 {
			t.Fatalf("TotalPlayers: want 3, got %d", stats.TotalPlayers)
		}
	})
}

func TestCreateMatchThenList(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	cleanup := seedGroupAndMembers(t, repo, testGroupID, testAliceID, testBobID, testCarolID)
	t.Cleanup(cleanup)

	err := repo.CreateMatch(ctx, testGroupID, "Reds", "Blues", 3, 1, testCreator,
		[]string{testAliceID, testCarolID}, []string{testBobID},
		map[string]int{testAliceID: 2})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	matches, err := repo.ListMatchesByGroup(ctx, testGroupID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].ScoreA != 3 {
		t.Fatalf("ScoreA: want 3, got %d", matches[0].ScoreA)
	}
	if matches[0].ScoreB != 1 {
		t.Fatalf("ScoreB: want 1, got %d", matches[0].ScoreB)
	}
	if matches[0].TeamAName != "Reds" {
		t.Fatalf("TeamAName: want Reds, got %s", matches[0].TeamAName)
	}
	if matches[0].TeamBName != "Blues" {
		t.Fatalf("TeamBName: want Blues, got %s", matches[0].TeamBName)
	}
}

// seedGroupAndMembers creates a group with 3 players and returns a cleanup func.
func seedGroupAndMembers(t *testing.T, repo *Repository, groupID, aliceID, bobID, carolID string) func() {
	t.Helper()
	ctx := context.Background()

	db := repo.DB()
	exec := func(sql string, args ...any) {
		_, err := bob.Exec(ctx, db, psql.RawQuery(sql, args...))
		if err != nil {
			t.Fatalf("exec error: %v\nsql: %s\nargs: %v", err, sql, args)
		}
	}

	exec(`INSERT INTO groups (id, name, created_by) VALUES (?, 'Test Group', ?)`, groupID, testCreator)
	exec(`INSERT INTO group_players (id, group_id, name, role) VALUES (?, ?, 'Alice', 'admin')`, aliceID, groupID)
	exec(`INSERT INTO group_players (id, group_id, name, role) VALUES (?, ?, 'Bob', 'member')`, bobID, groupID)
	exec(`INSERT INTO group_players (id, group_id, name, role) VALUES (?, ?, 'Carol', 'member')`, carolID, groupID)

	return func() {
		db := repo.DB()
		bob.Exec(ctx, db, psql.RawQuery(`DELETE FROM group_players WHERE group_id = ?`, groupID))
		bob.Exec(ctx, db, psql.RawQuery(`DELETE FROM groups WHERE id = ?`, groupID))
	}
}

func seedForLeaderboard(t *testing.T, repo *Repository, groupID, aliceID, bobID, carolID string) func() {
	t.Helper()
	ctx := context.Background()

	innerCleanup := seedGroupAndMembers(t, repo, groupID, aliceID, bobID, carolID)

	err := repo.CreateMatch(ctx, groupID, "Reds", "Blues", 3, 1, testCreator,
		[]string{aliceID, carolID}, []string{bobID},
		map[string]int{aliceID: 2})
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	matchIDs, err := bob.All[string](ctx, repo.DB(), psql.RawQuery(`SELECT id FROM matches WHERE group_id = ?`, groupID), scan.SingleColumnMapper[string])
	if err != nil {
		t.Fatalf("failed to get match IDs: %v", err)
	}

	return func() {
		for _, mid := range matchIDs {
			bob.Exec(ctx, repo.DB(), psql.RawQuery(`DELETE FROM match_events WHERE match_id = ?`, mid))
			bob.Exec(ctx, repo.DB(), psql.RawQuery(`DELETE FROM match_players WHERE match_id = ?`, mid))
		}
		for _, mid := range matchIDs {
			bob.Exec(ctx, repo.DB(), psql.RawQuery(`DELETE FROM matches WHERE id = ?`, mid))
		}
		bob.Exec(ctx, repo.DB(), psql.RawQuery(`DELETE FROM teams WHERE group_id = ?`, groupID))
		innerCleanup()
	}
}
