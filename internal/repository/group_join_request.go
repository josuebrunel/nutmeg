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
)

type JoinRequestInfo struct {
	ID        string    `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	CreatedAt time.Time `db:"created_at"`
}

func (r *Repository) CreateJoinRequest(ctx context.Context, groupID, userID, name, email string) error {
	query := psql.Insert(
		im.Into("group_join_requests", "group_id", "user_id", "name", "email"),
		im.Values(psql.Arg(groupID, userID, name, email)),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) GetPendingJoinRequest(ctx context.Context, groupID, userID string) (*model.JoinRequest, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "user_id", "name", "email", "status", "created_at", "updated_at"),
		sm.From("group_join_requests"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(psql.Quote("user_id").EQ(psql.Arg(userID))),
		sm.Where(psql.Quote("status").EQ(psql.Arg("pending"))),
	)
	return bob.One(ctx, r.db, query, scan.StructMapper[*model.JoinRequest]())
}

func (r *Repository) GetJoinRequest(ctx context.Context, groupID, requestID string) (*model.JoinRequest, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "user_id", "name", "email", "status", "created_at", "updated_at"),
		sm.From("group_join_requests"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(psql.Quote("id").EQ(psql.Arg(requestID))),
	)
	return bob.One(ctx, r.db, query, scan.StructMapper[*model.JoinRequest]())
}

func (r *Repository) ListPendingJoinRequests(ctx context.Context, groupID string) ([]JoinRequestInfo, error) {
	query := psql.Select(
		sm.Columns("id", "name", "email", "created_at"),
		sm.From("group_join_requests"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(psql.Quote("status").EQ(psql.Arg("pending"))),
		sm.OrderBy("created_at"),
	)
	return bob.All[JoinRequestInfo](ctx, r.db, query, scan.StructMapper[JoinRequestInfo]())
}

// PendingJoinGroupIDs returns the ids of every group where userID currently
// has a pending join request — used by group discovery to show "request
// pending" instead of a join button on groups the viewer already applied to.
func (r *Repository) PendingJoinGroupIDs(ctx context.Context, userID string) ([]string, error) {
	query := psql.Select(
		sm.Columns(psql.Raw("group_id")),
		sm.From("group_join_requests"),
		sm.Where(psql.Quote("user_id").EQ(psql.Arg(userID))),
		sm.Where(psql.Quote("status").EQ(psql.Arg("pending"))),
		sm.Distinct(),
	)
	return bob.All[string](ctx, r.db, query, scan.SingleColumnMapper[string])
}

// DeletePendingJoinRequestsByUser cancels every pending join request userID
// has open, across all groups — used when the user's account is being
// deleted, so an admin never lands on a request there's no one left to
// approve.
func (r *Repository) DeletePendingJoinRequestsByUser(ctx context.Context, userID string) error {
	query := psql.Delete(
		dm.From("group_join_requests"),
		dm.Where(psql.Quote("user_id").EQ(psql.Arg(userID))),
		dm.Where(psql.Quote("status").EQ(psql.Arg("pending"))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) UpdateJoinRequestStatus(ctx context.Context, requestID, status string) error {
	query := psql.Update(
		um.Table("group_join_requests"),
		um.SetCol("status").ToArg(status),
		um.SetCol("updated_at").ToArg(psql.Raw("NOW()")),
		um.Where(psql.Quote("id").EQ(psql.Arg(requestID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}
