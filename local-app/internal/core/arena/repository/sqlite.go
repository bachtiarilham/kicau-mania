// Package repository persists arena aggregates, sessions, and the transactional outbox.
// Business validation is supplied by UseCase; this package only handles storage.
package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
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
	fail := func(err error) (*SQLite, error) { db.Close(); return nil, err }
	for _, statement := range []string{
		`PRAGMA journal_mode=WAL`, `PRAGMA synchronous=FULL`, `PRAGMA foreign_keys=ON`, `PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS events(id TEXT PRIMARY KEY, body BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS revisions(seq INTEGER PRIMARY KEY AUTOINCREMENT,event_id TEXT NOT NULL,round_id TEXT NOT NULL,operation_id TEXT NOT NULL,body BLOB NOT NULL,UNIQUE(event_id,operation_id))`,
		`CREATE INDEX IF NOT EXISTS revisions_round ON revisions(event_id,round_id,seq)`,
		`CREATE INDEX IF NOT EXISTS revisions_judge ON revisions(event_id,json_extract(CAST(body AS TEXT),'$.judge_id'),seq)`,
		`CREATE TABLE IF NOT EXISTS sessions(token_hash TEXT PRIMARY KEY, event_id TEXT NOT NULL, judge_id TEXT NOT NULL, version INTEGER NOT NULL, expires_at INTEGER NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS sessions_expiry ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS outbox(seq INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT UNIQUE NOT NULL, body BLOB NOT NULL, acknowledged_at INTEGER)`,
		`CREATE INDEX IF NOT EXISTS outbox_pending ON outbox(acknowledged_at,seq)`,
		`CREATE TABLE IF NOT EXISTS commands(seq INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT UNIQUE NOT NULL,body BLOB NOT NULL,result BLOB,acknowledged INTEGER NOT NULL DEFAULT 0)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return fail(err)
		}
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fail(err)
	}
	store := &SQLite{db: db}
	if err := store.migrateAggregateRevisions(context.Background()); err != nil {
		return fail(err)
	}
	return store, nil
}

func (s *SQLite) Command(ctx context.Context, id string) (exchange.Command, *exchange.CommandResult, error) {
	var body, result []byte
	var command exchange.Command
	err := s.db.QueryRowContext(ctx, `SELECT body,result FROM commands WHERE id=?`, id).Scan(&body, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return command, nil, apperror.ErrNotFound
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
	var value exchange.CommandResult
	err = json.Unmarshal(result, &value)
	return command, &value, err
}
func (s *SQLite) SaveCommand(ctx context.Context, command exchange.Command) error {
	body, err := json.Marshal(command)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO commands(id,body) VALUES(?,?) ON CONFLICT(id) DO NOTHING`, command.ID, body)
	return err
}
func (s *SQLite) CompleteCommand(ctx context.Context, value exchange.CommandResult) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE commands SET result=? WHERE id=? AND result IS NULL`, body, value.ID)
	return err
}
func (s *SQLite) CommandResults(ctx context.Context) ([]exchange.CommandResult, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT result FROM commands WHERE result IS NOT NULL AND acknowledged=0 ORDER BY seq LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exchange.CommandResult{}
	for rows.Next() {
		var data []byte
		var value exchange.CommandResult
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}
func (s *SQLite) AcknowledgeCommands(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE commands SET acknowledged=1 WHERE id=? AND result IS NOT NULL`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) Close() error { return s.db.Close() }
func (s *SQLite) Event(ctx context.Context, id string) (model.Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	e, _, err := loadEvent(ctx, tx, id, "", "", "")
	if err != nil {
		return e, err
	}
	if e.Bundle.ID == "" {
		return e, apperror.ErrNotFound
	}
	return e, tx.Commit()
}
func (s *SQLite) Metadata(ctx context.Context, id string) (model.Event, error) {
	var body []byte
	err := s.db.QueryRowContext(ctx, `SELECT body FROM events WHERE id=?`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Event{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.Event{}, err
	}
	var e model.Event
	err = json.Unmarshal(body, &e)
	return e, err
}
func (s *SQLite) JudgeEvent(ctx context.Context, id, judge string) (model.Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	e, _, err := loadEvent(ctx, tx, id, "", "", judge)
	if err != nil {
		return e, err
	}
	if e.Bundle.ID == "" {
		return e, apperror.ErrNotFound
	}
	return e, tx.Commit()
}
func (s *SQLite) Events(ctx context.Context) ([]model.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM events ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []model.Event{}
	for rows.Next() {
		var data []byte
		var event model.Event
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// Update commits the aggregate and its outbound changes together, or neither.
// The callback must not call storage again. A single connection serializes writers.
func (s *SQLite) Update(ctx context.Context, id string, apply func(*model.Event) ([]exchange.Change, error)) error {
	return s.update(ctx, id, "", "", apply)
}

// UpdateScores reads only the current round and any duplicate operation being
// retried. A growing event archive does not get rewritten on every score submit.
func (s *SQLite) UpdateScores(ctx context.Context, id, round, operation string, apply func(*model.Event) ([]exchange.Change, error)) error {
	return s.update(ctx, id, round, operation, apply)
}
func (s *SQLite) update(ctx context.Context, id, round, operation string, apply func(*model.Event) ([]exchange.Change, error)) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	event, previous, err := loadEvent(ctx, tx, id, round, operation, "")
	if err != nil {
		return err
	}
	before := len(event.Revisions)
	changes, err := apply(&event)
	if err != nil {
		return err
	}
	if event.Bundle.ID != id {
		return fmt.Errorf("aggregate identity mismatch")
	}
	if len(event.Revisions) < before {
		return fmt.Errorf("score history is append-only")
	}
	for _, revision := range event.Revisions[before:] {
		body, err := json.Marshal(revision)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO revisions(event_id,round_id,operation_id,body) VALUES(?,?,?,?)`, id, revision.RoundID, revision.OperationID, body); err != nil {
			return err
		}
	}
	event.Revisions = nil
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if !bytes.Equal(previous, data) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO events(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body`, id, data); err != nil {
			return err
		}
	}
	for _, change := range changes {
		encoded, err := json.Marshal(change)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO outbox(id,body) VALUES(?,?)`, change.ID, encoded); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func loadEvent(ctx context.Context, tx *sql.Tx, id, round, operation, judge string) (model.Event, []byte, error) {
	var event model.Event
	var data []byte
	err := tx.QueryRowContext(ctx, `SELECT body FROM events WHERE id=?`, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return event, nil, nil
	}
	if err != nil {
		return event, nil, err
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return event, nil, err
	}
	query := `SELECT body FROM revisions WHERE event_id=?`
	args := []any{id}
	if round != "" {
		query += ` AND (round_id=? OR operation_id=?)`
		args = append(args, round, operation)
	}
	if judge != "" {
		query += ` AND json_extract(CAST(body AS TEXT),'$.judge_id')=?`
		args = append(args, judge)
	}
	rows, err := tx.QueryContext(ctx, query+` ORDER BY seq`, args...)
	if err != nil {
		return event, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var body []byte
		var revision model.Revision
		if err := rows.Scan(&body); err != nil {
			return event, nil, err
		}
		if err := json.Unmarshal(body, &revision); err != nil {
			return event, nil, err
		}
		event.Revisions = append(event.Revisions, revision)
	}
	return event, data, rows.Err()
}

func (s *SQLite) migrateAggregateRevisions(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT body FROM events`)
	if err != nil {
		return err
	}
	legacy := []model.Event{}
	for rows.Next() {
		var body []byte
		var event model.Event
		if err := rows.Scan(&body); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(body, &event); err != nil {
			rows.Close()
			return err
		}
		if len(event.Revisions) > 0 {
			legacy = append(legacy, event)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, event := range legacy {
		for _, revision := range event.Revisions {
			body, err := json.Marshal(revision)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO revisions(event_id,round_id,operation_id,body) VALUES(?,?,?,?)`, event.Bundle.ID, revision.RoundID, revision.OperationID, body); err != nil {
				return err
			}
		}
		event.Revisions = nil
		body, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE events SET body=? WHERE id=?`, body, event.Bundle.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) SaveSession(ctx context.Context, v model.Session) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=?`, time.Now().Unix()); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sessions VALUES(?,?,?,?,?)`, v.TokenHash, v.EventID, v.JudgeID, v.Version, v.ExpiresAt.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *SQLite) Session(ctx context.Context, hash string) (model.Session, error) {
	var v model.Session
	var expires int64
	err := s.db.QueryRowContext(ctx, `SELECT token_hash,event_id,judge_id,version,expires_at FROM sessions WHERE token_hash=?`, hash).Scan(&v.TokenHash, &v.EventID, &v.JudgeID, &v.Version, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return v, apperror.ErrUnauthorized
	}
	v.ExpiresAt = time.Unix(expires, 0)
	return v, err
}
func (s *SQLite) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, hash)
	return err
}
func (s *SQLite) Pending(ctx context.Context, limit int) ([]exchange.Change, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT body FROM outbox WHERE acknowledged_at IS NULL ORDER BY seq LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []exchange.Change{}
	for rows.Next() {
		var data []byte
		var v exchange.Change
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *SQLite) Acknowledge(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox SET acknowledged_at=? WHERE id=? AND acknowledged_at IS NULL`, time.Now().Unix(), id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLite) PendingCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM outbox WHERE acknowledged_at IS NULL`).Scan(&n)
	return n, err
}

// Backup uses SQLite's consistent snapshot mechanism; copying a live WAL file
// independently is not a valid backup. Destination must be a new filename.
func (s *SQLite) Backup(ctx context.Context, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, destination)
	if err == nil {
		err = os.Chmod(destination, 0600)
	}
	return err
}
