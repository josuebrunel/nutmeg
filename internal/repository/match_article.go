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

// CreateMatchArticle inserts a new article row with its deterministic
// fallback content already in place — the same "always something
// displayable immediately" discipline as CreateGroupActivity. Returns the
// new row's id, needed to enqueue the background job that upgrades it.
func (r *Repository) CreateMatchArticle(ctx context.Context, matchID, fallbackContent string) (string, error) {
	query := psql.Insert(
		im.Into("match_article", "match_id", "content"),
		im.Values(psql.Arg(matchID, fallbackContent)),
		im.Returning("id"),
	)
	return bob.One(ctx, r.db, query, scan.SingleColumnMapper[string])
}

// SetMatchArticleContent upgrades an article row's content with an
// AI-generated version, recording which model produced it.
func (r *Repository) SetMatchArticleContent(ctx context.Context, id, content, model string) error {
	query := psql.Update(
		um.Table("match_article"),
		um.SetCol("content").ToArg(content),
		um.SetCol("model").ToArg(model),
		um.SetCol("updated_at").ToArg(psql.Raw("NOW()")),
		um.Where(psql.Quote("id").EQ(psql.Arg(id))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

// GetMatchArticle returns the match's article, or nil if none has been
// created yet.
func (r *Repository) GetMatchArticle(ctx context.Context, matchID string) (*model.MatchArticle, error) {
	query := psql.Select(
		sm.Columns("id", "match_id", "content", "model", "created_at", "updated_at"),
		sm.From("match_article"),
		sm.Where(psql.Quote("match_id").EQ(psql.Arg(matchID))),
	)
	a, err := bob.One[*model.MatchArticle](ctx, r.db, query, scan.StructMapper[*model.MatchArticle]())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}
