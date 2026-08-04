package repository

import (
	"context"
	"slices"
	"strconv"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"

	"nutmeg/internal/model"
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
	MemberID    string  `db:"member_id"`
	Name        string  `db:"name"`
	Position    *string `db:"position"`
	Matches     int     `db:"matches"`
	Wins        int     `db:"wins"`
	Draws       int     `db:"draws"`
	Losses      int     `db:"losses"`
	Goals       int     `db:"goals"`
	Assists     int     `db:"assists"`
	CleanSheets int     `db:"clean_sheets"`
	Score       float64 `db:"score"`
	Qualified   bool    `db:"qualified"`
}

// TopScorerID returns the member id with the strictly-highest Goals in
// entries, or "" if entries is empty or nobody has scored yet — so no
// caller awards a "top scorer" badge before any goals exist.
func TopScorerID(entries []LeaderboardEntry) string {
	id, max := "", 0
	for _, e := range entries {
		if e.Goals > max {
			id, max = e.MemberID, e.Goals
		}
	}
	return id
}

// TopPasserID is TopScorerID's counterpart for Assists.
func TopPasserID(entries []LeaderboardEntry) string {
	id, max := "", 0
	for _, e := range entries {
		if e.Assists > max {
			id, max = e.MemberID, e.Assists
		}
	}
	return id
}

// TopDefenderID is TopScorerID's counterpart for CleanSheets, restricted to
// goalkeepers and defenders — a forward who happened to play on a
// clean-sheet-heavy team shouldn't wear the defensive badge.
func TopDefenderID(entries []LeaderboardEntry) string {
	id, max := "", 0
	for _, e := range entries {
		if e.Position == nil {
			continue
		}
		if *e.Position != model.PositionGK && *e.Position != model.PositionD {
			continue
		}
		if e.CleanSheets > max {
			id, max = e.MemberID, e.CleanSheets
		}
	}
	return id
}

// PlayerLeaderboardEntry returns the entry belonging to memberID, or
// false if the leaderboard doesn't contain them (it always should, for
// any real member of the group — GetGroupLeaderboard is a LEFT JOIN from
// group_players, so even a player with zero matches gets a row).
func PlayerLeaderboardEntry(entries []LeaderboardEntry, memberID string) (LeaderboardEntry, bool) {
	for _, e := range entries {
		if e.MemberID == memberID {
			return e, true
		}
	}
	return LeaderboardEntry{}, false
}

type PlayerStats struct {
	MatchesPlayed int `db:"matches_played"`
	Wins          int `db:"wins"`
	Draws         int `db:"draws"`
	Losses        int `db:"losses"`
	Goals         int `db:"goals"`
	Assists       int `db:"assists"`
	CleanSheets   int `db:"clean_sheets"`
}

// PlayerMatchResult is one match from a player's history, most-recent
// first — the raw material for streak derivation (see service layer),
// which compares TeamID against HomeTeamID/AwayTeamID and the score
// columns directly rather than recomputing a result from match_events.
type PlayerMatchResult struct {
	MatchID     string    `db:"match_id"`
	PlayedAt    time.Time `db:"played_at"`
	TeamID      string    `db:"team_id"`
	HomeTeamID  string    `db:"home_team_id"`
	AwayTeamID  string    `db:"away_team_id"`
	HomeScore   int       `db:"home_score"`
	AwayScore   int       `db:"away_score"`
	GoalsScored int       `db:"goals_scored"`
}

// Result classifies the match from this player's team's perspective —
// "win", "loss", or "draw" — by comparing TeamID against HomeTeamID/
// AwayTeamID and the stored score columns directly, never recomputed from
// match_events.
func (m PlayerMatchResult) Result() string {
	if m.HomeScore == m.AwayScore {
		return "draw"
	}
	if m.TeamID == m.HomeTeamID {
		if m.HomeScore > m.AwayScore {
			return "win"
		}
		return "loss"
	}
	if m.AwayScore > m.HomeScore {
		return "win"
	}
	return "loss"
}

