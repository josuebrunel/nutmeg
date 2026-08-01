package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"
)

type PlayerCommentary struct {
	ID            string    `db:"id"`
	GroupPlayerID string    `db:"group_player_id"`
	Content       string    `db:"content"`
	Model         string    `db:"model"`
	CreatedAt     time.Time `db:"created_at"`
}

// GetActivePlayerCommentary returns the player's current (non-superseded)
// commentary, or nil if none has been generated yet.
func (r *Repository) GetActivePlayerCommentary(ctx context.Context, groupPlayerID string) (*PlayerCommentary, error) {
	query := psql.Select(
		sm.Columns("id", "group_player_id", "content", "model", "created_at"),
		sm.From("player_commentary"),
		sm.Where(psql.Quote("group_player_id").EQ(psql.Arg(groupPlayerID))),
		sm.Where(psql.Raw("NOT superseded")),
	)
	pc, err := bob.One[*PlayerCommentary](ctx, r.db, query, scan.StructMapper[*PlayerCommentary]())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return pc, nil
}

// ReplacePlayerCommentary supersedes the player's current active commentary
// (if any) and inserts new content, atomically. Callers must only invoke
// this after the new content has already passed validation, so a bad
// generation never replaces a working one.
func (r *Repository) ReplacePlayerCommentary(ctx context.Context, groupPlayerID string, matchID *string, content, model string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = bob.Exec(ctx, tx, psql.Update(
		um.Table("player_commentary"),
		um.SetCol("superseded").ToArg(true),
		um.Where(psql.Quote("group_player_id").EQ(psql.Arg(groupPlayerID))),
		um.Where(psql.Raw("NOT superseded")),
	))
	if err != nil {
		return err
	}

	_, err = bob.Exec(ctx, tx, psql.Insert(
		im.Into("player_commentary", "group_player_id", "match_id", "content", "model"),
		im.Values(psql.Arg(groupPlayerID, matchID, content, model)),
	))
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
