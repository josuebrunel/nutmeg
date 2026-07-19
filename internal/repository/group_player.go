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
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Phone     *string   `db:"phone"`
	Email     *string   `db:"email"`
	Role      string    `db:"role"`
	JoinedAt  time.Time `db:"joined_at"`
}

func (r *Repository) AddMember(ctx context.Context, groupID, name string, phone, email *string, role string) error {
	query := psql.Insert(
		im.Into("group_players", "group_id", "name", "phone", "email", "role"),
		im.Values(psql.Arg(groupID, name, phone, email, role)),
		im.OnConflict("group_id", "name").DoUpdate(
			im.SetCol("role").ToArg(role),
		),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) RemoveMember(ctx context.Context, groupID, memberID string) error {
	query := psql.Delete(
		dm.From("group_players"),
		dm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		dm.Where(psql.Quote("id").EQ(psql.Arg(memberID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) ListMembers(ctx context.Context, groupID string) ([]MemberInfo, error) {
	query := psql.Select(
		sm.Columns("id", "name", "phone", "email", "role", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.OrderBy("role"),
		sm.OrderBy("name"),
	)
	return bob.All[MemberInfo](ctx, r.db, query, scan.StructMapper[MemberInfo]())
}

func (r *Repository) GetMember(ctx context.Context, groupID, memberID string) (*model.GroupPlayer, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "name", "phone", "email", "role", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(psql.Quote("id").EQ(psql.Arg(memberID))),
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