// CreateMatch returns the new match's id, primarily so callers can enqueue
// per-player follow-up work (e.g. async commentary generation) tied to the
// match that triggered it.
func (r *Repository) CreateMatch(ctx context.Context, groupID, teamAName, teamBName string, scoreA, scoreB int, createdBy string, teamAPlayers, teamBPlayers []string, goals, assists map[string]int, playedAt time.Time) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	teamA, err := insertTeam(ctx, tx, groupID, teamAName)
	if err != nil {
		return "", err
	}
	teamB, err := insertTeam(ctx, tx, groupID, teamBName)
	if err != nil {
		return "", err
	}

	matchID, err := bob.One[string](ctx, tx, psql.Insert(
		im.Into("matches", "group_id", "home_team_id", "away_team_id", "home_score", "away_score", "created_by", "played_at"),
		im.Values(psql.Arg(groupID, teamA, teamB, scoreA, scoreB, createdBy, playedAt)),
		im.Returning("id"),
	), scan.SingleColumnMapper[string])
	if err != nil {
		return "", err
	}

	for _, pid := range teamAPlayers {
		_, err = bob.Exec(ctx, tx, psql.Insert(
			im.Into("match_players", "match_id", "team_id", "player_id"),
			im.Values(psql.Arg(matchID, teamA, pid)),
		))
		if err != nil {
			return "", err
		}
	}
	for _, pid := range teamBPlayers {
		_, err = bob.Exec(ctx, tx, psql.Insert(
			im.Into("match_players", "match_id", "team_id", "player_id"),
			im.Values(psql.Arg(matchID, teamB, pid)),
		))
		if err != nil {
			return "", err
		}
	}

	if err := insertGoalEvents(ctx, tx, matchID, teamA, teamB, teamBPlayers, goals, assists); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return matchID, nil
}

