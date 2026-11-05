package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"kicau.cloud/control/internal/exchange/model"
	_ "modernc.org/sqlite"
)

type SQLite struct{ db *sql.DB }

func Open(path string) (*SQLite, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{`PRAGMA journal_mode=WAL`, `PRAGMA synchronous=FULL`, `PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS changes(seq INTEGER PRIMARY KEY AUTOINCREMENT,engine TEXT NOT NULL,id TEXT NOT NULL,body BLOB NOT NULL,UNIQUE(engine,id))`,
		`CREATE TABLE IF NOT EXISTS commands(seq INTEGER PRIMARY KEY AUTOINCREMENT,engine TEXT NOT NULL,id TEXT NOT NULL,body BLOB NOT NULL,result BLOB,UNIQUE(engine,id))`,
		`CREATE TABLE IF NOT EXISTS packages(engine TEXT NOT NULL,id TEXT NOT NULL,version INTEGER NOT NULL,body BLOB NOT NULL,PRIMARY KEY(engine,id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := os.Chmod(path, 0600); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) SavePackage(ctx context.Context, p model.Package, body []byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	var previous []byte
	err = tx.QueryRowContext(ctx, `SELECT version,body FROM packages WHERE engine=? AND id=?`, p.EngineID, p.ID).Scan(&version, &previous)
	if err == nil {
		if p.Version == version && string(body) == string(previous) {
			return tx.Commit()
		}
		if p.Version <= version {
			return model.ErrConflict
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO packages(engine,id,version,body) VALUES(?,?,?,?) ON CONFLICT(engine,id) DO UPDATE SET version=excluded.version,body=excluded.body`, p.EngineID, p.ID, p.Version, body); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) Package(ctx context.Context, engine, id string) ([]byte, error) {
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM packages WHERE engine=? AND id=?`, engine, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return body, err
}
func (s *SQLite) Close() error { return s.db.Close() }
func (s *SQLite) Receive(ctx context.Context, engine string, changes []model.Change) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, change := range changes {
		body, err := json.Marshal(change)
		if err != nil {
			return err
		}
		var previous []byte
		err = tx.QueryRowContext(ctx, `SELECT body FROM changes WHERE engine=? AND id=?`, engine, change.ID).Scan(&previous)
		if err == nil {
			if string(previous) != string(body) {
				return model.ErrConflict
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO changes(engine,id,body) VALUES(?,?,?)`, engine, change.ID, body); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) Enqueue(ctx context.Context, command model.Command) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	body, err := json.Marshal(command)
	if err != nil {
		return err
	}
	var previous []byte
	err = tx.QueryRowContext(ctx, `SELECT body FROM commands WHERE engine=? AND id=?`, command.EngineID, command.ID).Scan(&previous)
	if err == nil {
		if string(previous) != string(body) {
			return model.ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO commands(engine,id,body) VALUES(?,?,?)`, command.EngineID, command.ID, body)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) Pending(ctx context.Context, engine string) ([]model.Command, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM commands WHERE engine=? AND result IS NULL ORDER BY seq LIMIT 100`, engine)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Command{}
	for rows.Next() {
		var data []byte
		var command model.Command
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &command); err != nil {
			return nil, err
		}
		out = append(out, command)
	}
	return out, rows.Err()
}

func (s *SQLite) Command(ctx context.Context, engine, id string) (model.Command, *model.Result, error) {
	var command model.Command
	var body, result []byte
	err := s.db.QueryRowContext(ctx, `SELECT body,result FROM commands WHERE engine=? AND id=?`, engine, id).Scan(&body, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return command, nil, model.ErrNotFound
	}
	if err != nil {
		return command, nil, err
	}
	if err := json.Unmarshal(body, &command); err != nil {
		return command, nil, err
	}
	if result == nil {
		return command, nil, nil
	}
	var value model.Result
	err = json.Unmarshal(result, &value)
	return command, &value, err
}
func (s *SQLite) Results(ctx context.Context, engine string, results []model.Result) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, result := range results {
		var body []byte
		err := tx.QueryRowContext(ctx, `SELECT result FROM commands WHERE engine=? AND id=?`, engine, result.ID).Scan(&body)
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrInvalid
		}
		if err != nil {
			return err
		}
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if body != nil {
			if string(body) != string(data) {
				return model.ErrConflict
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE commands SET result=? WHERE engine=? AND id=?`, data, engine, result.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
