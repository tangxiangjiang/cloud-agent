// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package persist

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore persists Snapshot rows transactionally.
// Also reserves project_sync for future Slave→Gateway engineering-state sync.
type SQLiteStore struct {
	path string
	db   *sql.DB

	mu       sync.Mutex
	pending  *Snapshot
	timer    *time.Timer
	debounce time.Duration
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite + simple writer
	s := &SQLiteStore{
		path:     path,
		db:       db,
		debounce: 250 * time.Millisecond,
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.maybeImportLegacyJSON(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate from state.json: %w", err)
	}
	return s, nil
}

func (s *SQLiteStore) Path() string { return s.path }

func (s *SQLiteStore) Close() error {
	s.Flush()
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY NOT NULL,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS tokens (
  token TEXT PRIMARY KEY NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS workflows (
  id TEXT PRIMARY KEY NOT NULL,
  body TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS diffs (
  key TEXT PRIMARY KEY NOT NULL,
  body TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
-- Future: Slave "同步" pushes AI-summarized project/milestone progress here.
CREATE TABLE IF NOT EXISTS project_sync (
  slave_id TEXT NOT NULL,
  repo_id TEXT NOT NULL,
  synced_at TEXT NOT NULL,
  summary_json TEXT NOT NULL,
  PRIMARY KEY (slave_id, repo_id)
);
CREATE TABLE IF NOT EXISTS chat_sessions (
  id TEXT PRIMARY KEY NOT NULL,
  slave_id TEXT NOT NULL,
  repo_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  model TEXT NOT NULL,
  status TEXT NOT NULL,
  messages_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_sessions_repo
  ON chat_sessions(slave_id, repo_id, updated_at);
CREATE TABLE IF NOT EXISTS model_selected (
  id TEXT PRIMARY KEY NOT NULL,
  label TEXT NOT NULL,
  pos INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS model_available (
  id TEXT PRIMARY KEY NOT NULL,
  label TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO meta(key, value) VALUES ('schemaVersion', ?)`,
		fmt.Sprintf("%d", SchemaVersion),
	)
	return err
}

func (s *SQLiteStore) rowCount(table string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n)
	return n, err
}

func (s *SQLiteStore) maybeImportLegacyJSON() error {
	wfN, err := s.rowCount("workflows")
	if err != nil {
		return err
	}
	tokN, err := s.rowCount("tokens")
	if err != nil {
		return err
	}
	if wfN > 0 || tokN > 0 {
		return nil
	}
	legacy := filepath.Join(filepath.Dir(s.path), "state.json")
	b, err := os.ReadFile(legacy)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return err
	}
	if err := s.SaveNow(&snap); err != nil {
		return err
	}
	// Rename so we don't re-import after intentional wipe.
	_ = os.Rename(legacy, legacy+".migrated")
	return nil
}

func (s *SQLiteStore) Load() (*Snapshot, error) {
	snap := &Snapshot{
		SchemaVersion: SchemaVersion,
		Tokens:        map[string]string{},
	}

	rows, err := s.db.Query(`SELECT token, expires_at FROM tokens`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tok, exp string
		if err := rows.Scan(&tok, &exp); err != nil {
			_ = rows.Close()
			return nil, err
		}
		snap.Tokens[tok] = exp
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var runs []json.RawMessage
	wrows, err := s.db.Query(`SELECT body FROM workflows`)
	if err != nil {
		return nil, err
	}
	for wrows.Next() {
		var body string
		if err := wrows.Scan(&body); err != nil {
			_ = wrows.Close()
			return nil, err
		}
		runs = append(runs, json.RawMessage(body))
	}
	_ = wrows.Close()
	if err := wrows.Err(); err != nil {
		return nil, err
	}
	if runs == nil {
		runs = []json.RawMessage{}
	}
	wfJSON, err := json.Marshal(runs)
	if err != nil {
		return nil, err
	}
	snap.Workflows = wfJSON

	diffMap := map[string]json.RawMessage{}
	drows, err := s.db.Query(`SELECT key, body FROM diffs`)
	if err != nil {
		return nil, err
	}
	for drows.Next() {
		var k, body string
		if err := drows.Scan(&k, &body); err != nil {
			_ = drows.Close()
			return nil, err
		}
		diffMap[k] = json.RawMessage(body)
	}
	_ = drows.Close()
	if err := drows.Err(); err != nil {
		return nil, err
	}
	diffJSON, err := json.Marshal(diffMap)
	if err != nil {
		return nil, err
	}
	snap.Diffs = diffJSON

	var savedAt string
	_ = s.db.QueryRow(`SELECT value FROM meta WHERE key = 'savedAt'`).Scan(&savedAt)
	snap.SavedAt = savedAt

	// Treat completely empty DB as "no snapshot" so main logs "starting fresh".
	if len(snap.Tokens) == 0 && len(runs) == 0 && len(diffMap) == 0 && savedAt == "" {
		return nil, nil
	}
	return snap, nil
}

func (s *SQLiteStore) ScheduleSave(snap *Snapshot) {
	if s == nil || snap == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = snap
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.debounce, func() {
		s.mu.Lock()
		p := s.pending
		s.pending = nil
		s.timer = nil
		s.mu.Unlock()
		if p != nil {
			_ = s.SaveNow(p)
		}
	})
}

func (s *SQLiteStore) SaveNow(snap *Snapshot) error {
	if s == nil || snap == nil {
		return nil
	}
	snap.SchemaVersion = SchemaVersion
	if snap.SavedAt == "" {
		snap.SavedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	var runs []json.RawMessage
	if len(snap.Workflows) > 0 {
		if err := json.Unmarshal(snap.Workflows, &runs); err != nil {
			return fmt.Errorf("workflows json: %w", err)
		}
	}
	diffMap := map[string]json.RawMessage{}
	if len(snap.Diffs) > 0 {
		if err := json.Unmarshal(snap.Diffs, &diffMap); err != nil {
			return fmt.Errorf("diffs json: %w", err)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM tokens`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM workflows`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM diffs`); err != nil {
		return err
	}

	for tok, exp := range snap.Tokens {
		if strings.TrimSpace(tok) == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO tokens(token, expires_at) VALUES (?, ?)`,
			tok, exp,
		); err != nil {
			return err
		}
	}

	now := snap.SavedAt
	for _, raw := range runs {
		var meta struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &meta); err != nil || meta.ID == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO workflows(id, body, updated_at) VALUES (?, ?, ?)`,
			meta.ID, string(raw), now,
		); err != nil {
			return err
		}
	}

	for k, raw := range diffMap {
		if _, err := tx.Exec(
			`INSERT INTO diffs(key, body, updated_at) VALUES (?, ?, ?)`,
			k, string(raw), now,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(
		`INSERT OR REPLACE INTO meta(key, value) VALUES ('savedAt', ?), ('schemaVersion', ?)`,
		snap.SavedAt, fmt.Sprintf("%d", SchemaVersion),
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteStore) Flush() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	p := s.pending
	s.pending = nil
	s.mu.Unlock()
	if p != nil {
		_ = s.SaveNow(p)
	}
}