// insertGoalEvents writes one match_events row per goal, distributing the
// flat per-player assist tally across that team's goal rows (one assister
// per goal row, first-come-first-served) so assist counts can later be
// derived with a simple COUNT(...WHERE assister_id = ?) query. The pairing
// of a specific assist to a specific goal row is arbitrary — only the
// per-team, per-match assist total is meaningful.
func insertGoalEvents(ctx context.Context, tx bob.Executor, matchID, teamA, teamB string, teamBPlayers []string, goals, assists map[string]int) error {
	assistQueueA, assistQueueB := buildAssistQueues(assists, teamBPlayers)

	for playerID, count := range goals {
		teamID := teamA
		queue := &assistQueueA
		if slices.Contains(teamBPlayers, playerID) {
			teamID = teamB
			queue = &assistQueueB
		}
		for i := 0; i < count; i++ {
			var assisterID *string
			if len(*queue) > 0 {
				a := (*queue)[0]
				assisterID = &a
				*queue = (*queue)[1:]
			}
			_, err := bob.Exec(ctx, tx, psql.Insert(
				im.Into("match_events", "match_id", "team_id", "scorer_id", "assister_id"),
				im.Values(psql.Arg(matchID, teamID, playerID, assisterID)),
			))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// buildAssistQueues flattens the assist tally into per-team queues of
// assister player IDs, one entry per assist, so each queue can be popped
// once per goal-event row inserted for that team.
func buildAssistQueues(assists map[string]int, teamBPlayers []string) (queueA, queueB []string) {
	for playerID, count := range assists {
		for i := 0; i < count; i++ {
			if slices.Contains(teamBPlayers, playerID) {
				queueB = append(queueB, playerID)
			} else {
				queueA = append(queueA, playerID)
			}
		}
	}
	return
}

func insertTeam(ctx context.Context, exec bob.Executor, groupID, name string) (string, error) {
	return bob.One[string](ctx, exec, psql.Insert(
		im.Into("teams", "group_id", "name"),
		im.Values(psql.Arg(groupID, name)),
		im.Returning("id"),
	), scan.SingleColumnMapper[string])
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

// minMatchesForRanking is the number of matches a player must have played
// before they're ranked by performance score rather than raw wins — a
// single lucky win otherwise puts a 1-match player at the top of the
// board. Players below this bar are listed beneath every qualified
// player, ordered by wins among themselves. When nobody in a group has
// reached this yet (e.g. a brand-new group), everyone falls into that
// same tier together and is ranked by wins — nobody is singled out.
const minMatchesForRanking = 3

// leaderboardOrderClause whitelists the leaderboard sort key so it can be
// safely spliced into the raw SQL's ORDER BY without risking injection from
// an arbitrary query param. The default ranks by performance score
// (qualified players first, then score, with wins as the final tiebreak —
// which also fully orders the unqualified tier, since none of them are
// "qualified"); goals/assists/matches remain raw-count sorts.
func leaderboardOrderClause(sortBy string) string {
	switch sortBy {
	case "goals":
		return "goals DESC, wins DESC"
	case "assists":
		return "assists DESC, wins DESC"
	case "clean_sheets":
		return "clean_sheets DESC, wins DESC"
	case "matches":
		return "matches DESC, wins DESC"
	default:
		return "qualified DESC, score DESC, wins DESC"
	}
}

// cleanSheetScoreBonus is how many score points a clean sheet is worth for
// a goalkeeper or defender — the same weight class as a goal or assist, so
// a defender who shuts out the other team can climb the board the way a
// forward does by scoring. Forwards/midfielders don't get this bonus even
// though a clean sheet is technically a team-wide fact, so it can't be
// farmed for free points by a position it isn't that player's job to hold.
const cleanSheetScoreBonus = 2.0

func (r *Repository) GetGroupLeaderboard(ctx context.Context, groupID string, sortBy string) ([]LeaderboardEntry, error) {
	query := psql.RawQuery(`
		WITH stats AS (
			SELECT
				gp.id AS member_id,
				gp.name AS name,
				gp.position AS position,
				COUNT(DISTINCT mp.match_id) AS matches,
				COUNT(DISTINCT mp.match_id) FILTER (WHERE
					(m.home_team_id = mp.team_id AND m.home_score > m.away_score)
					OR (m.away_team_id = mp.team_id AND m.away_score > m.home_score)
				) AS wins,
				COUNT(DISTINCT mp.match_id) FILTER (WHERE m.home_score = m.away_score) AS draws,
				COUNT(DISTINCT mp.match_id) FILTER (WHERE
					(m.home_team_id = mp.team_id AND m.home_score < m.away_score)
					OR (m.away_team_id = mp.team_id AND m.away_score < m.home_score)
				) AS losses,
				COUNT(DISTINCT me.id) AS goals,
				COUNT(DISTINCT mea.id) AS assists,
				COUNT(DISTINCT mp.match_id) FILTER (WHERE
					(m.home_team_id = mp.team_id AND m.away_score = 0)
					OR (m.away_team_id = mp.team_id AND m.home_score = 0)
				) AS clean_sheets
			FROM group_players gp
			LEFT JOIN matches m ON m.group_id = gp.group_id
			LEFT JOIN match_players mp ON mp.match_id = m.id AND mp.player_id = gp.id
			LEFT JOIN match_events me ON me.match_id = m.id AND me.scorer_id = gp.id
			LEFT JOIN match_events mea ON mea.match_id = m.id AND mea.assister_id = gp.id
			WHERE gp.group_id = ?
			GROUP BY gp.id, gp.name, gp.position
		)
		SELECT
			member_id, name, position, matches, wins, draws, losses, goals, assists, clean_sheets,
			CASE WHEN matches > 0
				THEN (3.0 * wins + draws + goals + assists +
					CASE WHEN position IN ('`+model.PositionGK+`', '`+model.PositionD+`')
						THEN `+strconv.FormatFloat(cleanSheetScoreBonus, 'f', -1, 64)+` * clean_sheets
						ELSE 0
					END) / matches
				ELSE 0
			END AS score,
			(matches >= `+strconv.Itoa(minMatchesForRanking)+`) AS qualified
		FROM stats
		ORDER BY `+leaderboardOrderClause(sortBy)+`
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

func (r *Repository) GetMatchAssists(ctx context.Context, matchID string) (map[string]int, error) {
	type assistRow struct {
		AssisterID string `db:"assister_id"`
		Count      int    `db:"count"`
	}
	query := psql.RawQuery(`
		SELECT assister_id, COUNT(*) AS count
		FROM match_events
		WHERE match_id = ? AND assister_id IS NOT NULL
		GROUP BY assister_id
	`, matchID)
	rows, err := bob.All[assistRow](ctx, r.db, query, scan.StructMapper[assistRow]())
	if err != nil {
		return nil, err
	}
	assists := make(map[string]int)
	for _, row := range rows {
		assists[row.AssisterID] = row.Count
	}
	return assists, nil
}

// GetMatchDefenders returns the names of goalkeepers/defenders on each side
// of a match — teamADefenders played for the home team, teamBDefenders for
// the away team, matching the Team A/Team B convention used by
// GetMatchDetail. Used to give the match-report prompt enough to credit a
// clean sheet to the players actually responsible for it.
func (r *Repository) GetMatchDefenders(ctx context.Context, matchID string) (teamADefenders, teamBDefenders []string, err error) {
	type defenderRow struct {
		Name   string `db:"name"`
		IsHome bool   `db:"is_home"`
	}
	query := psql.RawQuery(`
		SELECT gp.name AS name, (mp.team_id = m.home_team_id) AS is_home
		FROM match_players mp
		JOIN matches m ON m.id = mp.match_id
		JOIN group_players gp ON gp.id = mp.player_id
		WHERE mp.match_id = ? AND gp.position IN (?, ?)
		ORDER BY gp.name
	`, matchID, model.PositionGK, model.PositionD)
	rows, err := bob.All[defenderRow](ctx, r.db, query, scan.StructMapper[defenderRow]())
	if err != nil {
		return nil, nil, err
	}
	for _, row := range rows {
		if row.IsHome {
			teamADefenders = append(teamADefenders, row.Name)
		} else {
			teamBDefenders = append(teamBDefenders, row.Name)
		}
	}
	return teamADefenders, teamBDefenders, nil
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

func (r *Repository) UpdateMatch(ctx context.Context, matchID, teamAName, teamBName string, scoreA, scoreB int, teamAPlayers, teamBPlayers []string, goals, assists map[string]int, playedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = bob.Exec(ctx, tx, psql.Update(
		um.Table("matches"),
		um.SetCol("home_score").ToArg(scoreA),
		um.SetCol("away_score").ToArg(scoreB),
		um.SetCol("played_at").ToArg(playedAt),
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

	if err := insertGoalEvents(ctx, tx, matchID, homeTeamID, awayTeamID, teamBPlayers, goals, assists); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type GlobalStats struct {
	TotalMatches int `db:"total_matches"`
	TotalGoals   int `db:"total_goals"`
	TotalAssists int `db:"total_assists"`
	TotalDraws   int `db:"total_draws"`
	TotalPlayers int `db:"total_players"`
}

func (r *Repository) GetGlobalStats(ctx context.Context, userID string) (*GlobalStats, error) {
	query := psql.RawQuery(`
		SELECT
			(SELECT COUNT(*) FROM matches WHERE group_id IN (SELECT id FROM groups WHERE created_by = ?)) AS total_matches,
			(SELECT COUNT(*) FROM match_events WHERE match_id IN (SELECT id FROM matches WHERE group_id IN (SELECT id FROM groups WHERE created_by = ?))) AS total_goals,
			(SELECT COUNT(*) FROM match_events WHERE assister_id IS NOT NULL AND match_id IN (SELECT id FROM matches WHERE group_id IN (SELECT id FROM groups WHERE created_by = ?))) AS total_assists,
			(SELECT COUNT(*) FROM matches WHERE home_score = away_score AND group_id IN (SELECT id FROM groups WHERE created_by = ?)) AS total_draws,
			(SELECT COUNT(*) FROM group_players WHERE group_id IN (SELECT id FROM groups WHERE created_by = ?)) AS total_players
	`, userID, userID, userID, userID, userID)
	return bob.One[*GlobalStats](ctx, r.db, query, scan.StructMapper[*GlobalStats]())
}

func (r *Repository) GetPlayerStats(ctx context.Context, memberID string) (*PlayerStats, error) {
	query := psql.RawQuery(`
		SELECT
			COUNT(DISTINCT mp.match_id) AS matches_played,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.home_score > m.away_score)
				OR (m.away_team_id = mp.team_id AND m.away_score > m.home_score) THEN mp.match_id END) AS wins,
			COUNT(DISTINCT CASE WHEN m.home_score = m.away_score THEN mp.match_id END) AS draws,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.home_score < m.away_score)
				OR (m.away_team_id = mp.team_id AND m.away_score < m.home_score) THEN mp.match_id END) AS losses,
			COUNT(DISTINCT me.id) FILTER (WHERE me.scorer_id = ?) AS goals,
			COUNT(DISTINCT me.id) FILTER (WHERE me.assister_id = ?) AS assists,
			COUNT(DISTINCT CASE WHEN (m.home_team_id = mp.team_id AND m.away_score = 0)
				OR (m.away_team_id = mp.team_id AND m.home_score = 0) THEN mp.match_id END) AS clean_sheets
		FROM match_players mp
		JOIN matches m ON m.id = mp.match_id
		LEFT JOIN match_events me ON me.match_id = mp.match_id
		WHERE mp.player_id = ?
	`, memberID, memberID, memberID)
	return bob.One[*PlayerStats](ctx, r.db, query, scan.StructMapper[*PlayerStats]())
}

// GetPlayerMatchHistory returns a player's most recent matches, newest
// first, capped at limit — used to derive streaks (scoreless/losing) that
// GetPlayerStats' aggregate counts can't express.
func (r *Repository) GetPlayerMatchHistory(ctx context.Context, memberID string, limit int) ([]PlayerMatchResult, error) {
	query := psql.RawQuery(`
		SELECT
			m.id AS match_id,
			m.played_at AS played_at,
			mp.team_id AS team_id,
			m.home_team_id AS home_team_id,
			m.away_team_id AS away_team_id,
			m.home_score AS home_score,
			m.away_score AS away_score,
			COUNT(me.id) FILTER (WHERE me.scorer_id = ?) AS goals_scored
		FROM match_players mp
		JOIN matches m ON m.id = mp.match_id
		LEFT JOIN match_events me ON me.match_id = mp.match_id
		WHERE mp.player_id = ?
		GROUP BY m.id, mp.team_id
		ORDER BY m.played_at DESC
		LIMIT ?
	`, memberID, memberID, limit)
	return bob.All[PlayerMatchResult](ctx, r.db, query, scan.StructMapper[PlayerMatchResult]())
}
