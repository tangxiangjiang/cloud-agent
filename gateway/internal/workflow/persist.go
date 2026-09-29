// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

import (
	"encoding/json"
	"log"
)

// SnapshotPayload is JSON-serializable workflow+diff state for persist.
type SnapshotPayload struct {
	Workflows []Run               `json:"workflows"`
	Diffs     map[string]NodeDiff `json:"diffs"`
}

// OnChange is invoked after mutating store contents (create/patch/review/…).
type OnChange func()

func (s *Store) SetOnChange(fn OnChange) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

func (s *Store) notifyChange() {
	s.mu.RLock()
	fn := s.onChange
	s.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// ExportSnapshot copies runs and diffs for persistence (caller may serialize).
func (s *Store) ExportSnapshot() SnapshotPayload {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := SnapshotPayload{
		Workflows: make([]Run, 0, len(s.runs)),
		Diffs:     make(map[string]NodeDiff, len(s.diffs)),
	}
	for _, r := range s.runs {
		out.Workflows = append(out.Workflows, *cloneRun(r))
	}
	for k, d := range s.diffs {
		out.Diffs[k] = *cloneDiff(d)
	}
	return out
}

// ImportSnapshot replaces in-memory runs/diffs. Normalizes interrupted "running" nodes.
func (s *Store) ImportSnapshot(payload SnapshotPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs = make(map[string]*Run, len(payload.Workflows))
	s.diffs = make(map[string]*NodeDiff, len(payload.Diffs))
	for i := range payload.Workflows {
		r := payload.Workflows[i]
		normalizeInterruptedRun(&r, payload.Diffs)
		cp := cloneRun(&r)
		s.runs[cp.ID] = cp
	}
	for k, d := range payload.Diffs {
		dd := d
		s.diffs[k] = cloneDiff(&dd)
	}
	log.Printf("workflow state restored: %d run(s), %d diff(s)", len(s.runs), len(s.diffs))
}

func normalizeInterruptedRun(r *Run, diffs map[string]NodeDiff) {
	for i := range r.Nodes {
		if r.Nodes[i].Status != NodeRunning {
			continue
		}
		key := diffKey(r.ID, r.Nodes[i].ID)
		if _, ok := diffs[key]; ok {
			r.Nodes[i].Status = NodeAwaitingReview
		} else {
			r.Nodes[i].Status = NodeReady
		}
	}
}

// MarshalSnapshotJSON helper for persist layer.
func (s *Store) MarshalSnapshotJSON() (workflows, diffs []byte, err error) {
	snap := s.ExportSnapshot()
	workflows, err = json.Marshal(snap.Workflows)
	if err != nil {
		return nil, nil, err
	}
	diffs, err = json.Marshal(snap.Diffs)
	if err != nil {
		return nil, nil, err
	}
	return workflows, diffs, nil
}

func (s *Store) UnmarshalSnapshotJSON(workflows, diffs []byte) error {
	var payload SnapshotPayload
	if len(workflows) > 0 {
		if err := json.Unmarshal(workflows, &payload.Workflows); err != nil {
			return err
		}
	}
	if len(diffs) > 0 {
		if err := json.Unmarshal(diffs, &payload.Diffs); err != nil {
			return err
		}
	}
	if payload.Diffs == nil {
		payload.Diffs = map[string]NodeDiff{}
	}
	s.ImportSnapshot(payload)
	return nil
}
