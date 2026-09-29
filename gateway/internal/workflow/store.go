// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
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

// ReviseEntry is an audit record for App revise instructions (never discarded).
type ReviseEntry struct {
	NodeID      string  `json:"nodeId"`
	Instruction string  `json:"instruction"`
	At          string  `json:"at"`
	TaskID      *string `json:"taskId,omitempty"`
}

// Run is a runtime workflow instance (WorkflowRun).
type Run struct {
	ID            string        `json:"id"`
	BundleID      string        `json:"bundleId"`
	BundleRef     *string       `json:"bundleRef"`
	SlaveID       *string       `json:"slaveId"`
	RepoID        string        `json:"repoId"`
	ProgressDoc   *string       `json:"progressDoc"`
	Status        string        `json:"status"`
	Nodes         []Node        `json:"nodes"`
	ReviseHistory []ReviseEntry `json:"reviseHistory,omitempty"`
	CreatedAt     string        `json:"createdAt"`
	UpdatedAt     *string       `json:"updatedAt"`
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

// Starter delivers workflow / revise / review messages to an online Slave.
type Starter interface {
	AssignWorkflow(run *Run) bool
	AssignRevise(slaveID, workflowID, nodeID, instruction string) bool
	AssignReview(slaveID, workflowID, nodeID, decision, comment string) bool
}

type nodePatchRequest struct {
	Status *string `json:"status"`
	TaskID *string `json:"taskId"`
	UnitID *string `json:"unitId"`
}

type reviseRequest struct {
	Instruction string `json:"instruction"`
}

type reviewRequest struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

type Store struct {
	mu       sync.RWMutex
	runs     map[string]*Run
	diffs    map[string]*NodeDiff // workflowId\0nodeId
	now      func() time.Time
	starter  Starter
	onChange OnChange
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
	mux.HandleFunc("POST /v1/workflows/{id}/nodes/{nodeId}/revise", s.handleRevise)
	mux.HandleFunc("POST /v1/workflows/{id}/nodes/{nodeId}/review", s.handleReview)
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
	s.notifyChange()

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

// IsActiveRun reports whether a workflow is still in progress (App isActive).
func IsActiveRun(status string) bool {
	return status == StatusPending || status == StatusRunning
}

// IsMilestoneBundle reports bundleId like "milestone:M01".
func IsMilestoneBundle(bundleID string) bool {
	return strings.HasPrefix(strings.TrimSpace(bundleID), "milestone:")
}

// ListMilestoneRuns returns milestone:* workflows for slave+repo, newest first.
func (s *Store) ListMilestoneRuns(slaveID, repoID string) []*Run {
	slaveID = strings.TrimSpace(slaveID)
	repoID = strings.TrimSpace(repoID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Run, 0)
	for _, run := range s.runs {
		if !IsMilestoneBundle(run.BundleID) {
			continue
		}
		if repoID != "" && run.RepoID != repoID {
			continue
		}
		if slaveID != "" {
			if run.SlaveID == nil || *run.SlaveID != slaveID {
				continue
			}
		}
		out = append(out, cloneRun(run))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

// FindLatestMilestoneRun prefers newest active milestone:* run; else newest matching.
// active is true when the returned run is pending/running.
func (s *Store) FindLatestMilestoneRun(slaveID, repoID string) (run *Run, active bool) {
	list := s.ListMilestoneRuns(slaveID, repoID)
	if len(list) == 0 {
		return nil, false
	}
	for _, r := range list {
		if IsActiveRun(r.Status) {
			return r, true
		}
	}
	return list[0], false
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
	s.notifyChange()

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

	saved := false
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		if saved {
			s.notifyChange()
		}
	}()
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
			// Attach taskId to the latest revise audit entry for this node (if any).
			for i := len(run.ReviseHistory) - 1; i >= 0; i-- {
				if run.ReviseHistory[i].NodeID == nodeID && run.ReviseHistory[i].TaskID == nil {
					run.ReviseHistory[i].TaskID = &v
					break
				}
			}
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
	saved = true
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

func (s *Store) handleRevise(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	var req reviseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	instruction := strings.TrimSpace(req.Instruction)
	if instruction == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instruction required"})
		return
	}

	s.mu.Lock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
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
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if run.Nodes[idx].Status != NodeAwaitingReview {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node must be awaiting_review"})
		return
	}
	// Never approve via revise.
	u := s.now().Format(time.RFC3339Nano)
	run.Nodes[idx].Status = NodeRunning
	run.UpdatedAt = &u
	run.Status = StatusRunning
	entry := ReviseEntry{
		NodeID:      nodeID,
		Instruction: instruction,
		At:          u,
	}
	run.ReviseHistory = append(run.ReviseHistory, entry)
	slaveID := ""
	if run.SlaveID != nil {
		slaveID = *run.SlaveID
	}
	out := cloneRun(run)
	s.mu.Unlock()
	s.notifyChange()

	delivered := false
	if s.starter != nil && slaveID != "" {
		delivered = s.starter.AssignRevise(slaveID, id, nodeID, instruction)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":  out,
		"delivered": delivered,
		"revise":    entry,
	})
}

func (s *Store) handleReview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	decision := strings.TrimSpace(strings.ToLower(req.Decision))
	if decision != "approve" && decision != "reject" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decision must be approve or reject"})
		return
	}
	comment := strings.TrimSpace(req.Comment)

	s.mu.Lock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
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
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	if run.Nodes[idx].Status != NodeAwaitingReview {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node must be awaiting_review"})
		return
	}

	u := s.now().Format(time.RFC3339Nano)
	if decision == "approve" {
		run.Nodes[idx].Status = NodeApproved
	} else {
		run.Nodes[idx].Status = NodeRejected
	}
	RecomputeReady(run.Nodes)
	if anyNode(run.Nodes, NodeFailed) {
		run.Status = StatusFailed
	} else if allNodesTerminalSuccess(run.Nodes) {
		run.Status = StatusCompleted
	} else if decision == "reject" {
		// Reject stops the gate; leave workflow running/failed-ish — mark failed if no path.
		run.Status = StatusFailed
	} else {
		run.Status = StatusRunning
	}
	run.UpdatedAt = &u
	slaveID := ""
	if run.SlaveID != nil {
		slaveID = *run.SlaveID
	}
	out := cloneRun(run)
	s.mu.Unlock()
	s.notifyChange()

	delivered := false
	if s.starter != nil && slaveID != "" {
		delivered = s.starter.AssignReview(slaveID, id, nodeID, decision, comment)
		if !delivered {
			log.Printf("workflow.review not delivered to slave %q (offline?); progress/commit may be skipped", slaveID)
		}
		// After approve, push workflow again so Slave can schedule newly ready nodes.
		if decision == "approve" {
			_ = s.starter.AssignWorkflow(out)
		}
	} else if slaveID == "" {
		log.Printf("workflow.review: workflow %s has empty slaveId; progress/commit skipped", id)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":  out,
		"delivered": delivered,
		"decision":  decision,
		"comment":   comment,
	})
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
	saved := false
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		if saved {
			s.notifyChange()
		}
	}()
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
	saved = true
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
	if r.ReviseHistory != nil {
		cp.ReviseHistory = make([]ReviseEntry, len(r.ReviseHistory))
		copy(cp.ReviseHistory, r.ReviseHistory)
		for i := range cp.ReviseHistory {
			if r.ReviseHistory[i].TaskID != nil {
				v := *r.ReviseHistory[i].TaskID
				cp.ReviseHistory[i].TaskID = &v
			}
		}
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
