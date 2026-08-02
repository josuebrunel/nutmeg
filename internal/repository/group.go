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

	"nutmeg/internal/model"
	"nutmeg/internal/slug"
)

func (r *Repository) CreateGroup(ctx context.Context, g *model.Group) error {
	query := psql.Insert(
		im.Into("groups", "name", "description", "created_by"),
		im.Values(psql.Arg(g.Name, g.Description, g.CreatedBy)),
		im.Returning("id", "created_at", "updated_at"),
	)
	// Scanned into a narrow struct matching exactly the RETURNING columns,
	// not the full model.Group — a `*g = *result` against a Group-shaped
	// result would silently zero out g.Name/g.Description/g.CreatedBy,
	// since those columns aren't in this RETURNING clause.
	type created struct {
		ID        string    `db:"id"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}
	result, err := bob.One(ctx, r.db, query, scan.StructMapper[created]())
	if err != nil {
		return err
	}
	g.ID, g.CreatedAt, g.UpdatedAt = result.ID, result.CreatedAt, result.UpdatedAt
	if err := setSlugIfEmpty(ctx, r.db, "groups", g.ID, g.Name); err != nil {
		return err
	}
	s := slug.New(g.Name, g.ID)
	g.Slug = &s
	return nil
}

// GetGroup looks up a group by id or, once slugged (see internal/slug),
// by its slug — both current UUID links and newer slug links resolve.
func (r *Repository) GetGroup(ctx context.Context, idOrSlug string) (*model.Group, error) {
	query := psql.Select(
		sm.Columns("id", "name", "description", "created_by", "slug", "created_at", "updated_at"),
		sm.From("groups"),
		sm.Where(resolveByIDOrSlug(idOrSlug)),
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

func (r *Repository) GetGroupsByIDs(ctx context.Context, ids []string) ([]*model.Group, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]bob.Expression, len(ids))
	for i, id := range ids {
		args[i] = psql.Arg(id)
	}
	query := psql.Select(
		sm.Columns("id", "name", "description", "created_by", "created_at", "updated_at"),
		sm.From("groups"),
		sm.Where(psql.Quote("id").In(args...)),
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
