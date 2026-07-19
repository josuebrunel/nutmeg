package repository

import (
	"context"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"
)

type MatchWithTeams struct {
	ID        string    `db:"id"`
	GroupID   string    `db:"group_id"`
	TeamAName string    `db:"team_a_name"`
	TeamBName string    `db:"team_b_name"`
	ScoreA    int       `db:"score_a"`
	ScoreB    int       `db:"score_b"`
	PlayedAt  time.Time `db:"played_at"`
}

type LeaderboardEntry struct {
	MemberID string `db:"member_id"`
	Name     string `db:"name"`
	Matches  int    `db:"matches"`
	Wins     int    `db:"wins"`
	Losses   int    `db:"losses"`
	Goals    int    `db:"goals"`
	Assists  int    `db:"assists"`
}

type PlayerStats struct {
	MatchesPlayed int `db:"matches_played"`
	Wins          int `db:"wins"`
	Losses        int `db:"losses"`
	Goals         int `db:"goals"`
	Assists       int `db:"assists"`
}

func (r *Repository) CreateMatch(ctx context.Context, groupID, teamAName, teamBName string, scoreA, scoreB int, createdBy string, teamAPlayers, teamBPlayers []string, goals map[string]int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	teamA, err := insertTeam(ctx, tx, groupID, teamAName)
	if err != nil {
		return err
	}
	teamB, err := insertTeam(ctx, tx, groupID, teamBName)
	if err != nil {
		return err
	}

	matchID, err := bob.One[string](ctx, tx, psql.Insert(
		im.Into("matches", "group_id", "home_team_id", "away_team_id", "home_score", "away_score", "created_by"),
		im.Values(psql.Arg(groupID, teamA, teamB, scoreA, scoreB, createdBy)),
		im.Returning("id"),
	), scan.SingleColumnMapper[string])
	if err != nil {
		return err
	}

	for _, pid := range teamAPlayers {
		_, err = bob.Exec(ctx, tx, psql.Insert(
			im.Into("match_players", "match_id", "team_id", "player_id"),
			im.Values(psql.Arg(matchID, teamA, pid)),
		))
		if err != nil {
			return err
		}
	}
	for _, pid := range teamBPlayers {
		_, err = bob.Exec(ctx, tx, psql.Insert(
			im.Into("match_players", "match_id", "team_id", "player_id"),
			im.Values(psql.Arg(matchID, teamB, pid)),
		))
		if err != nil {
			return err
		}
	}

	for playerID, count := range goals {
		teamID := teamA
		if contains(teamBPlayers, playerID) {
			teamID = teamB
		}
		for i := 0; i < count; i++ {
			_, err = bob.Exec(ctx, tx, psql.Insert(
				im.Into("match_events", "match_id", "team_id", "scorer_id"),
				im.Values(psql.Arg(matchID, teamID, playerID)),
			))
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func insertTeam(ctx context.Context, exec bob.Executor, groupID, name string) (string, error) {
	return bob.One[string](ctx, exec, psql.Insert(
		im.Into("teams", "group_id", "name"),
		im.Values(psql.Arg(groupID, name)),
		im.Returning("id"),
	), scan.SingleColumnMapper[string])
}

func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func (r *Repository) ListMatchesByGroup(ctx context.Context, groupID string) ([]MatchWithTeams, error) {
	query := psql.Select(
		sm.Columns(
			"m.id",
			"m.group_id",
			"ha.name AS team_a_name",
			"hb.name AS team_b_name",
			"m.home_score AS score_a",
			"m.away_score AS score_b",
			"m.played_at",
		),
		sm.From("matches m"),
		sm.InnerJoin("teams ha ON ha.id = m.home_team_id"),
		sm.InnerJoin("teams hb ON hb.id = m.away_team_id"),
		sm.Where(psql.Quote("m", "group_id").EQ(psql.Arg(groupID))),
		sm.OrderBy("m.played_at DESC"),
	)
	return bob.All[MatchWithTeams](ctx, r.db, query, scan.StructMapper[MatchWithTeams]())
}

func (r *Repository) DeleteMatch(ctx context.Context, matchID string) error {
	query := psql.Delete(
		dm.From("matches"),
		dm.Where(psql.Quote("id").EQ(psql.Arg(matchID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) GetGroupLeaderboard(ctx context.Context, groupID string) ([]LeaderboardEntry, error) {
	query := psql.RawQuery(`
		SELECT
			gp.id AS member_id,
			gp.name AS name,
			COUNT(DISTINCT mp.match_id) AS matches,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.home_score > m.away_score)
				OR (m.away_team_id = mp.team_id AND m.away_score > m.home_score) THEN mp.match_id END) AS wins,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.home_score < m.away_score)
				OR (m.away_team_id = mp.team_id AND m.away_score < m.home_score) THEN mp.match_id END) AS losses,
			COUNT(DISTINCT me.id) FILTER (WHERE me.scorer_id = gp.id) AS goals,
			COUNT(DISTINCT me2.id) FILTER (WHERE me2.assister_id = gp.id) AS assists
		FROM group_players gp
		LEFT JOIN match_players mp ON mp.player_id = gp.id AND mp.match_id IN (
			SELECT id FROM matches WHERE group_id = $1
		)
		LEFT JOIN matches m ON m.id = mp.match_id
		LEFT JOIN match_events me ON me.match_id = mp.match_id AND me.scorer_id = gp.id
		LEFT JOIN match_events me2 ON me2.match_id = mp.match_id AND me2.assister_id = gp.id
		WHERE gp.group_id = $1
		GROUP BY gp.id, gp.name
		ORDER BY wins DESC, goals DESC
	`, groupID)
	return bob.All[LeaderboardEntry](ctx, r.db, query, scan.StructMapper[LeaderboardEntry]())
}

type MatchDetail struct {
	ID         string    `db:"id"`
	GroupID    string    `db:"group_id"`
	HomeTeamID string    `db:"home_team_id"`
	AwayTeamID string    `db:"away_team_id"`
	TeamAName  string    `db:"team_a_name"`
	TeamBName  string    `db:"team_b_name"`
	ScoreA     int       `db:"score_a"`
	ScoreB     int       `db:"score_b"`
	PlayedAt   time.Time `db:"played_at"`
}

type MatchPlayerRow struct {
	PlayerID string `db:"player_id"`
	TeamID   string `db:"team_id"`
}

func (r *Repository) GetMatchDetail(ctx context.Context, matchID string) (*MatchDetail, error) {
	query := psql.Select(
		sm.Columns(
			"m.id", "m.group_id",
			"m.home_team_id", "m.away_team_id",
			"ha.name AS team_a_name", "hb.name AS team_b_name",
			"m.home_score AS score_a", "m.away_score AS score_b",
			"m.played_at",
		),
		sm.From("matches m"),
		sm.InnerJoin("teams ha ON ha.id = m.home_team_id"),
		sm.InnerJoin("teams hb ON hb.id = m.away_team_id"),
		sm.Where(psql.Quote("m", "id").EQ(psql.Arg(matchID))),
	)
	return bob.One[*MatchDetail](ctx, r.db, query, scan.StructMapper[*MatchDetail]())
}

func (r *Repository) GetMatchPlayers(ctx context.Context, matchID string) ([]MatchPlayerRow, error) {
	query := psql.Select(
		sm.Columns("player_id", "team_id"),
		sm.From("match_players"),
		sm.Where(psql.Quote("match_id").EQ(psql.Arg(matchID))),
	)
	return bob.All[MatchPlayerRow](ctx, r.db, query, scan.StructMapper[MatchPlayerRow]())
}

func (r *Repository) GetMatchGoals(ctx context.Context, matchID string) (map[string]int, error) {
	type goalRow struct {
		ScorerID string `db:"scorer_id"`
		Count    int    `db:"count"`
	}
	query := psql.Select(
		sm.Columns("scorer_id", psql.Raw("COUNT(*) AS count")),
		sm.From("match_events"),
		sm.Where(psql.Quote("match_id").EQ(psql.Arg(matchID))),
		sm.GroupBy("scorer_id"),
	)
	rows, err := bob.All[goalRow](ctx, r.db, query, scan.StructMapper[goalRow]())
	if err != nil {
		return nil, err
	}
	goals := make(map[string]int)
	for _, r := range rows {
		goals[r.ScorerID] = r.Count
	}
	return goals, nil
}

type matchTeamIDs struct {
	Home string `db:"home_team_id"`
	Away string `db:"away_team_id"`
}

func (r *Repository) getMatchTeamIDs(ctx context.Context, exec bob.Executor, matchID string) (string, string, error) {
	query := psql.Select(
		sm.Columns("home_team_id", "away_team_id"),
		sm.From("matches"),
		sm.Where(psql.Quote("id").EQ(psql.Arg(matchID))),
	)
	ids, err := bob.One[*matchTeamIDs](ctx, exec, query, scan.StructMapper[*matchTeamIDs]())
	if err != nil {
		return "", "", err
	}
	return ids.Home, ids.Away, nil
}

func (r *Repository) UpdateMatch(ctx context.Context, matchID, teamAName, teamBName string, scoreA, scoreB int, teamAPlayers, teamBPlayers []string, goals map[string]int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = bob.Exec(ctx, tx, psql.Update(
		um.Table("matches"),
		um.SetCol("home_score").ToArg(scoreA),
		um.SetCol("away_score").ToArg(scoreB),
		um.Where(psql.Quote("id").EQ(psql.Arg(matchID))),
	))
	if err != nil {
		return err
	}

	homeTeamID, awayTeamID, err := r.getMatchTeamIDs(ctx, tx, matchID)
	if err != nil {
		return err
	}

	_, err = bob.Exec(ctx, tx, psql.Update(
		um.Table("teams"),
		um.SetCol("name").ToArg(teamAName),
		um.Where(psql.Quote("id").EQ(psql.Arg(homeTeamID))),
	))
	if err != nil {
		return err
	}
	_, err = bob.Exec(ctx, tx, psql.Update(
		um.Table("teams"),
		um.SetCol("name").ToArg(teamBName),
		um.Where(psql.Quote("id").EQ(psql.Arg(awayTeamID))),
	))
	if err != nil {
		return err
	}

	_, err = bob.Exec(ctx, tx, psql.Delete(
		dm.From("match_players"),
		dm.Where(psql.Quote("match_id").EQ(psql.Arg(matchID))),
	))
	if err != nil {
		return err
	}

	for _, pid := range teamAPlayers {
		_, err = bob.Exec(ctx, tx, psql.Insert(
			im.Into("match_players", "match_id", "team_id", "player_id"),
			im.Values(psql.Arg(matchID, homeTeamID, pid)),
		))
		if err != nil {
			return err
		}
	}
	for _, pid := range teamBPlayers {
		_, err = bob.Exec(ctx, tx, psql.Insert(
			im.Into("match_players", "match_id", "team_id", "player_id"),
			im.Values(psql.Arg(matchID, awayTeamID, pid)),
		))
		if err != nil {
			return err
		}
	}

	_, err = bob.Exec(ctx, tx, psql.Delete(
		dm.From("match_events"),
		dm.Where(psql.Quote("match_id").EQ(psql.Arg(matchID))),
	))
	if err != nil {
		return err
	}

	for playerID, count := range goals {
		teamID := homeTeamID
		if contains(teamBPlayers, playerID) {
			teamID = awayTeamID
		}
		for i := 0; i < count; i++ {
			_, err = bob.Exec(ctx, tx, psql.Insert(
				im.Into("match_events", "match_id", "team_id", "scorer_id"),
				im.Values(psql.Arg(matchID, teamID, playerID)),
			))
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

type GlobalStats struct {
	TotalMatches int `db:"total_matches"`
	TotalGoals   int `db:"total_goals"`
	TotalPlayers int `db:"total_players"`
}

func (r *Repository) GetGlobalStats(ctx context.Context, userID string) (*GlobalStats, error) {
	query := psql.RawQuery(`
		SELECT
			COALESCE((SELECT COUNT(*) FROM matches WHERE group_id IN (
				SELECT id FROM groups WHERE created_by = $1
			)), 0) AS total_matches,
			COALESCE((SELECT COUNT(*) FROM match_events WHERE match_id IN (
				SELECT id FROM matches WHERE group_id IN (
					SELECT id FROM groups WHERE created_by = $1
				)
			)), 0) AS total_goals,
			COALESCE((SELECT COUNT(*) FROM group_players WHERE group_id IN (
				SELECT id FROM groups WHERE created_by = $1
			)), 0) AS total_players
	`, userID)
	return bob.One[*GlobalStats](ctx, r.db, query, scan.StructMapper[*GlobalStats]())
}

func (r *Repository) GetPlayerStats(ctx context.Context, memberID string) (*PlayerStats, error) {
	query := psql.RawQuery(`
		SELECT
			COUNT(DISTINCT mp.match_id) AS matches_played,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.home_score > m.away_score)
				OR (m.away_team_id = mp.team_id AND m.away_score > m.home_score) THEN mp.match_id END) AS wins,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.home_score < m.away_score)
				OR (m.away_team_id = mp.team_id AND m.away_score < m.home_score) THEN mp.match_id END) AS losses,
			COUNT(DISTINCT me.id) FILTER (WHERE me.scorer_id = $1) AS goals,
			0 AS assists
		FROM match_players mp
		JOIN matches m ON m.id = mp.match_id
		LEFT JOIN match_events me ON me.match_id = mp.match_id
		WHERE mp.player_id = $1
	`, memberID)
	return bob.One[*PlayerStats](ctx, r.db, query, scan.StructMapper[*PlayerStats]())
}
