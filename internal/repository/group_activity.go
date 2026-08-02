package repository

import (
	"context"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"

	"nutmeg/internal/model"
)

// CreateGroupActivity inserts a new activity row with its deterministic
// fallback content already in place — the feed always has something
// displayable immediately, before any AI upgrade runs. Returns the new
// row's id, needed to enqueue the background job that may replace it.
func (r *Repository) CreateGroupActivity(ctx context.Context, groupID, kind, subjectID, fallbackContent string) (string, error) {
	query := psql.Insert(
		im.Into("group_activity", "group_id", "kind", "subject_id", "content"),
		im.Values(psql.Arg(groupID, kind, subjectID, fallbackContent)),
		im.Returning("id"),
	)
	return bob.One(ctx, r.db, query, scan.SingleColumnMapper[string])
}

// SetGroupActivityContent upgrades an activity row's content with an
// AI-generated version, recording which model produced it.
func (r *Repository) SetGroupActivityContent(ctx context.Context, id, content, model string) error {
	query := psql.Update(
		um.Table("group_activity"),
		um.SetCol("content").ToArg(content),
		um.SetCol("model").ToArg(model),
		um.Where(psql.Quote("id").EQ(psql.Arg(id))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

// ListGroupActivity returns a group's most recent activity entries,
// newest first.
func (r *Repository) ListGroupActivity(ctx context.Context, groupID string, limit int) ([]model.GroupActivity, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "kind", "subject_id", "content", "model", "created_at"),
		sm.From("group_activity"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.OrderBy("created_at").Desc(),
		sm.Limit(limit),
	)
	return bob.All(ctx, r.db, query, scan.StructMapper[model.GroupActivity]())
}
