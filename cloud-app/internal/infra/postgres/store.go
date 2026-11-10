package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"kicau.cloud/control/db/queries"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

type executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type txKey struct{}
type Store struct{ Pool *pgxpool.Pool }

func (s *Store) executor(ctx context.Context) (executor, error) {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx, nil
	}
	if s.Pool == nil {
		return nil, apperror.ErrUnavailable
	}
	return s.Pool, nil
}
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrNotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505", "23503", "40001", "40P01":
			return apperror.ErrConflict
		case "23514", "23502", "22P02", "22003", "22007", "22008":
			return apperror.ErrInvalid
		case "42P01", "42703", "42501":
			return apperror.ErrUnavailable
		}
		if strings.HasPrefix(p.Code, "22") {
			return apperror.ErrInvalid
		}
	}
	return err
}
func (s *Store) Do(ctx context.Context, f func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return f(ctx)
	}
	if s.Pool == nil {
		return apperror.ErrUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return classify(err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if err = f(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return classify(tx.Commit(ctx))
}
func decode(b []byte) (model.Record, error) {
	var r model.Record
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	err := d.Decode(&r)
	return r, err
}
func (s *Store) One(ctx context.Context, name string, args ...any) (model.Record, error) {
	e, err := s.executor(ctx)
	if err != nil {
		return nil, err
	}
	var b []byte
	if err = e.QueryRow(ctx, queries.Get(name), args...).Scan(&b); err != nil {
		return nil, classify(err)
	}
	return decode(b)
}
func (s *Store) Many(ctx context.Context, name string, args ...any) ([]model.Record, error) {
	e, err := s.executor(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := e.Query(ctx, queries.Get(name), args...)
	if err != nil {
		return nil, classify(err)
	}
	defer rows.Close()
	out := []model.Record{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		r, err := decode(b)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, classify(rows.Err())
}
func (s *Store) Exec(ctx context.Context, name string, args ...any) error {
	e, err := s.executor(ctx)
	if err != nil {
		return err
	}
	_, err = e.Exec(ctx, queries.Get(name), args...)
	return classify(err)
}
func (s *Store) Ready(ctx context.Context) error { _, err := s.One(ctx, "SchemaReady"); return err }

func (s *Store) Ping(ctx context.Context) error {
	if s.Pool == nil {
		return apperror.ErrUnavailable
	}
	if e := s.Pool.Ping(ctx); e != nil {
		return e
	}
	return s.Ready(ctx)
}
