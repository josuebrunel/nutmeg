package repository

import "github.com/stephenafamo/bob"

type Repository struct {
	db bob.DB
}

func (r *Repository) DB() bob.DB {
	return r.db
}

func New(db bob.DB) *Repository {
	return &Repository{db: db}
}
