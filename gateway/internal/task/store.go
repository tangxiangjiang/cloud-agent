// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package task

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

// Status values align with contracts/schemas/task-status.schema.json.
const (
	StatusQueued     = "queued"
	StatusRunning    = "running"
	StatusCancelling = "cancelling"
	StatusFinished   = "finished"
	StatusError      = "error"
	StatusCancelled  = "cancelled"
)

type TaskError struct {
	Message   string  `json:"message"`
	Retryable bool    `json:"retryable,omitempty"`
	Code      *string `json:"code"`
}

type Task struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	SlaveID       *string    `json:"slaveId"`
	RepoID        *string    `json:"repoId"`
	Prompt        *string    `json:"prompt"`
	Model         *string    `json:"model"`
	ChatID        *string    `json:"chatId,omitempty"`
	Mode          *string    `json:"mode,omitempty"`
	AgentID       *string    `json:"agentId"`
	RunID         *string    `json:"runId"`
	WorkflowID    *string    `json:"workflowId"`
	NodeID        *string    `json:"nodeId"`
	ResultSummary *string    `json:"resultSummary"`
	Error         *TaskError `json:"error"`
	CreatedAt     string     `json:"createdAt"`
	UpdatedAt     *string    `json:"updatedAt"`
}

// Dispatcher delivers tasks/cancels to an online Slave (implemented by slaves.OutboundHub).
type Dispatcher interface {
	Assign(t *Task) bool
	RequestCancel(taskID, slaveID string) bool
}

// EventSource provides HTTP fallback snapshots of task.event stream.
type EventSource interface {
	EventsAfterMaps(taskID string, afterSeq int) (events []map[string]any, latestSeq int)
}

type createRequest struct {
	SlaveID    string `json:"slaveId"`
	RepoID     string `json:"repoId"`
	Prompt     string `json:"prompt"`
	Model      string `json:"model"`
	WorkflowID string `json:"workflowId"`
	NodeID     string `json:"nodeId"`
	ChatID     string `json:"chatId"`
	Mode       string `json:"mode"`
}

// CreateInput is used by Chat and HTTP create.
type CreateInput struct {
	SlaveID    string
	RepoID     string
	Prompt     string
	Model      string
	WorkflowID string
	NodeID     string
	ChatID     string
	Mode       string
	IdemKey    string
}

type Store struct {
	mu         sync.RWMutex
	tasks      map[string]*Task
	byIdem     map[string]string
	now        func() time.Time
	dispatcher Dispatcher
	events     EventSource
}

func NewStore() *Store {
	return &Store{
		tasks:  make(map[string]*Task),
		byIdem: make(map[string]string),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Store) SetDispatcher(d Dispatcher) { s.dispatcher = d }

func (s *Store) SetEventSource(e EventSource) { s.events = e }

func (s *Store) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tasks", s.handleCreate)
	mux.HandleFunc("GET /v1/tasks", s.handleList)
	mux.HandleFunc("GET /v1/tasks/{id}", s.handleGet)
	mux.HandleFunc("GET /v1/tasks/{id}/events", s.handleEvents)
	mux.HandleFunc("POST /v1/tasks/{id}/cancel", s.handleCancel)
	return mux
}

