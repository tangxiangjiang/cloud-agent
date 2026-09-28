// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Workflow / node status align with contracts/schemas.
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"

	NodePending        = "pending"
	NodeReady          = "ready"
	NodeRunning        = "running"
	NodeAwaitingReview = "awaiting_review"
	NodeApproved       = "approved"
	NodeRejected       = "rejected"
	NodeFailed         = "failed"
	NodeCancelled      = "cancelled"
	NodeSkipped        = "skipped"
)

type PromptSpec struct {
	Mode   string `json:"mode,omitempty"`
	Extra  string `json:"extra,omitempty"`
	Inline string `json:"inline,omitempty"`
}

// Node is a DAG node snapshot inside a WorkflowRun.
type Node struct {
	ID        string      `json:"id"`
	PhaseRef  *string     `json:"phaseRef"`
	Title     *string     `json:"title"`
	DependsOn []string    `json:"dependsOn"`
	Status    string      `json:"status"`
	TaskID    *string     `json:"taskId"`
	UnitID    *string     `json:"unitId"`
	Model     *string     `json:"model,omitempty"`
	DodChecks []string    `json:"dodChecks,omitempty"`
	OnFailure *string     `json:"onFailure,omitempty"`
	Prompt    *PromptSpec `json:"prompt,omitempty"`
}

