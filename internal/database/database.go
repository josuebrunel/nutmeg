package database

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PoolConfig tunes the connection pool. A zero value falls back to the
// defaults Open used to hardcode (25 max open / 5 max idle / 5m lifetime).
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func Open(dsn string, pool PoolConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	maxOpenConns := pool.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 25
	}
	maxIdleConns := pool.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 5
	}
	connMaxLifetime := pool.ConnMaxLifetime
	if connMaxLifetime == 0 {
		connMaxLifetime = 5 * time.Minute
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
