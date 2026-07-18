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

	"nutmeg/internal/model"
)

type MemberInfo struct {
	UserID    string    `db:"user_id"`
	Email     string    `db:"email"`
	FirstName string    `db:"first_name"`
	LastName  string    `db:"last_name"`
	Role      string    `db:"role"`
	JoinedAt  time.Time `db:"joined_at"`
}

func (r *Repository) AddMember(ctx context.Context, groupID, userID, role string) error {
	query := psql.Insert(
		im.Into("group_players", "group_id", "user_id", "role"),
		im.Values(psql.Arg(groupID, userID, role)),
		im.OnConflict("group_id", "user_id").DoUpdate(
			im.SetCol("role").ToArg(role),
		),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) RemoveMember(ctx context.Context, groupID, userID string) error {
	query := psql.Delete(
		dm.From("group_players"),
		dm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		dm.Where(psql.Quote("user_id").EQ(psql.Arg(userID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) ListMembers(ctx context.Context, groupID string) ([]MemberInfo, error) {
	query := psql.Select(
		sm.Columns("gp.user_id", "u.email", "u.first_name", "u.last_name", "gp.role", "gp.joined_at"),
		sm.From("group_players gp"),
		sm.InnerJoin("ezauth_users u ON u.id = gp.user_id"),
		sm.Where(psql.Quote("gp", "group_id").EQ(psql.Arg(groupID))),
		sm.OrderBy("gp.role"),
		sm.OrderBy("u.first_name"),
		sm.OrderBy("u.last_name"),
	)
	return bob.All(ctx, r.db, query, scan.StructMapper[MemberInfo]())
}

func (r *Repository) GetMember(ctx context.Context, groupID, userID string) (*model.GroupPlayer, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "user_id", "role", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(psql.Quote("user_id").EQ(psql.Arg(userID))),
	)
	return bob.One(ctx, r.db, query, scan.StructMapper[*model.GroupPlayer]())
}

func (r *Repository) MemberCount(ctx context.Context, groupID string) (int, error) {
	query := psql.Select(
		sm.Columns(psql.Raw("COUNT(*)")),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
	)
	return bob.One(ctx, r.db, query, scan.SingleColumnMapper[int])
}
