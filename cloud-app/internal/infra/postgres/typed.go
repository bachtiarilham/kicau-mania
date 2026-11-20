package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// QueryOne executes a query and scans a single row into a typed struct using field/tag names.
func QueryOne[T any](ctx context.Context, s *Store, sql string, args ...any) (*T, error) {
	exec, err := s.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	item, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, classify(err)
	}
	return &item, nil
}

// QueryMany executes a query and scans multiple rows into a slice of typed structs.
func QueryMany[T any](ctx context.Context, s *Store, sql string, args ...any) ([]T, error) {
	exec, err := s.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := exec.Query(ctx, sql, args...)
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	items, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, classify(err)
	}
	return items, nil
}

// Exec executes a raw SQL statement within the active transaction or pool.
func (s *Store) ExecSQL(ctx context.Context, sql string, args ...any) error {
	exec, err := s.executor(ctx)
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, sql, args...)
	return classify(err)
}

// QueryRow exposes the underlying pgx.Row for custom manual scanning if needed.
func (s *Store) QueryRow(ctx context.Context, sql string, args ...any) (pgx.Row, error) {
	exec, err := s.executor(ctx)
	if err != nil {
		return nil, err
	}
	return exec.QueryRow(ctx, sql, args...), nil
}

// Classify classifies a PostgreSQL error into standard apperror.
func Classify(err error) error {
	return classify(err)
}
