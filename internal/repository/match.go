package repository

import (
	"context"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
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
