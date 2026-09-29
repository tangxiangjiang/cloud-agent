// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Package persist loads/saves Gateway durable state (workflows, diffs, auth tokens)
// so review progress survives process restarts.
package persist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const SchemaVersion = 1

// Snapshot is the on-disk format under GATEWAY_STATE_FILE.
type Snapshot struct {
	SchemaVersion int               `json:"schemaVersion"`
	SavedAt       string            `json:"savedAt"`
	Tokens        map[string]string `json:"tokens,omitempty"` // token -> expiresAt RFC3339
	Workflows     json.RawMessage   `json:"workflows,omitempty"`
	Diffs         json.RawMessage   `json:"diffs,omitempty"`
}

// FileStore atomically writes snapshots to path.
type FileStore struct {
	path string

	mu       sync.Mutex
	pending  *Snapshot
	timer    *time.Timer
	debounce time.Duration
}

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path:     path,
		debounce: 250 * time.Millisecond,
	}
}

func (f *FileStore) Path() string { return f.path }

func (f *FileStore) Load() (*Snapshot, error) {
	b, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// ScheduleSave coalesces rapid mutations into one disk write.
func (f *FileStore) ScheduleSave(snap *Snapshot) {
	if f == nil || snap == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = snap
	if f.timer != nil {
		f.timer.Stop()
	}
	f.timer = time.AfterFunc(f.debounce, func() {
		f.mu.Lock()
		p := f.pending
		f.pending = nil
		f.timer = nil
		f.mu.Unlock()
		if p != nil {
			_ = f.SaveNow(p)
		}
	})
}

func (f *FileStore) SaveNow(snap *Snapshot) error {
	if f == nil || snap == nil {
		return nil
	}
	snap.SchemaVersion = SchemaVersion
	if snap.SavedAt == "" {
		snap.SavedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

// Flush writes any pending snapshot immediately (call on shutdown if needed).
func (f *FileStore) Flush() {
	if f == nil {
		return
	}
	f.mu.Lock()
	if f.timer != nil {
		f.timer.Stop()
		f.timer = nil
	}
	p := f.pending
	f.pending = nil
	f.mu.Unlock()
	if p != nil {
		_ = f.SaveNow(p)
	}
}
