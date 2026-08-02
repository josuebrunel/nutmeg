package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/um"

	"nutmeg/internal/slug"
)

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// postgresUniqueViolation is error code 23505 — see
// https://www.postgresql.org/docs/current/errcodes-appendix.html.
const postgresUniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation
}

// setSlugIfEmpty backfills a row's slug column right after an insert or
// upsert, only when it doesn't already have one — a fresh row, or an
// upsert conflict against a row from before this column existed. An
// existing slug is never regenerated, so a group/player's public URL
// stays stable across renames.
//
// It tries the plain slugified name first (e.g. "chris") so URLs read as
// someone's actual nickname rather than a UUID fragment, and only falls
// back to internal/slug.New's id-suffixed form (e.g. "chris-6b7505a9") if
// that collides with another row's slug — group_players names are already
// unique per group at the DB level, but slugification is lossy ("Chris"
// and "Chris!" collide), and group names have no uniqueness constraint at
// all, so a fallback is required either way rather than assumed away.
func setSlugIfEmpty(ctx context.Context, db bob.DB, table, id, name string) error {
	if err := trySetSlug(ctx, db, table, id, slug.Slugify(name)); err != nil {
		if !isUniqueViolation(err) {
			return err
		}
		return trySetSlug(ctx, db, table, id, slug.New(name, id))
	}
	return nil
}

func trySetSlug(ctx context.Context, db bob.DB, table, id, s string) error {
	query := psql.Update(
		um.Table(table),
		um.SetCol("slug").ToArg(s),
		um.Where(psql.Quote("id").EQ(psql.Arg(id))),
		um.Where(psql.Quote("slug").IsNull()),
	)
	_, err := bob.Exec(ctx, db, query)
	return err
}

// resolveByIDOrSlug returns a WHERE condition matching idOrSlug against
// either the id column (only when it parses as a UUID — a raw slug string
// bound against a uuid-typed column would otherwise fail outright at the
// Postgres level) or the slug column.
func resolveByIDOrSlug(idOrSlug string) bob.Expression {
	cond := psql.Quote("slug").EQ(psql.Arg(idOrSlug))
	if isUUID(idOrSlug) {
		return psql.Or(psql.Quote("id").EQ(psql.Arg(idOrSlug)), cond)
	}
	return cond
}
