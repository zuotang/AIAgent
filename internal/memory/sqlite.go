package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS user_memory (
  user_id TEXT PRIMARY KEY,
  profile_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);`)
	return err
}

// LoadProfile returns a JSON object (map) for flexibility.
func (s *Store) LoadProfile(ctx context.Context, userID string) (map[string]any, error) {
	var raw string
	err := s.db.QueryRowContext(ctx,
		`SELECT profile_json FROM user_memory WHERE user_id = ?`,
		userID,
	).Scan(&raw)

	if err == sql.ErrNoRows {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		// If corrupted, start fresh rather than crash
		return map[string]any{}, nil
	}
	return m, nil
}

func (s *Store) SaveProfile(ctx context.Context, userID string, profile map[string]any) error {
	b, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO user_memory(user_id, profile_json, updated_at)
VALUES(?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
  profile_json = excluded.profile_json,
  updated_at = excluded.updated_at
`, userID, string(b), time.Now().Unix())
	return err
}

func (s *Store) Close() error { return s.db.Close() }
