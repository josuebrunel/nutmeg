package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"

	"nutmeg/internal/model"
)

// CreateGroupNews inserts a new news row with its deterministic fallback
// content already in place — the feed always has something displayable
// immediately, before any AI upgrade runs. Returns the new row's id, needed
// to enqueue the background job that may replace it.
func (r *Repository) CreateGroupNews(ctx context.Context, groupID, kind, subjectID, fallbackContent string) (string, error) {
	query := psql.Insert(
		im.Into("group_news", "group_id", "kind", "subject_id", "content"),
		im.Values(psql.Arg(groupID, kind, subjectID, fallbackContent)),
		im.Returning("id"),
	)
	return bob.One(ctx, r.db, query, scan.SingleColumnMapper[string])
}

// SetGroupNewsContent upgrades a news row's content with an AI-generated
// version, recording which model produced it.
func (r *Repository) SetGroupNewsContent(ctx context.Context, id, content, model string) error {
	query := psql.Update(
		um.Table("group_news"),
		um.SetCol("content").ToArg(content),
		um.SetCol("model").ToArg(model),
		um.SetCol("updated_at").ToArg(psql.Raw("NOW()")),
		um.Where(psql.Quote("id").EQ(psql.Arg(id))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

// ListGroupNews returns a group's most recent news entries, newest first.
func (r *Repository) ListGroupNews(ctx context.Context, groupID string, limit int) ([]model.GroupNews, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "kind", "subject_id", "content", "model", "created_at", "updated_at"),
		sm.From("group_news"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.OrderBy("created_at").Desc(),
		sm.Limit(limit),
	)
	return bob.All(ctx, r.db, query, scan.StructMapper[model.GroupNews]())
}

// GetGroupNewsBySubject returns the group_news row for a given kind and
// subject (e.g. kind="match_logged", subjectID=matchID), or nil if none
// exists yet — the "match_logged" equivalent of the old GetMatchArticle.
func (r *Repository) GetGroupNewsBySubject(ctx context.Context, kind, subjectID string) (*model.GroupNews, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "kind", "subject_id", "content", "model", "created_at", "updated_at"),
		sm.From("group_news"),
		sm.Where(psql.And(psql.Quote("kind").EQ(psql.Arg(kind)), psql.Quote("subject_id").EQ(psql.Arg(subjectID)))),
	)
	n, err := bob.One[*model.GroupNews](ctx, r.db, query, scan.StructMapper[*model.GroupNews]())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return n, nil
}
