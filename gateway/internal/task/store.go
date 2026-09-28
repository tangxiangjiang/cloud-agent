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
	AgentID       *string    `json:"agentId"`
	RunID         *string    `json:"runId"`
	WorkflowID    *string    `json:"workflowId"`
	NodeID        *string    `json:"nodeId"`
	ResultSummary *string    `json:"resultSummary"`
	Error         *TaskError `json:"error"`
	CreatedAt     string     `json:"createdAt"`
	UpdatedAt     *string    `json:"updatedAt"`
}

type createRequest struct {
	SlaveID string `json:"slaveId"`
	RepoID  string `json:"repoId"`
	Prompt  string `json:"prompt"`
	Model   string `json:"model"`
}

type Store struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	byIdem  map[string]string // Idempotency-Key -> task id
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{
		tasks:  make(map[string]*Task),
		byIdem: make(map[string]string),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Store) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tasks", s.handleCreate)
	mux.HandleFunc("GET /v1/tasks", s.handleList)
	mux.HandleFunc("GET /v1/tasks/{id}", s.handleGet)
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

	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem != "" {
		s.mu.RLock()
		if id, ok := s.byIdem[idem]; ok {
			t := cloneTask(s.tasks[id])
			s.mu.RUnlock()
			writeJSON(w, http.StatusOK, t)
			return
		}
		s.mu.RUnlock()
	}

	now := s.now()
	id, err := newID("tsk")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "id generation failed"})
		return
	}
	created := now.Format(time.RFC3339Nano)
	updated := created
	t := &Task{
		ID:        id,
		Status:    StatusQueued,
		SlaveID:   strPtr(req.SlaveID),
		RepoID:    strPtr(req.RepoID),
		Prompt:    strPtr(req.Prompt),
		Model:     strPtr(req.Model),
		CreatedAt: created,
		UpdatedAt: &updated,
	}

	s.mu.Lock()
	if idem != "" {
		if idExisting, ok := s.byIdem[idem]; ok {
			existing := cloneTask(s.tasks[idExisting])
			s.mu.Unlock()
			writeJSON(w, http.StatusOK, existing)
			return
		}
		s.byIdem[idem] = id
	}
	s.tasks[id] = t
	out := cloneTask(t)
	s.mu.Unlock()

	writeJSON(w, http.StatusCreated, out)
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

func (s *Store) handleCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch t.Status {
	case StatusFinished, StatusError, StatusCancelled:
		writeJSON(w, http.StatusOK, map[string]string{"status": t.Status})
		return
	case StatusCancelling:
		writeJSON(w, http.StatusOK, map[string]string{"status": StatusCancelling})
		return
	default:
		// No executor yet: go straight to cancelled (phase allows this).
		t.Status = StatusCancelled
		u := s.now().Format(time.RFC3339Nano)
		t.UpdatedAt = &u
		writeJSON(w, http.StatusOK, map[string]string{"status": StatusCancelled})
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
