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

type MemberInfo struct {
	ID       string    `db:"id"`
	Name     string    `db:"name"`
	Phone    *string   `db:"phone"`
	Email    *string   `db:"email"`
	Role     string    `db:"role"`
	Position *string   `db:"position"`
	JoinedAt time.Time `db:"joined_at"`
}

// AddMember returns the member's id (the row's own id whether this insert
// or an ON CONFLICT update, so callers with a single new name can enqueue
// follow-up work like activity-feed generation without a second query).
func (r *Repository) AddMember(ctx context.Context, groupID, name string, phone, email, position *string, role string) (string, error) {
	query := psql.Insert(
		im.Into("group_players", "group_id", "name", "phone", "email", "position", "role"),
		im.Values(psql.Arg(groupID, name, phone, email, position, role)),
		im.OnConflict("group_id", "name").DoUpdate(
			im.SetCol("role").ToArg(role),
		),
		im.Returning("id"),
	)
	id, err := bob.One(ctx, r.db, query, scan.SingleColumnMapper[string])
	if err != nil {
		return "", err
	}
	if err := setSlugIfEmpty(ctx, r.db, "group_players", id, name); err != nil {
		return "", err
	}
	return id, nil
}

// ImportMember upserts a CSV-imported roster row: unlike AddMember (which
// only touches role on conflict, relied on by call sites that intentionally
// pass nil phone/email), this updates phone/email/position when the new
// value is non-null while preserving the existing value on a blank CSV
// cell, and never touches role — so re-importing a CSV can't demote an
// existing admin.
func (r *Repository) ImportMember(ctx context.Context, groupID, name string, phone, email, position *string) error {
	query := psql.RawQuery(`
		INSERT INTO group_players (group_id, name, phone, email, position, role)
		VALUES (?, ?, ?, ?, ?, 'member')
		ON CONFLICT (group_id, name) DO UPDATE SET
			phone = COALESCE(EXCLUDED.phone, group_players.phone),
			email = COALESCE(EXCLUDED.email, group_players.email),
			position = COALESCE(EXCLUDED.position, group_players.position)
		RETURNING id
	`, groupID, name, phone, email, position)
	id, err := bob.One(ctx, r.db, query, scan.SingleColumnMapper[string])
	if err != nil {
		return err
	}
	return setSlugIfEmpty(ctx, r.db, "group_players", id, name)
}

func (r *Repository) UpdateMember(ctx context.Context, groupID, memberID, name string, phone, email, position *string) error {
	query := psql.Update(
		um.Table("group_players"),
		um.SetCol("name").ToArg(name),
		um.SetCol("phone").ToArg(phone),
		um.SetCol("email").ToArg(email),
		um.SetCol("position").ToArg(position),
		um.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		um.Where(psql.Quote("id").EQ(psql.Arg(memberID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

func (r *Repository) UpdateMemberRole(ctx context.Context, groupID, memberID, role string) error {
	query := psql.Update(
		um.Table("group_players"),
		um.SetCol("role").ToArg(role),
		um.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		um.Where(psql.Quote("id").EQ(psql.Arg(memberID))),
	)
	_, err := bob.Exec(ctx, r.db, query)
	return err
}

// UpdateMemberPosition updates a single roster member's position — a
// lighter-weight sibling to UpdateMember for the roster row's inline
// position picker, which shouldn't require resubmitting name/phone/email
// just to change one field.
func (r *Repository) UpdateMemberPosition(ctx context.Context, groupID, memberID, position string) error {
	query := psql.Update(
		um.Table("group_players"),
		um.SetCol("position").ToArg(position),
		um.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		um.Where(psql.Quote("id").EQ(psql.Arg(memberID))),
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
		sm.Columns("id", "name", "phone", "email", "role", "position", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.OrderBy("role"),
		sm.OrderBy("name"),
	)
	return bob.All[MemberInfo](ctx, r.db, query, scan.StructMapper[MemberInfo]())
}

// GetMember looks up a player by id or, once slugged (see internal/slug),
// by its slug — groupID is always the group's real id here (callers must
// resolve a group slug via GetGroup first; slugs are only scoped within a
// group, not globally unique, so it can't be branched on independently).
func (r *Repository) GetMember(ctx context.Context, groupID, memberIDOrSlug string) (*model.GroupPlayer, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "name", "phone", "email", "role", "slug", "position", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(resolveByIDOrSlug(memberIDOrSlug)),
	)
	return bob.One(ctx, r.db, query, scan.StructMapper[*model.GroupPlayer]())
}

// GetMemberByID looks up a group_players row by id alone — safe because
// the id is a globally unique UUID, not scoped to a group. Used where only
// the player id is on hand (e.g. a background job's arguments) and looking
// up the owning group first would just be extra round trips.
func (r *Repository) GetMemberByID(ctx context.Context, memberID string) (*model.GroupPlayer, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "name", "phone", "email", "role", "position", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("id").EQ(psql.Arg(memberID))),
	)
	return bob.One(ctx, r.db, query, scan.StructMapper[*model.GroupPlayer]())
}

func (r *Repository) GetMemberByEmail(ctx context.Context, groupID, email string) (*model.GroupPlayer, error) {
	query := psql.Select(
		sm.Columns("id", "group_id", "name", "phone", "email", "role", "position", "joined_at"),
		sm.From("group_players"),
		sm.Where(psql.Quote("group_id").EQ(psql.Arg(groupID))),
		sm.Where(psql.Quote("email").EQ(psql.Arg(email))),
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
