// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"path"
	"regexp"
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

// NodePolicy controls per-node execution behaviour (M10). Defaults are all false.
type NodePolicy struct {
	AutoApprove   bool `json:"autoApprove"`
	AutoStartNext bool `json:"autoStartNext"`
}

// DefaultNodePolicy is the safe default: manual review + manual continue.
func DefaultNodePolicy() NodePolicy {
	return NodePolicy{AutoApprove: false, AutoStartNext: false}
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
	Policy    *NodePolicy `json:"policy,omitempty"`
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
	Policy    *NodePolicy `json:"policy"`
	DodChecks []string    `json:"dodChecks"`
	OnFailure string      `json:"onFailure"`
	Prompt    *PromptSpec `json:"prompt"`
}

type createRequest struct {
	BundleID      string              `json:"bundleId"`
	BundleRef     string              `json:"bundleRef"`
	SlaveID       string              `json:"slaveId"`
	RepoID        string              `json:"repoId"`
	ProgressDoc   string              `json:"progressDoc"`
	DefaultModel  string              `json:"defaultModel"`
	DefaultPolicy *NodePolicy         `json:"defaultPolicy"`
	Nodes         []createNodeRequest `json:"nodes"`
}

// Starter delivers workflow / revise / review messages to an online Slave.
type Starter interface {
	AssignWorkflow(run *Run) bool
	AssignRevise(slaveID, workflowID, nodeID, instruction string) bool
	// AssignReview delivers approve/reject. autoApprove marks M10-P03 automatic approve.
	AssignReview(slaveID, workflowID, nodeID, decision, comment string, autoApprove bool) bool
}

// ReviewAuditor records approve/reject with auto vs human distinction (optional).
type ReviewAuditor interface {
	AuditReview(workflowID, nodeID, decision string, autoApprove, autoStartNext bool)
}