func (s *Store) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt required"})
		return
	}

	out, replay, err := s.Create(CreateInput{
		SlaveID:    req.SlaveID,
		RepoID:     req.RepoID,
		Prompt:     req.Prompt,
		Model:      req.Model,
		WorkflowID: req.WorkflowID,
		NodeID:     req.NodeID,
		ChatID:     req.ChatID,
		Mode:       req.Mode,
		IdemKey:    strings.TrimSpace(r.Header.Get("Idempotency-Key")),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if replay {
		writeJSON(w, http.StatusOK, out)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// Create inserts a queued task and optionally dispatches to Slave (non-workflow).
// replay=true when Idempotency-Key hit an existing task.
func (s *Store) Create(in CreateInput) (out *Task, replay bool, err error) {
	idem := strings.TrimSpace(in.IdemKey)
	if idem != "" {
		s.mu.RLock()
		if id, ok := s.byIdem[idem]; ok {
			t := cloneTask(s.tasks[id])
			s.mu.RUnlock()
			return t, true, nil
		}
		s.mu.RUnlock()
	}

	now := s.now()
	id, err := newID("tsk")
	if err != nil {
		return nil, false, err
	}
	created := now.Format(time.RFC3339Nano)
	updated := created
	t := &Task{
		ID:        id,
		Status:    StatusQueued,
		SlaveID:   strPtr(strings.TrimSpace(in.SlaveID)),
		RepoID:    strPtr(strings.TrimSpace(in.RepoID)),
		Prompt:    strPtr(in.Prompt),
		Model:     strPtr(strings.TrimSpace(in.Model)),
		CreatedAt: created,
		UpdatedAt: &updated,
	}
	if v := strings.TrimSpace(in.WorkflowID); v != "" {
		t.WorkflowID = strPtr(v)
	}
	if v := strings.TrimSpace(in.NodeID); v != "" {
		t.NodeID = strPtr(v)
	}
	if v := strings.TrimSpace(in.ChatID); v != "" {
		t.ChatID = strPtr(v)
	}
	if v := strings.TrimSpace(in.Mode); v != "" {
		t.Mode = strPtr(v)
	}

	s.mu.Lock()
	if idem != "" {
		if idExisting, ok := s.byIdem[idem]; ok {
			existing := cloneTask(s.tasks[idExisting])
			s.mu.Unlock()
			return existing, true, nil
		}
		s.byIdem[idem] = id
	}
	s.tasks[id] = t
	out = cloneTask(t)
	s.mu.Unlock()

	// Workflow-owned tasks are executed by Slave DAG scheduler; skip WS assign.
	if s.dispatcher != nil && (out.WorkflowID == nil || *out.WorkflowID == "") {
		go s.dispatcher.Assign(out)
	}
	return out, false, nil
}

func (s *Store) handleList(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			limit = n
		}
	}

	s.mu.RLock()
	list := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		list = append(list, cloneTask(t))
	}
	s.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt > list[j].CreatedAt
	})
	if len(list) > limit {
		list = list[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": list})
}

func (s *Store) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	t, ok := s.tasks[id]
	var out *Task
	if ok {
		out = cloneTask(t)
	}
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Store) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	_, ok := s.tasks[id]
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	after := 0
	if v := r.URL.Query().Get("afterSeq"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n >= 0 {
			after = n
		}
	}
	events := []map[string]any{}
	latest := 0
	if s.events != nil {
		events, latest = s.events.EventsAfterMaps(id, after)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":    events,
		"latestSeq": latest,
	})
}

func (s *Store) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status, code := s.Cancel(id)
	if code == http.StatusNotFound {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, code, map[string]string{"status": status})
}

// Cancel requests Slave cancel when possible; returns resulting status and HTTP-ish code.
func (s *Store) Cancel(id string) (status string, code int) {
	s.mu.Lock()
	t, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return "", http.StatusNotFound
	}
	switch t.Status {
	case StatusFinished, StatusError, StatusCancelled:
		st := t.Status
		s.mu.Unlock()
		return st, http.StatusOK
	case StatusCancelling:
		s.mu.Unlock()
		return StatusCancelling, http.StatusOK
	}
	slaveID := ""
	if t.SlaveID != nil {
		slaveID = *t.SlaveID
	}
	s.mu.Unlock()

	delivered := false
	if s.dispatcher != nil && slaveID != "" {
		delivered = s.dispatcher.RequestCancel(id, slaveID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok = s.tasks[id]
	if !ok {
		return "", http.StatusNotFound
	}
	u := s.now().Format(time.RFC3339Nano)
	t.UpdatedAt = &u
	if delivered {
		t.Status = StatusCancelling
		return StatusCancelling, http.StatusOK
	}
	t.Status = StatusCancelled
	return StatusCancelled, http.StatusOK
}

// ListQueuedForSlave returns queued tasks for a slave (for claim-on-register).
func (s *Store) ListQueuedForSlave(slaveID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Task
	for _, t := range s.tasks {
		if t.Status != StatusQueued {
			continue
		}
		if t.SlaveID != nil && *t.SlaveID == slaveID {
			out = append(out, cloneTask(t))
		}
	}
	return out
}

// ApplySlaveEvent updates task status from a slave-reported event kind/payload.
func (s *Store) ApplySlaveEvent(taskID, kind string, payload map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return
	}
	u := s.now().Format(time.RFC3339Nano)
	t.UpdatedAt = &u
	switch kind {
	case "status":
		if st, _ := payload["status"].(string); st != "" {
			t.Status = st
		}
	case "done":
		st, _ := payload["status"].(string)
		switch st {
		case StatusFinished, StatusError, StatusCancelled:
			t.Status = st
		default:
			t.Status = StatusFinished
		}
	case "error":
		msg, _ := payload["message"].(string)
		if msg == "" {
			msg = "slave error"
		}
		t.Status = StatusError
		t.Error = &TaskError{Message: msg}
	}
}

func cloneTask(t *Task) *Task {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func newID(prefix string) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
