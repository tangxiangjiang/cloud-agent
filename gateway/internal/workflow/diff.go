// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

import (
	"path"
	"strings"
)

// NodeDiff aligns with contracts/schemas/node-diff.schema.json (read-only for App).
type NodeDiff struct {
	WorkflowID string         `json:"workflowId"`
	NodeID     string         `json:"nodeId"`
	Baseline   *string        `json:"baseline"`
	Files      []NodeDiffFile `json:"files"`
}

type NodeDiffFile struct {
	Path        string         `json:"path"`
	Status      string         `json:"status,omitempty"`
	Additions   *int           `json:"additions,omitempty"`
	Deletions   *int           `json:"deletions,omitempty"`
	UnifiedDiff string         `json:"unifiedDiff,omitempty"`
	Hunks       []NodeDiffHunk `json:"hunks,omitempty"`
}

type NodeDiffHunk struct {
	Header string   `json:"header"`
	Lines  []string `json:"lines"`
}

func diffKey(workflowID, nodeID string) string {
	return workflowID + "\x00" + nodeID
}

func isProgressPath(p string) bool {
	p = strings.ReplaceAll(p, "\\", "/")
	return strings.HasSuffix(strings.ToLower(p), "/progress.md") ||
		strings.EqualFold(path.Base(p), "progress.md")
}

func isCodePath(p string) bool {
	p = strings.ReplaceAll(p, "\\", "/")
	base := path.Base(p)
	switch strings.ToLower(base) {
	case "cargo.toml", "go.mod", "go.sum", "package.json", "cmakelists.txt", "makefile", "meson.build":
		return true
	}
	ext := strings.ToLower(path.Ext(p))
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".go", ".rs", ".py", ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp",
		".ny", ".nex", ".ni", ".nu",
		".swift", ".kt", ".kts", ".java", ".cs", ".rb", ".php", ".zig", ".mm", ".m":
		return true
	}
	return false
}

// DiffIsSubstantive reports whether a stored node diff is enough to approve.
// Empty or progress.md-only diffs are rubber stamps and must be rejected.
func DiffIsSubstantive(files []NodeDiffFile) (ok bool, reason string) {
	if len(files) == 0 {
		return false, "empty diff (no changes vs baseline)"
	}
	nonProgress := 0
	hasCode := false
	for _, f := range files {
		if isProgressPath(f.Path) {
			continue
		}
		nonProgress++
		if isCodePath(f.Path) {
			hasCode = true
		}
	}
	if nonProgress == 0 {
		return false, "diff is only progress.md"
	}
	_ = hasCode
	return true, "ok"
}

