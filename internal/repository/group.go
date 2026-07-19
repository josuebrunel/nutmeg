package repository

import (
	"context"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/im"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/scan"

	"nutmeg/internal/model"
)

func (r *Repository) CreateGroup(ctx context.Context, g *model.Group) error {
	query := psql.Insert(
		im.Into("groups", "name", "description", "created_by"),
		im.Values(psql.Arg(g.Name, g.Description, g.CreatedBy)),
		im.Returning("id", "created_at", "updated_at"),
	)
	result, err := bob.One(ctx, r.db, query, scan.StructMapper[*model.Group]())
	if err != nil {
		return err
	}
	*g = *result
	return nil
}

func (r *Repository) GetGroup(ctx context.Context, id string) (*model.Group, error) {
	query := psql.Select(
		sm.Columns("id", "name", "description", "created_by", "created_at", "updated_at"),
		sm.From("groups"),
		sm.Where(psql.Quote("id").EQ(psql.Arg(id))),
	)
	return bob.One(ctx, r.db, query, scan.StructMapper[*model.Group]())
}

func (r *Repository) ListGroups(ctx context.Context, userID string) ([]*model.Group, error) {
	query := psql.Select(
		sm.Columns("id", "name", "description", "created_by", "created_at", "updated_at"),
		sm.From("groups"),
		sm.Where(psql.Quote("created_by").EQ(psql.Arg(userID))),
		sm.OrderBy("name"),
	)
	return bob.All(ctx, r.db, query, scan.StructMapper[*model.Group]())
}

func (r *Repository) UpdateGroup(ctx context.Context, g *model.Group) error {
	query := psql.Update(
		um.Table("groups"),
		um.SetCol("name").ToArg(g.Name),
		um.SetCol("description").ToArg(g.Description),
		um.SetCol("updated_at").ToArg(psql.Raw("NOW()")),
		um.Where(psql.Quote("id").EQ(psql.Arg(g.ID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) DeleteGroup(ctx context.Context, id string) error {
	query := psql.Delete(
		dm.From("groups"),
		dm.Where(psql.Quote("id").EQ(psql.Arg(id))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}
