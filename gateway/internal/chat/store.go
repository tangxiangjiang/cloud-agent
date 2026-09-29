// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Package chat implements Gateway ChatSession → Task bridge (M09).
package chat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

const (
	ModeAgent = "agent"
	ModeAsk   = "ask"
	ModePlan  = "plan"

	StatusIdle    = "idle"
	StatusRunning = "running"
	StatusError   = "error"

	// Truncation policy (documented in project-chat.md) — prevent DB bloat.
	// Never persist tool raw payloads / secrets; only role/content text.
	MaxMessagesPerSession = 80
	MaxMessageChars       = 4000
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
	ID           string    `json:"id"`
	SlaveID      string    `json:"slaveId"`
	RepoID       string    `json:"repoId"`
	Mode         string    `json:"mode"`
	Model        string    `json:"model"`
	Status       string    `json:"status"`
	Messages     []Message `json:"messages"`
	CreatedAt    string    `json:"createdAt"`
	UpdatedAt    string    `json:"updatedAt"`
	Preview      string    `json:"preview,omitempty"`
	MessageCount int       `json:"messageCount,omitempty"`
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

type assistantSummaryRequest struct {
	TaskID  string `json:"taskId"`
	Content string `json:"content"`
}

// Store holds chat sessions; optional SQLite via persist.ChatSessionStore (M09-P04).
type Store struct {
	mu      sync.RWMutex
	chats   map[string]*Session
	now     func() time.Time
	tasks   *task.Store
	persist persist.ChatSessionStore
}

func NewStore(tasks *task.Store) *Store {
	return &Store{
		chats: map[string]*Session{},
		now:   func() time.Time { return time.Now().UTC() },
		tasks: tasks,
	}
}

// SetPersist wires durable storage (SQLite or memory).
func (s *Store) SetPersist(p persist.ChatSessionStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persist = p
}

// LoadFromPersist hydrates in-memory map from durable store (call at Gateway start).
func (s *Store) LoadFromPersist() error {
	s.mu.RLock()
	p := s.persist
	s.mu.RUnlock()
	if p == nil {
		return nil
	}
	rows, err := p.LoadAllChatSessions()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range rows {
		sess, err := sessionFromRow(row)
		if err != nil {
			log.Printf("chat restore skip %s: %v", row.ID, err)
			continue
		}
		s.chats[sess.ID] = sess
	}
	return nil
}

func (s *Store) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chats", s.handleCreate)
	mux.HandleFunc("GET /v1/chats", s.handleList)
	mux.HandleFunc("GET /v1/chats/{id}", s.handleGet)
	mux.HandleFunc("POST /v1/chats/{id}/messages", s.handleMessage)
	mux.HandleFunc("POST /v1/chats/{id}/assistant", s.handleAssistant)
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

// TruncateContent cuts to MaxMessageChars on rune boundaries.
func TruncateContent(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= MaxMessageChars {
		return s
	}
	runes := []rune(s)
	return string(runes[:MaxMessageChars]) + "…"
}

func truncateMessages(msgs []Message) []Message {
	for i := range msgs {
		msgs[i].Content = TruncateContent(msgs[i].Content)
	}
	if len(msgs) <= MaxMessagesPerSession {
		return msgs
	}
	return msgs[len(msgs)-MaxMessagesPerSession:]
}

func previewFrom(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && strings.TrimSpace(msgs[i].Content) != "" {
			c := msgs[i].Content
			if utf8.RuneCountInString(c) > 120 {
				return string([]rune(c)[:120]) + "…"
			}
			return c
		}
	}
	return ""
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
	out := cloneSession(sess, true)
	s.persistLocked(sess)
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
		list = append(list, cloneSession(c, false)) // summary: no full messages
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
		out = cloneSession(c, true)
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
	text := TruncateContent(req.Text)
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
	sess.Messages = truncateMessages(sess.Messages)
	sess.Status = StatusRunning
	sess.UpdatedAt = now
	slaveID := sess.SlaveID
	repoID := sess.RepoID
	s.persistLocked(sess)
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
			s.persistLocked(c)
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
		s.persistLocked(c)
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"taskId": tsk.ID,
		"chatId": id,
		"mode":   mode,
		"model":  model,
	})
}

// handleAssistant records a truncated assistant summary after a turn (no tool payloads).
func (s *Store) handleAssistant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req assistantSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	content := TruncateContent(req.Content)
	if content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}
	s.mu.Lock()
	sess, ok := s.chats[id]
	if !ok {
		s.mu.Unlock()
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	msgID, err := newID("msg")
	if err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "id generation failed"})
		return
	}
	now := s.now().Format(time.RFC3339Nano)
	var tid *string
	if v := strings.TrimSpace(req.TaskID); v != "" {
		tid = &v
	}
	sess.Messages = append(sess.Messages, Message{
		ID:      msgID,
		Role:    "assistant",
		Content: content,
		TaskID:  tid,
		Mode:    sess.Mode,
		Model:   sess.Model,
		At:      now,
	})
	sess.Messages = truncateMessages(sess.Messages)
	sess.Status = StatusIdle
	sess.UpdatedAt = now
	out := cloneSession(sess, true)
	s.persistLocked(sess)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
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
		s.persistLocked(c)
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": st, "taskId": taskID})
}

// persistLocked writes session; caller must hold s.mu.
func (s *Store) persistLocked(sess *Session) {
	if s.persist == nil || sess == nil {
		return
	}
	msgs := truncateMessages(append([]Message(nil), sess.Messages...))
	body, err := json.Marshal(msgs)
	if err != nil {
		log.Printf("chat persist marshal %s: %v", sess.ID, err)
		return
	}
	row := persist.ChatSessionRow{
		ID:        sess.ID,
		SlaveID:   sess.SlaveID,
		RepoID:    sess.RepoID,
		Mode:      sess.Mode,
		Model:     sess.Model,
		Status:    sess.Status,
		Messages:  body,
		CreatedAt: sess.CreatedAt,
		UpdatedAt: sess.UpdatedAt,
	}
	if err := s.persist.UpsertChatSession(row); err != nil {
		log.Printf("chat persist %s: %v", sess.ID, err)
	}
}

func sessionFromRow(row persist.ChatSessionRow) (*Session, error) {
	var msgs []Message
	if len(row.Messages) > 0 {
		if err := json.Unmarshal(row.Messages, &msgs); err != nil {
			return nil, err
		}
	}
	if msgs == nil {
		msgs = []Message{}
	}
	msgs = truncateMessages(msgs)
	return &Session{
		ID:        row.ID,
		SlaveID:   row.SlaveID,
		RepoID:    row.RepoID,
		Mode:      row.Mode,
		Model:     row.Model,
		Status:    row.Status,
		Messages:  msgs,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func cloneSession(c *Session, withMessages bool) *Session {
	if c == nil {
		return nil
	}
	cp := *c
	cp.MessageCount = len(c.Messages)
	cp.Preview = previewFrom(c.Messages)
	if withMessages {
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
