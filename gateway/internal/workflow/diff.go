// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

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
