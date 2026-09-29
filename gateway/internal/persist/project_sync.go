// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package persist

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"
)

// ProjectSyncRow is one engineering-state snapshot from a Slave project.
type ProjectSyncRow struct {
	SlaveID     string          `json:"slaveId"`
	RepoID      string          `json:"repoId"`
	SyncedAt    string          `json:"syncedAt"`
	SummaryJSON json.RawMessage `json:"payload"`
}

// ProjectSyncStore UPSERT/GET for M08 project sync.
type ProjectSyncStore interface {
	UpsertProjectSync(row ProjectSyncRow) error
	GetProjectSync(slaveID, repoID string) (*ProjectSyncRow, error)
}

// AsProjectSyncStore returns SQLite-backed store when available; else memory.
func AsProjectSyncStore(b Backend) ProjectSyncStore {
	if s, ok := b.(*SQLiteStore); ok {
		return s
	}
	return NewMemoryProjectSync()
}

// MemoryProjectSync is process-local (tests / no SQLite).
type MemoryProjectSync struct {
	mu   sync.RWMutex
	rows map[string]ProjectSyncRow // slaveId\0repoId
}

func NewMemoryProjectSync() *MemoryProjectSync {
	return &MemoryProjectSync{rows: map[string]ProjectSyncRow{}}
}

func syncKey(slaveID, repoID string) string {
	return slaveID + "\x00" + repoID
}

func (m *MemoryProjectSync) UpsertProjectSync(row ProjectSyncRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if row.SyncedAt == "" {
		row.SyncedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	cp := row
	if row.SummaryJSON != nil {
		cp.SummaryJSON = append(json.RawMessage(nil), row.SummaryJSON...)
	}
	m.rows[syncKey(row.SlaveID, row.RepoID)] = cp
	return nil
}

func (m *MemoryProjectSync) GetProjectSync(slaveID, repoID string) (*ProjectSyncRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	row, ok := m.rows[syncKey(slaveID, repoID)]
	if !ok {
		return nil, nil
	}
	cp := row
	if row.SummaryJSON != nil {
		cp.SummaryJSON = append(json.RawMessage(nil), row.SummaryJSON...)
	}
	return &cp, nil
}

// UpsertProjectSync implements ProjectSyncStore for SQLite.
func (s *SQLiteStore) UpsertProjectSync(row ProjectSyncRow) error {
	if row.SyncedAt == "" {
		row.SyncedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	body := string(row.SummaryJSON)
	if body == "" {
		body = "{}"
	}
	_, err := s.db.Exec(
		`INSERT INTO project_sync(slave_id, repo_id, synced_at, summary_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(slave_id, repo_id) DO UPDATE SET
		   synced_at = excluded.synced_at,
		   summary_json = excluded.summary_json`,
		row.SlaveID, row.RepoID, row.SyncedAt, body,
	)
	return err
}

// GetProjectSync implements ProjectSyncStore for SQLite.
func (s *SQLiteStore) GetProjectSync(slaveID, repoID string) (*ProjectSyncRow, error) {
	var syncedAt, body string
	err := s.db.QueryRow(
		`SELECT synced_at, summary_json FROM project_sync WHERE slave_id = ? AND repo_id = ?`,
		slaveID, repoID,
	).Scan(&syncedAt, &body)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ProjectSyncRow{
		SlaveID:     slaveID,
		RepoID:      repoID,
		SyncedAt:    syncedAt,
		SummaryJSON: json.RawMessage(body),
	}, nil
}