type nodePatchRequest struct {
	Status *string     `json:"status"`
	TaskID *string     `json:"taskId"`
	UnitID *string     `json:"unitId"`
	Model  *string     `json:"model"`
	Policy *NodePolicy `json:"policy"`
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
	auditor  ReviewAuditor
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

func (s *Store) SetReviewAuditor(a ReviewAuditor) { s.auditor = a }

func (s *Store) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/workflows", s.handleCreate)
	mux.HandleFunc("GET /v1/workflows", s.handleList)
	mux.HandleFunc("GET /v1/workflows/{id}", s.handleGet)
	mux.HandleFunc("GET /v1/workflows/{id}/nodes", s.handleListNodes)
	mux.HandleFunc("POST /v1/workflows/{id}/start", s.handleStart)
	mux.HandleFunc("POST /v1/workflows/{id}/continue", s.handleContinue)
	mux.HandleFunc("PATCH /v1/workflows/{id}/nodes/{nodeId}", s.handlePatchNode)
	mux.HandleFunc("POST /v1/workflows/{id}/nodes/{nodeId}/start", s.handleStartNode)
	mux.HandleFunc("POST /v1/workflows/{id}/nodes/{nodeId}/reset", s.handleResetNode)
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
	defModel := "auto"
	if m := strings.TrimSpace(req.DefaultModel); m != "" {
		defModel = m
	}
	defPol := DefaultNodePolicy()
	if req.DefaultPolicy != nil {
		defPol.AutoApprove = req.DefaultPolicy.AutoApprove
		defPol.AutoStartNext = req.DefaultPolicy.AutoStartNext
	}
	milestoneSerial := strings.HasPrefix(req.BundleID, "milestone:")
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
			Status:    NodePending, // set after optional serial rewrite
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
		} else {
			node.Model = strPtr(defModel)
		}
		pol := defPol
		if n.Policy != nil {
			pol = NodePolicy{
				AutoApprove:   n.Policy.AutoApprove,
				AutoStartNext: n.Policy.AutoStartNext,
			}
		}
		node.Policy = &pol
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

	if milestoneSerial {
		EnforceSerialDependsOn(nodes)
	}
	for i := range nodes {
		nodes[i].Status = InitialNodeStatus(nodes[i].DependsOn)
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
// Prefer exact slaveId match; if none, fall back to same repoId (M11 remap).
func (s *Store) ListMilestoneRuns(slaveID, repoID string) []*Run {
	slaveID = strings.TrimSpace(slaveID)
	repoID = strings.TrimSpace(repoID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	collect := func(requireSlave bool) []*Run {
		out := make([]*Run, 0)
		for _, run := range s.runs {
			if !IsMilestoneBundle(run.BundleID) {
				continue
			}
			if repoID != "" && run.RepoID != repoID {
				continue
			}
			if requireSlave && slaveID != "" {
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

	exact := collect(true)
	if len(exact) > 0 || slaveID == "" || repoID == "" {
		return exact
	}
	return collect(false)
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

// CatchUpFromProgress marks nodes approved on active milestone runs when local
// progress.md already lists those phases as done (progress_ahead recovery).
// Clears taskId; recomputes ready/workflow status. Returns how many nodes changed.
func (s *Store) CatchUpFromProgress(slaveID, repoID string, donePhases []string) (approved int, workflowIDs []string) {
	slaveID = strings.TrimSpace(slaveID)
	repoID = strings.TrimSpace(repoID)
	done := map[string]struct{}{}
	for _, p := range donePhases {
		p = strings.TrimSpace(p)
		if p != "" {
			done[p] = struct{}{}
		}
	}
	if len(done) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	wfSeen := map[string]struct{}{}
	u := s.now().Format(time.RFC3339Nano)
	for _, run := range s.runs {
		if !IsMilestoneBundle(run.BundleID) {
			continue
		}
		if !IsActiveRun(run.Status) {
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
		changed := false
		for i := range run.Nodes {
			key := phaseKeyFromNode(run.Nodes[i])
			if key == "" {
				continue
			}
			if _, ok := done[key]; !ok {
				continue
			}
			st := run.Nodes[i].Status
			switch st {
			case NodePending, NodeReady, NodeRunning, NodeAwaitingReview,
				NodeFailed, NodeRejected, NodeCancelled:
				run.Nodes[i].Status = NodeApproved
				run.Nodes[i].TaskID = nil
				changed = true
				approved++
			}
		}
		if changed {
			run.UpdatedAt = &u
			syncWorkflowStatusAfterNodes(run)
			wfSeen[run.ID] = struct{}{}
		}
	}
	s.mu.Unlock()
	if approved > 0 {
		s.notifyChange()
	}
	workflowIDs = make([]string, 0, len(wfSeen))
	for id := range wfSeen {
		workflowIDs = append(workflowIDs, id)
	}
	sort.Strings(workflowIDs)
	return approved, workflowIDs
}

var (
	phaseIDExact = regexp.MustCompile(`^M\d{2}-P\d{2}$`)
	phaseIDAny   = regexp.MustCompile(`M\d{2}-P\d{2}`)
)

func phaseKeyFromNode(n Node) string {
	id := strings.TrimSpace(n.ID)
	if phaseIDExact.MatchString(id) {
		return id
	}
	if n.UnitID != nil {
		u := strings.TrimSpace(*n.UnitID)
		if phaseIDExact.MatchString(u) {
			return u
		}
	}
	if n.PhaseRef != nil {
		base := path.Base(strings.TrimSpace(*n.PhaseRef))
		if m := phaseIDAny.FindString(base); m != "" {
			return m
		}
	}
	if m := phaseIDAny.FindString(id); m != "" {
		return m
	}
	return ""
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

// handleContinue re-assigns the run so Slave can pick up ready nodes (manual continue gate).
func (s *Store) handleContinue(w http.ResponseWriter, r *http.Request) {
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
	if !anyNode(run.Nodes, NodeReady) {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "no ready nodes to continue"})
		return
	}
	u := s.now().Format(time.RFC3339Nano)
	run.UpdatedAt = &u
	if run.Status == StatusPending {
		run.Status = StatusRunning
	}
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

// handleStartNode assigns the workflow when the named node is ready (explicit start).
func (s *Store) handleStartNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	s.mu.Lock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
		return
	}
	switch run.Status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "workflow already terminal"})
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
	if run.Nodes[idx].Status != NodeReady {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node must be ready"})
		return
	}
	u := s.now().Format(time.RFC3339Nano)
	run.UpdatedAt = &u
	if run.Status == StatusPending {
		run.Status = StatusRunning
	}
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
		"nodeId":    nodeID,
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
	if req.Status == nil && req.TaskID == nil && req.UnitID == nil && req.Model == nil && req.Policy == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no fields to patch"})
		return
	}

	s.mu.Lock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
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
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	enteredAwaiting := false
	if req.Status != nil {
		st := strings.TrimSpace(*req.Status)
		if !validNodeStatus(st) {
			s.mu.Unlock()
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid node status"})
			return
		}
		// Review gate: Agent/Slave must report awaiting_review; approve is via review or autoApprove.
		run.Nodes[idx].Status = st
		enteredAwaiting = st == NodeAwaitingReview
	}
	if req.TaskID != nil {
		v := strings.TrimSpace(*req.TaskID)
		if v == "" {
			run.Nodes[idx].TaskID = nil
		} else {
			run.Nodes[idx].TaskID = &v
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
	if req.Model != nil {
		v := strings.TrimSpace(*req.Model)
		if v == "" {
			v = "auto"
		}
		run.Nodes[idx].Model = &v
	}
	if req.Policy != nil {
		pol := DefaultNodePolicy()
		if run.Nodes[idx].Policy != nil {
			pol = *run.Nodes[idx].Policy
		}
		pol.AutoApprove = req.Policy.AutoApprove
		pol.AutoStartNext = req.Policy.AutoStartNext
		run.Nodes[idx].Policy = &pol
	}

	autoApproved := false
	autoStartNext := false
	if enteredAwaiting && nodeWantsAutoApprove(run.Nodes[idx]) {
		// Same pipeline as human approve (progress + commit via AssignReview).
		run.Nodes[idx].Status = NodeApproved
		autoApproved = true
		autoStartNext = nodeWantsAutoStartNext(run.Nodes[idx])
	}

	syncWorkflowStatusAfterNodes(run)
	u := s.now().Format(time.RFC3339Nano)
	run.UpdatedAt = &u
	slaveID := ""
	if run.SlaveID != nil {
		slaveID = *run.SlaveID
	}
	out := cloneRun(run)
	s.mu.Unlock()
	s.notifyChange()

	delivered := false
	if autoApproved {
		delivered = s.deliverReview(slaveID, id, nodeID, "approve", "autoApprove", true, autoStartNext, out)
		_ = delivered
	}
	// Body remains the Run (backward compatible with Slave/App PATCH clients).
	writeJSON(w, http.StatusOK, out)
}

// InterruptRunningForSlave recovers nodes stuck in "running" after a Slave
// disconnect/restart. Same rules as gateway-restore normalizeInterruptedRun:
// running + stored diff → awaiting_review; else → ready. Clears taskId.
// Returns how many nodes were changed.
func (s *Store) InterruptRunningForSlave(slaveID string) int {
	if strings.TrimSpace(slaveID) == "" {
		return 0
	}
	s.mu.Lock()
	n := 0
	u := s.now().Format(time.RFC3339Nano)
	for _, run := range s.runs {
		if run.SlaveID == nil || *run.SlaveID != slaveID {
			continue
		}
		if run.Status == StatusCompleted || run.Status == StatusCancelled {
			continue
		}
		changed := false
		for i := range run.Nodes {
			if run.Nodes[i].Status != NodeRunning {
				continue
			}
			interruptRunningNode(run, i, s.diffs)
			changed = true
			n++
		}
		if changed {
			run.UpdatedAt = &u
			syncWorkflowStatusAfterNodes(run)
		}
	}
	s.mu.Unlock()
	if n > 0 {
		s.notifyChange()
	}
	return n
}

// interruptRunningNode applies restore/offline recovery to one running node.
// Caller must hold s.mu when using store diffs.
func interruptRunningNode(run *Run, idx int, diffs map[string]*NodeDiff) {
	run.Nodes[idx].TaskID = nil
	key := diffKey(run.ID, run.Nodes[idx].ID)
	if _, ok := diffs[key]; ok {
		run.Nodes[idx].Status = NodeAwaitingReview
	} else {
		run.Nodes[idx].Status = NodeReady
	}
}

// handleResetNode clears a failed/rejected/cancelled/running node back to ready/pending
// without recreating the whole workflow. Optional body: {"start":true} to assign if ready.
func (s *Store) handleResetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	nodeID := r.PathValue("nodeId")
	startAfter := false
	if r.Body != nil && r.ContentLength != 0 {
		var req struct {
			Start *bool `json:"start"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Start != nil {
			startAfter = *req.Start
		}
	}

	s.mu.Lock()
	run, ok := s.runs[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workflow not found"})
		return
	}
	if run.Status == StatusCompleted {
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "workflow completed"})
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
	st := run.Nodes[idx].Status
	switch st {
	case NodeFailed, NodeRejected, NodeCancelled, NodeRunning:
		// running: orphaned after Slave restart / lost task (manual recover)
	default:
		s.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "node must be failed, rejected, cancelled, or running",
		})
		return
	}

	// Clear prior task so logs/UI don't stick to the failed run.
	run.Nodes[idx].TaskID = nil
	run.Nodes[idx].Status = NodePending
	syncWorkflowStatusAfterNodes(run)
	// Ensure this node is ready when deps are satisfied (RecomputeReady only touches pending).
	if run.Nodes[idx].Status == NodePending {
		// deps already checked inside RecomputeReady; if still pending, deps not approved
	}
	u := s.now().Format(time.RFC3339Nano)
	run.UpdatedAt = &u
	out := cloneRun(run)
	nodeReady := out.Nodes[idx].Status == NodeReady
	s.mu.Unlock()
	s.notifyChange()

	delivered := false
	if startAfter && nodeReady && s.starter != nil {
		delivered = s.starter.AssignWorkflow(out)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":  out,
		"delivered": delivered,
		"nodeId":    nodeID,
		"started":   startAfter && nodeReady,
	})
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

// syncWorkflowStatusAfterNodes recomputes ready nodes and workflow status.
// Clearing the last failed/rejected node can revive a failed workflow to running.
func syncWorkflowStatusAfterNodes(run *Run) {
	if run == nil {
		return
	}
	RecomputeReady(run.Nodes)
	if anyNode(run.Nodes, NodeFailed) || anyNode(run.Nodes, NodeRejected) {
		run.Status = StatusFailed
		return
	}
	if allNodesTerminalSuccess(run.Nodes) {
		run.Status = StatusCompleted
		return
	}
	if run.Status == StatusFailed || run.Status == StatusCancelled {
		run.Status = StatusRunning
		return
	}
	if run.Status == StatusPending {
		run.Status = StatusRunning
	}
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
	autoStartNext := false
	if decision == "approve" {
		run.Nodes[idx].Status = NodeApproved
		autoStartNext = nodeWantsAutoStartNext(run.Nodes[idx])
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

	delivered := s.deliverReview(slaveID, id, nodeID, decision, comment, false, autoStartNext, out)
	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":      out,
		"delivered":     delivered,
		"decision":      decision,
		"comment":       comment,
		"autoApprove":   false,
		"autoStartNext": autoStartNext,
	})
}

// deliverReview notifies Slave (progress/commit on approve) and optionally continues the DAG.
func (s *Store) deliverReview(
	slaveID, workflowID, nodeID, decision, comment string,
	autoApprove, autoStartNext bool,
	out *Run,
) bool {
	if s.auditor != nil {
		s.auditor.AuditReview(workflowID, nodeID, decision, autoApprove, autoStartNext)
	}
	delivered := false
	if s.starter != nil && slaveID != "" {
		delivered = s.starter.AssignReview(slaveID, workflowID, nodeID, decision, comment, autoApprove)
		if !delivered {
			log.Printf("workflow.review not delivered to slave %q (offline?); progress/commit may be skipped autoApprove=%v", slaveID, autoApprove)
		}
		if decision == "approve" && autoStartNext {
			_ = s.starter.AssignWorkflow(out)
		}
	} else if slaveID == "" {
		log.Printf("workflow.review: workflow %s has empty slaveId; progress/commit skipped autoApprove=%v", workflowID, autoApprove)
	}
	return delivered
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
		if n.Policy != nil {
			p := *n.Policy
			out[i].Policy = &p
		}
	}
	return out
}

func nodeWantsAutoStartNext(n Node) bool {
	return n.Policy != nil && n.Policy.AutoStartNext
}

func nodeWantsAutoApprove(n Node) bool {
	return n.Policy != nil && n.Policy.AutoApprove
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
