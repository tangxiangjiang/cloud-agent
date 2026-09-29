// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package persist

import (
	"path/filepath"
	"strings"
)

// Backend is durable Gateway state (JSON file or SQLite).
type Backend interface {
	Path() string
	Load() (*Snapshot, error)
	ScheduleSave(snap *Snapshot)
	Flush()
}

// Open picks backend by path extension:
//   - .json → FileStore (legacy)
//   - .db / .sqlite / .sqlite3 / other → SQLite (preferred)
//
// When opening SQLite on an empty DB, imports sibling state.json once if present.
func Open(path string) (Backend, error) {
	path = strings.TrimSpace(path)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return NewFileStore(path), nil
	default:
		return OpenSQLite(path)
	}
}
