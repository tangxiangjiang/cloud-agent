// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package persist

import (
	"database/sql"
	"sync"
	"time"
)

// ModelEntry is one picker / catalog row.
type ModelEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ModelCatalogStore persists App picker selection + Slave-reported available models.
type ModelCatalogStore interface {
	ListSelected() ([]ModelEntry, error)
	ReplaceSelected(entries []ModelEntry, defaultID string) error
	AddSelected(e ModelEntry) error
	RemoveSelected(id string) error
	GetDefault() (string, error)
	SetDefault(id string) error
	ListAvailable() ([]ModelEntry, error)
	ReplaceAvailable(entries []ModelEntry, slaveID string) error
}

// AsModelCatalogStore returns SQLite-backed store when available; else memory.
func AsModelCatalogStore(b Backend) ModelCatalogStore {
	if s, ok := b.(*SQLiteStore); ok {
		return s
	}
	return NewMemoryModelCatalog()
}

// MemoryModelCatalog is process-local.
type MemoryModelCatalog struct {
	mu        sync.RWMutex
	selected  []ModelEntry
	available []ModelEntry
	defaultID string
}

func NewMemoryModelCatalog() *MemoryModelCatalog {
	return &MemoryModelCatalog{defaultID: "auto"}
}

func (m *MemoryModelCatalog) ListSelected() ([]ModelEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]ModelEntry(nil), m.selected...), nil
}

func (m *MemoryModelCatalog) ReplaceSelected(entries []ModelEntry, defaultID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selected = append([]ModelEntry(nil), entries...)
	if defaultID != "" {
		m.defaultID = defaultID
	}
	return nil
}

func (m *MemoryModelCatalog) AddSelected(e ModelEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.selected {
		if x.ID == e.ID {
			return nil
		}
	}
	m.selected = append(m.selected, e)
	return nil
}

func (m *MemoryModelCatalog) RemoveSelected(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.selected[:0]
	for _, x := range m.selected {
		if x.ID != id {
			out = append(out, x)
		}
	}
	m.selected = out
	if m.defaultID == id {
		m.defaultID = "auto"
	}
	return nil
}

func (m *MemoryModelCatalog) GetDefault() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaultID == "" {
		return "auto", nil
	}
	return m.defaultID, nil
}

func (m *MemoryModelCatalog) SetDefault(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultID = id
	return nil
}

func (m *MemoryModelCatalog) ListAvailable() ([]ModelEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]ModelEntry(nil), m.available...), nil
}

func (m *MemoryModelCatalog) ReplaceAvailable(entries []ModelEntry, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.available = append([]ModelEntry(nil), entries...)
	return nil
}

// --- SQLite ---

func (s *SQLiteStore) ListSelected() ([]ModelEntry, error) {
	rows, err := s.db.Query(`SELECT id, label FROM model_selected ORDER BY pos ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelEntry
	for rows.Next() {
		var e ModelEntry
		if err := rows.Scan(&e.ID, &e.Label); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ReplaceSelected(entries []ModelEntry, defaultID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM model_selected`); err != nil {
		return err
	}
	for i, e := range entries {
		if _, err := tx.Exec(
			`INSERT INTO model_selected(id, label, pos) VALUES(?,?,?)`,
			e.ID, e.Label, i,
		); err != nil {
			return err
		}
	}
	if defaultID != "" {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO meta(key, value) VALUES('models_default', ?)`,
			defaultID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) AddSelected(e ModelEntry) error {
	var max sql.NullInt64
	_ = s.db.QueryRow(`SELECT MAX(pos) FROM model_selected`).Scan(&max)
	pos := 0
	if max.Valid {
		pos = int(max.Int64) + 1
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO model_selected(id, label, pos) VALUES(?,?,?)`,
		e.ID, e.Label, pos,
	)
	if err != nil {
		return err
	}
	// Update label if already present
	_, err = s.db.Exec(`UPDATE model_selected SET label=? WHERE id=?`, e.Label, e.ID)
	return err
}

func (s *SQLiteStore) RemoveSelected(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM model_selected WHERE id=?`, id); err != nil {
		return err
	}
	var def string
	_ = tx.QueryRow(`SELECT value FROM meta WHERE key='models_default'`).Scan(&def)
	if def == id {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO meta(key, value) VALUES('models_default', 'auto')`,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) GetDefault() (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key='models_default'`).Scan(&v)
	if err == sql.ErrNoRows {
		return "auto", nil
	}
	if err != nil {
		return "auto", err
	}
	if v == "" {
		return "auto", nil
	}
	return v, nil
}

func (s *SQLiteStore) SetDefault(id string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO meta(key, value) VALUES('models_default', ?)`,
		id,
	)
	return err
}

func (s *SQLiteStore) ListAvailable() ([]ModelEntry, error) {
	rows, err := s.db.Query(`SELECT id, label FROM model_available ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModelEntry
	for rows.Next() {
		var e ModelEntry
		if err := rows.Scan(&e.ID, &e.Label); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ReplaceAvailable(entries []ModelEntry, slaveID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM model_available`); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, e := range entries {
		if _, err := tx.Exec(
			`INSERT INTO model_available(id, label, source, updated_at) VALUES(?,?,?,?)`,
			e.ID, e.Label, slaveID, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}
