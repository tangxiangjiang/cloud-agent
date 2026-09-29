// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Package chat implements Gateway ChatSession → Task bridge (M09-P01).
package chat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

const (
	ModeAgent = "agent"
	ModeAsk   = "ask"
	ModePlan  = "plan"

	StatusIdle    = "idle"
	StatusRunning = "running"
	StatusError   = "error"
)

// Message is a short transcript entry (full stream stays on task events).
type Message struct {
	ID      string  `json:"id"`
	Role    string  `json:"role"`
	Content string  `json:"content"`
	TaskID  *string `json:"taskId,omitempty"`
	Mode    string  `json:"mode,omitempty"`
	Model   string  `json:"model,omitempty"`
	At      string  `json:"at"`
}

// Session binds a project chat to slave/repo.
type Session struct {
	ID        string    `json:"id"`
	SlaveID   string    `json:"slaveId"`
	RepoID    string    `json:"repoId"`
	Mode      string    `json:"mode"`
	Model     string    `json:"model"`
	Status    string    `json:"status"`
	Messages  []Message `json:"messages"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
}

type createSessionRequest struct {
	SlaveID string `json:"slaveId"`
	RepoID  string `json:"repoId"`
	Mode    string `json:"mode"`
	Model   string `json:"model"`
}

type postMessageRequest struct {
	Text  string `json:"text"`
	Mode  string `json:"mode"`
	Model string `json:"model"`
}

// Store is process-local chat sessions (SQLite persistence is M09-P04).
type Store struct {
	mu    sync.RWMutex
	chats map[string]*Session
	now   func() time.Time
	tasks *task.Store
}

func NewStore(tasks *task.Store) *Store {
	return &Store{
		chats: map[string]*Session{},
		now:   func() time.Time { return time.Now().UTC() },
		tasks: tasks,
	}
}

func (s *Store) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chats", s.handleCreate)
	mux.HandleFunc("GET /v1/chats", s.handleList)
	mux.HandleFunc("GET /v1/chats/{id}", s.handleGet)
	mux.HandleFunc("POST /v1/chats/{id}/messages", s.handleMessage)
	mux.HandleFunc("POST /v1/chats/{id}/stop", s.handleStop)
	return mux
}

func normalizeMode(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case ModeAsk:
		return ModeAsk
	case ModePlan:
		return ModePlan
	default:
		return ModeAgent
	}
}

func normalizeModel(m string) string {
	v := strings.TrimSpace(m)
	if v == "" {
		return "auto"
	}
	return v
}

// BuildPrompt injects mode guidance; never includes secrets / cwd from App.
func BuildPrompt(mode, userText string) string {
	text := strings.TrimSpace(userText)
	switch normalizeMode(mode) {
	case ModeAsk:
		return "[Mode: Ask — READ-ONLY. Do not write, edit, delete, or create files; do not run mutating git commands; answer from inspection only.]\n\n" + text
	case ModePlan:
		return "[Mode: Plan — Produce a structured plan with clear steps. Prefer not to modify the workspace unless the user explicitly asks to execute the plan.]\n\n" + text
	default:
		return text
	}
}

func (s *Store) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	slaveID := strings.TrimSpace(req.SlaveID)
	repoID := strings.TrimSpace(req.RepoID)
	if slaveID == "" || repoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slaveId and repoId required"})
		return
	}
	id, err := newID("chat")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "id generation failed"})
		return
	}
	now := s.now().Format(time.RFC3339Nano)
	sess := &Session{
		ID:        id,
		SlaveID:   slaveID,
		RepoID:    repoID,
		Mode:      normalizeMode(req.Mode),
		Model:     normalizeModel(req.Model),
		Status:    StatusIdle,
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.mu.Lock()
	s.chats[id] = sess
	out := cloneSession(sess)
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, out)
}

func (s *Store) handleList(w http.ResponseWriter, r *http.Request) {
	slaveID := strings.TrimSpace(r.URL.Query().Get("slaveId"))
	repoID := strings.TrimSpace(r.URL.Query().Get("repoId"))
	s.mu.RLock()
	list := make([]*Session, 0, len(s.chats))
	for _, c := range s.chats {
		if slaveID != "" && c.SlaveID != slaveID {
			continue
		}
		if repoID != "" && c.RepoID != repoID {
			continue
		}
		list = append(list, cloneSession(c))
	}
	s.mu.RUnlock()
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt > list[j].UpdatedAt
	})
	writeJSON(w, http.StatusOK, map[string]any{"chats": list})
}

func (s *Store) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	c, ok := s.chats[id]
	var out *Session
	if ok {
		out = cloneSession(c)
	}
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Store) handleMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text required"})
		return
	}
	if s.tasks == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "task store unavailable"})
		return
	}

	s.mu.Lock()
	sess, ok := s.chats[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	mode := sess.Mode
	if strings.TrimSpace(req.Mode) != "" {
		mode = normalizeMode(req.Mode)
		sess.Mode = mode
	}
	model := sess.Model
	if strings.TrimSpace(req.Model) != "" {
		model = normalizeModel(req.Model)
		sess.Model = model
	}
	prompt := BuildPrompt(mode, text)
	msgID, err := newID("msg")
	if err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "id generation failed"})
		return
	}
	now := s.now().Format(time.RFC3339Nano)
	sess.Messages = append(sess.Messages, Message{
		ID:      msgID,
		Role:    "user",
		Content: text,
		Mode:    mode,
		Model:   model,
		At:      now,
	})
	sess.Status = StatusRunning
	sess.UpdatedAt = now
	slaveID := sess.SlaveID
	repoID := sess.RepoID
	s.mu.Unlock()

	tsk, _, err := s.tasks.Create(task.CreateInput{
		SlaveID: slaveID,
		RepoID:  repoID,
		Prompt:  prompt,
		Model:   model,
		ChatID:  id,
		Mode:    mode,
	})
	if err != nil || tsk == nil {
		s.mu.Lock()
		if c, ok := s.chats[id]; ok {
			c.Status = StatusError
			c.UpdatedAt = s.now().Format(time.RFC3339Nano)
		}
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create task failed"})
		return
	}

	s.mu.Lock()
	if c, ok := s.chats[id]; ok {
		if n := len(c.Messages); n > 0 {
			tid := tsk.ID
			c.Messages[n-1].TaskID = &tid
		}
		c.UpdatedAt = s.now().Format(time.RFC3339Nano)
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"taskId": tsk.ID,
		"chatId": id,
		"mode":   mode,
		"model":  model,
	})
}

func (s *Store) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	sess, ok := s.chats[id]
	taskID := ""
	if ok {
		for i := len(sess.Messages) - 1; i >= 0; i-- {
			if sess.Messages[i].TaskID != nil && *sess.Messages[i].TaskID != "" {
				taskID = *sess.Messages[i].TaskID
				break
			}
		}
	}
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if taskID == "" || s.tasks == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": StatusIdle})
		return
	}
	st, code := s.tasks.Cancel(taskID)
	if code == http.StatusNotFound {
		writeJSON(w, http.StatusOK, map[string]string{"status": StatusIdle})
		return
	}
	s.mu.Lock()
	if c, ok := s.chats[id]; ok {
		c.Status = StatusIdle
		c.UpdatedAt = s.now().Format(time.RFC3339Nano)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": st, "taskId": taskID})
}

func cloneSession(c *Session) *Session {
	if c == nil {
		return nil
	}
	cp := *c
	if c.Messages != nil {
		cp.Messages = append([]Message(nil), c.Messages...)
		for i := range cp.Messages {
			if c.Messages[i].TaskID != nil {
				v := *c.Messages[i].TaskID
				cp.Messages[i].TaskID = &v
			}
		}
	} else {
		cp.Messages = []Message{}
	}
	return &cp
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
