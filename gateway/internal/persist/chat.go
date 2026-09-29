// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package persist

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// ChatSessionRow is one ChatSession + truncated message summaries (no tool payloads).
type ChatSessionRow struct {
	ID        string          `json:"id"`
	SlaveID   string          `json:"slaveId"`
	RepoID    string          `json:"repoId"`
	Mode      string          `json:"mode"`
	Model     string          `json:"model"`
	Status    string          `json:"status"`
	Messages  json.RawMessage `json:"messages"` // []Message JSON
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// ChatSessionStore persists project chat sessions (M09-P04).
type ChatSessionStore interface {
	UpsertChatSession(row ChatSessionRow) error
	GetChatSession(id string) (*ChatSessionRow, error)
	ListChatSessions(slaveID, repoID string) ([]ChatSessionRow, error)
	LoadAllChatSessions() ([]ChatSessionRow, error)
}

// AsChatSessionStore returns SQLite-backed store when available; else memory.
func AsChatSessionStore(b Backend) ChatSessionStore {
	if s, ok := b.(*SQLiteStore); ok {
		return s
	}
	return NewMemoryChatSessions()
}

// MemoryChatSessions is process-local (tests / no SQLite).
type MemoryChatSessions struct {
	mu   sync.RWMutex
	rows map[string]ChatSessionRow
}

func NewMemoryChatSessions() *MemoryChatSessions {
	return &MemoryChatSessions{rows: map[string]ChatSessionRow{}}
}

func (m *MemoryChatSessions) UpsertChatSession(row ChatSessionRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := row
	if row.Messages != nil {
		cp.Messages = append(json.RawMessage(nil), row.Messages...)
	} else {
		cp.Messages = json.RawMessage("[]")
	}
	m.rows[row.ID] = cp
	return nil
}

func (m *MemoryChatSessions) GetChatSession(id string) (*ChatSessionRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	row, ok := m.rows[id]
	if !ok {
		return nil, nil
	}
	return cloneChatRow(row), nil
}

func (m *MemoryChatSessions) ListChatSessions(slaveID, repoID string) ([]ChatSessionRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ChatSessionRow, 0)
	for _, row := range m.rows {
		if slaveID != "" && row.SlaveID != slaveID {
			continue
		}
		if repoID != "" && row.RepoID != repoID {
			continue
		}
		out = append(out, *cloneChatRow(row))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (m *MemoryChatSessions) LoadAllChatSessions() ([]ChatSessionRow, error) {
	return m.ListChatSessions("", "")
}

func cloneChatRow(row ChatSessionRow) *ChatSessionRow {
	cp := row
	if row.Messages != nil {
		cp.Messages = append(json.RawMessage(nil), row.Messages...)
	}
	return &cp
}

// UpsertChatSession implements ChatSessionStore for SQLite.
func (s *SQLiteStore) UpsertChatSession(row ChatSessionRow) error {
	body := string(row.Messages)
	if strings.TrimSpace(body) == "" {
		body = "[]"
	}
	_, err := s.db.Exec(
		`INSERT INTO chat_sessions(id, slave_id, repo_id, mode, model, status, messages_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   slave_id = excluded.slave_id,
		   repo_id = excluded.repo_id,
		   mode = excluded.mode,
		   model = excluded.model,
		   status = excluded.status,
		   messages_json = excluded.messages_json,
		   updated_at = excluded.updated_at`,
		row.ID, row.SlaveID, row.RepoID, row.Mode, row.Model, row.Status,
		body, row.CreatedAt, row.UpdatedAt,
	)
	return err
}

// GetChatSession implements ChatSessionStore for SQLite.
func (s *SQLiteStore) GetChatSession(id string) (*ChatSessionRow, error) {
	var slaveID, repoID, mode, model, status, body, createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT slave_id, repo_id, mode, model, status, messages_json, created_at, updated_at
		 FROM chat_sessions WHERE id = ?`,
		id,
	).Scan(&slaveID, &repoID, &mode, &model, &status, &body, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ChatSessionRow{
		ID:        id,
		SlaveID:   slaveID,
		RepoID:    repoID,
		Mode:      mode,
		Model:     model,
		Status:    status,
		Messages:  json.RawMessage(body),
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// ListChatSessions implements ChatSessionStore for SQLite.
func (s *SQLiteStore) ListChatSessions(slaveID, repoID string) ([]ChatSessionRow, error) {
	q := `SELECT id, slave_id, repo_id, mode, model, status, messages_json, created_at, updated_at
	      FROM chat_sessions WHERE 1=1`
	args := []any{}
	if slaveID != "" {
		q += ` AND slave_id = ?`
		args = append(args, slaveID)
	}
	if repoID != "" {
		q += ` AND repo_id = ?`
		args = append(args, repoID)
	}
	q += ` ORDER BY updated_at DESC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ChatSessionRow, 0)
	for rows.Next() {
		var row ChatSessionRow
		var body string
		if err := rows.Scan(
			&row.ID, &row.SlaveID, &row.RepoID, &row.Mode, &row.Model, &row.Status,
			&body, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		row.Messages = json.RawMessage(body)
		out = append(out, row)
	}
	return out, rows.Err()
}

// LoadAllChatSessions implements ChatSessionStore for SQLite.
func (s *SQLiteStore) LoadAllChatSessions() ([]ChatSessionRow, error) {
	return s.ListChatSessions("", "")
}