// Run is a runtime workflow instance (WorkflowRun).
type Run struct {
	ID          string  `json:"id"`
	BundleID    string  `json:"bundleId"`
	BundleRef   *string `json:"bundleRef"`
	SlaveID     *string `json:"slaveId"`
	RepoID      string  `json:"repoId"`
	ProgressDoc *string `json:"progressDoc"`
	Status      string  `json:"status"`
	Nodes       []Node  `json:"nodes"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   *string `json:"updatedAt"`
}

type createNodeRequest struct {
	ID        string      `json:"id"`
	PhaseRef  string      `json:"phaseRef"`
	Title     string      `json:"title"`
	DependsOn []string    `json:"dependsOn"`
	Model     string      `json:"model"`
	DodChecks []string    `json:"dodChecks"`
	OnFailure string      `json:"onFailure"`
	Prompt    *PromptSpec `json:"prompt"`
}

type createRequest struct {
	BundleID    string              `json:"bundleId"`
	BundleRef   string              `json:"bundleRef"`
	SlaveID     string              `json:"slaveId"`
	RepoID      string              `json:"repoId"`
	ProgressDoc string              `json:"progressDoc"`
	Nodes       []createNodeRequest `json:"nodes"`
}

// Starter delivers a workflow run to an online Slave (implemented by slaves.OutboundHub).
type Starter interface {
	AssignWorkflow(run *Run) bool
}

type nodePatchRequest struct {
	Status *string `json:"status"`
	TaskID *string `json:"taskId"`
	UnitID *string `json:"unitId"`
}

type Store struct {
	mu      sync.RWMutex
	runs    map[string]*Run
	diffs   map[string]*NodeDiff // workflowId\0nodeId
	now     func() time.Time
	starter Starter
}

func NewStore() *Store {
	return &Store{
		runs:  make(map[string]*Run),
		diffs: make(map[string]*NodeDiff),
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Store) SetStarter(st Starter) { s.starter = st }

func (s *Store) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workflows", s.handleCreate)
	mux.HandleFunc("GET /v1/workflows", s.handleList)
	mux.HandleFunc("GET /v1/workflows/{id}", s.handleGet)
	mux.HandleFunc("GET /v1/workflows/{id}/nodes", s.handleListNodes)
	mux.HandleFunc("POST /v1/workflows/{id}/start", s.handleStart)
	mux.HandleFunc("PATCH /v1/workflows/{id}/nodes/{nodeId}", s.handlePatchNode)
	mux.HandleFunc("GET /v1/workflows/{id}/nodes/{nodeId}/diff", s.handleGetDiff)
	mux.HandleFunc("PUT /v1/workflows/{id}/nodes/{nodeId}/diff", s.handlePutDiff)
	return mux
}

func (s *Store) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.BundleID = strings.TrimSpace(req.BundleID)
	req.RepoID = strings.TrimSpace(req.RepoID)
	if req.BundleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bundleId required"})
		return
	}
	if req.RepoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repoId required"})
		return
	}
	if len(req.Nodes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "nodes required"})
		return
	}

	nodes := make([]Node, 0, len(req.Nodes))
	for _, n := range req.Nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node.id required"})
			return
		}
		deps := make([]string, 0, len(n.DependsOn))
		deps = append(deps, n.DependsOn...)
		node := Node{
			ID:        id,
			DependsOn: deps,
			Status:    InitialNodeStatus(deps),
			TaskID:    nil,
			UnitID:    nil,
		}
		if p := strings.TrimSpace(n.PhaseRef); p != "" {
			node.PhaseRef = strPtr(p)
		} else {
			node.PhaseRef = nil
		}
		if t := strings.TrimSpace(n.Title); t != "" {
			node.Title = strPtr(t)
		} else {
			node.Title = nil
		}
		if m := strings.TrimSpace(n.Model); m != "" {
			node.Model = strPtr(m)
		}
		if len(n.DodChecks) > 0 {
			node.DodChecks = append([]string(nil), n.DodChecks...)
		}
		if of := strings.TrimSpace(n.OnFailure); of != "" {
			node.OnFailure = strPtr(of)
		}
		if n.Prompt != nil {
			cp := *n.Prompt
			node.Prompt = &cp
		}
		nodes = append(nodes, node)
	}

	if err := ValidateDAG(nodes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	id, err := newID("wf_")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "id generation failed"})
		return
	}
	now := s.now().Format(time.RFC3339Nano)
	run := &Run{
		ID:        id,
		BundleID:  req.BundleID,
		RepoID:    req.RepoID,
		Status:    StatusPending,
		Nodes:     nodes,
		CreatedAt: now,
	}
	if v := strings.TrimSpace(req.BundleRef); v != "" {
		run.BundleRef = strPtr(v)
	} else {
		run.BundleRef = nil
	}
	if v := strings.TrimSpace(req.SlaveID); v != "" {
		run.SlaveID = strPtr(v)
	} else {
		run.SlaveID = nil
	}
	if v := strings.TrimSpace(req.ProgressDoc); v != "" {
		run.ProgressDoc = strPtr(v)
	} else {
		run.ProgressDoc = nil
	}

	s.mu.Lock()
	s.runs[id] = run
	out := cloneRun(run)
	s.mu.Unlock()

	writeJSON(w, http.StatusCreated, out)
}

func (s *Store) handleList(w http.ResponseWriter, r *http.Request) {
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	s.mu.RLock()
	list := make([]*Run, 0, len(s.runs))
	for _, run := range s.runs {
		if statusFilter != "" && run.Status != statusFilter {
			continue
		}
		list = append(list, cloneRun(run))
	}
	s.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	if len(list) > limit {
		list = list[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflows": list})
}

func (s *Store) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.RUnlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	out := cloneRun(run)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Store) handleListNodes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.RUnlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	nodes := cloneNodes(run.Nodes)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes})
}

func (s *Store) handleStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch run.Status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "workflow already terminal"})
		return
	}
	u := s.now().Format(time.RFC3339Nano)
	run.UpdatedAt = &u
	run.Status = StatusRunning
	out := cloneRun(run)
	s.mu.Unlock()

	delivered := false
	if s.starter != nil {
		delivered = s.starter.AssignWorkflow(out)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":  out,
		"delivered": delivered,
	})
}

func (s *Store) handlePatchNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	var req nodePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Status == nil && req.TaskID == nil && req.UnitID == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no fields to patch"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	idx := -1
	for i := range run.Nodes {
		if run.Nodes[i].ID == nodeID {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if req.Status != nil {
		st := strings.TrimSpace(*req.Status)
		if !validNodeStatus(st) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid node status"})
			return
		}
		// Review gate: never accept approved via this path in P02? Approve is P05.
		// Still allow storing approved when App/Slave later patches — but RecomputeReady only unlocks on approved.
		run.Nodes[idx].Status = st
	}
	if req.TaskID != nil {
		v := strings.TrimSpace(*req.TaskID)
		if v == "" {
			run.Nodes[idx].TaskID = nil
		} else {
			run.Nodes[idx].TaskID = &v
		}
	}
	if req.UnitID != nil {
		v := strings.TrimSpace(*req.UnitID)
		if v == "" {
			run.Nodes[idx].UnitID = nil
		} else {
			run.Nodes[idx].UnitID = &v
		}
	}
	RecomputeReady(run.Nodes)

	// Workflow-level status from nodes (simple).
	if anyNode(run.Nodes, NodeFailed) {
		run.Status = StatusFailed
	} else if allNodesTerminalSuccess(run.Nodes) {
		run.Status = StatusCompleted
	} else if run.Status == StatusPending {
		run.Status = StatusRunning
	}
	u := s.now().Format(time.RFC3339Nano)
	run.UpdatedAt = &u
	writeJSON(w, http.StatusOK, cloneRun(run))
}

func validNodeStatus(st string) bool {
	switch st {
	case NodePending, NodeReady, NodeRunning, NodeAwaitingReview, NodeApproved,
		NodeRejected, NodeFailed, NodeCancelled, NodeSkipped:
		return true
	default:
		return false
	}
}

func anyNode(nodes []Node, status string) bool {
	for _, n := range nodes {
		if n.Status == status {
			return true
		}
	}
	return false
}

func allNodesTerminalSuccess(nodes []Node) bool {
	for _, n := range nodes {
		switch n.Status {
		case NodeApproved, NodeSkipped:
			continue
		default:
			return false
		}
	}
	return len(nodes) > 0
}

func (s *Store) handleGetDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	s.mu.RLock()
	if _, ok := s.runs[id]; !ok {
		s.mu.RUnlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
		return
	}
	d, ok := s.diffs[diffKey(id, nodeID)]
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "diff not found"})
		return
	}
	writeJSON(w, http.StatusOK, cloneDiff(d))
}

func (s *Store) handlePutDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	var body NodeDiff
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
		return
	}
	found := false
	for _, n := range run.Nodes {
		if n.ID == nodeID {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if body.Files == nil {
		body.Files = []NodeDiffFile{}
	}
	for i := range body.Files {
		f := &body.Files[i]
		if strings.TrimSpace(f.Path) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "files[].path required"})
			return
		}
		if f.UnifiedDiff == "" && len(f.Hunks) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "files[] need unifiedDiff or hunks"})
			return
		}
	}
	body.WorkflowID = id
	body.NodeID = nodeID
	stored := cloneDiff(&body)
	s.diffs[diffKey(id, nodeID)] = stored
	writeJSON(w, http.StatusOK, stored)
}

func cloneDiff(d *NodeDiff) *NodeDiff {
	if d == nil {
		return nil
	}
	cp := *d
	if d.Baseline != nil {
		v := *d.Baseline
		cp.Baseline = &v
	}
	cp.Files = make([]NodeDiffFile, len(d.Files))
	for i, f := range d.Files {
		cp.Files[i] = f
		if f.Additions != nil {
			v := *f.Additions
			cp.Files[i].Additions = &v
		}
		if f.Deletions != nil {
			v := *f.Deletions
			cp.Files[i].Deletions = &v
		}
		if f.Hunks != nil {
			cp.Files[i].Hunks = append([]NodeDiffHunk(nil), f.Hunks...)
			for j := range cp.Files[i].Hunks {
				cp.Files[i].Hunks[j].Lines = append([]string(nil), f.Hunks[j].Lines...)
			}
		}
	}
	return &cp
}

func cloneRun(r *Run) *Run {
	if r == nil {
		return nil
	}
	cp := *r
	cp.Nodes = cloneNodes(r.Nodes)
	if r.BundleRef != nil {
		v := *r.BundleRef
		cp.BundleRef = &v
	}
	if r.SlaveID != nil {
		v := *r.SlaveID
		cp.SlaveID = &v
	}
	if r.ProgressDoc != nil {
		v := *r.ProgressDoc
		cp.ProgressDoc = &v
	}
	if r.UpdatedAt != nil {
		v := *r.UpdatedAt
		cp.UpdatedAt = &v
	}
	return &cp
}

func cloneNodes(in []Node) []Node {
	out := make([]Node, len(in))
	for i, n := range in {
		out[i] = n
		deps := make([]string, 0, len(n.DependsOn))
		out[i].DependsOn = append(deps, n.DependsOn...)
		if n.PhaseRef != nil {
			v := *n.PhaseRef
			out[i].PhaseRef = &v
		}
		if n.Title != nil {
			v := *n.Title
			out[i].Title = &v
		}
		if n.TaskID != nil {
			v := *n.TaskID
			out[i].TaskID = &v
		}
		if n.UnitID != nil {
			v := *n.UnitID
			out[i].UnitID = &v
		}
		if n.Model != nil {
			v := *n.Model
			out[i].Model = &v
		}
		if n.DodChecks != nil {
			out[i].DodChecks = append([]string(nil), n.DodChecks...)
		}
		if n.OnFailure != nil {
			v := *n.OnFailure
			out[i].OnFailure = &v
		}
		if n.Prompt != nil {
			p := *n.Prompt
			out[i].Prompt = &p
		}
	}
	return out
}

func strPtr(s string) *string { return &s }

func newID(prefix string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b[:]), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
