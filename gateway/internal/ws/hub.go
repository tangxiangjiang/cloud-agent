// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // personal gateway; tighten later if needed
}

type EventBody struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

type Envelope struct {
	Type   string    `json:"type"`
	TaskID string    `json:"taskId"`
	Seq    int       `json:"seq"`
	At     string    `json:"at"`
	Event  EventBody `json:"event"`
}

type clientMsg struct {
	Type    string `json:"type"`
	Token   string `json:"token"`
	TaskID  string `json:"taskId"`
	LastSeq int    `json:"lastSeq"`
}

type client struct {
	conn *websocket.Conn
	send chan []byte
	hub  *Hub

	mu            sync.Mutex
	closeOnce     sync.Once
	authed        bool
	subscriptions map[string]struct{}
}

// Hub fans out task.event messages to subscribed App WebSocket clients.
type Hub struct {
	auth *auth.Store

	mu      sync.RWMutex
	clients map[*client]struct{}
	seqs    map[string]int
	buf     map[string][]Envelope // per-task history for lastSeq replay
}

func NewHub(store *auth.Store) *Hub {
	return &Hub{
		auth:    store,
		clients: make(map[*client]struct{}),
		seqs:    make(map[string]int),
		buf:     make(map[string][]Envelope),
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	c := &client{
		conn:          conn,
		send:          make(chan []byte, 32),
		hub:           h,
		subscriptions: make(map[string]struct{}),
	}
	// Optional query token (never CURSOR_API_KEY — only gateway bearer).
	if tok := r.URL.Query().Get("token"); tok != "" {
		if h.auth.ValidToken(tok) {
			c.authed = true
		} else {
			_ = conn.WriteJSON(map[string]string{"type": "error", "error": "unauthorized"})
			_ = conn.Close()
			return
		}
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	c.readPump()
}

// Publish appends a task.event with monotonic seq and fans out to subscribers.
func (h *Hub) Publish(taskID, kind string, payload map[string]any) Envelope {
	if payload == nil {
		payload = map[string]any{}
	}
	h.mu.Lock()
	h.seqs[taskID]++
	seq := h.seqs[taskID]
	env := Envelope{
		Type:   "task.event",
		TaskID: taskID,
		Seq:    seq,
		At:     time.Now().UTC().Format(time.RFC3339Nano),
		Event: EventBody{
			Kind:    kind,
			Payload: payload,
		},
	}
	h.buf[taskID] = append(h.buf[taskID], env)
	var targets []*client
	for c := range h.clients {
		c.mu.Lock()
		_, sub := c.subscriptions[taskID]
		authed := c.authed
		c.mu.Unlock()
		if authed && sub {
			targets = append(targets, c)
		}
	}
	h.mu.Unlock()

	data, _ := json.Marshal(env)
	for _, c := range targets {
		select {
		case c.send <- data:
		default:
			// slow client; drop
		}
	}
	return env
}

func (c *client) readPump() {
	defer func() {
		c.hub.remove(c)
		_ = c.conn.Close()
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var msg clientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendJSON(map[string]string{"type": "error", "error": "invalid json"})
			continue
		}
		switch msg.Type {
		case "auth":
			if c.hub.auth.ValidToken(msg.Token) {
				c.mu.Lock()
				c.authed = true
				c.mu.Unlock()
				c.sendJSON(map[string]string{"type": "auth.ok"})
			} else {
				_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_ = c.conn.WriteJSON(map[string]string{"type": "error", "error": "unauthorized"})
				return
			}
		case "subscribe":
			if !c.requireAuth() {
				continue
			}
			if msg.TaskID == "" {
				c.sendJSON(map[string]string{"type": "error", "error": "taskId required"})
				continue
			}
			c.mu.Lock()
			c.subscriptions[msg.TaskID] = struct{}{}
			c.mu.Unlock()
			c.hub.replay(c, msg.TaskID, msg.LastSeq)
			c.sendJSON(map[string]any{"type": "subscribed", "taskId": msg.TaskID})
		case "unsubscribe":
			if !c.requireAuth() {
				continue
			}
			c.mu.Lock()
			delete(c.subscriptions, msg.TaskID)
			c.mu.Unlock()
			c.sendJSON(map[string]any{"type": "unsubscribed", "taskId": msg.TaskID})
		case "ping":
			c.sendJSON(map[string]string{"type": "pong"})
		default:
			c.sendJSON(map[string]string{"type": "error", "error": "unknown type"})
		}
	}
}

func (c *client) requireAuth() bool {
	c.mu.Lock()
	ok := c.authed
	c.mu.Unlock()
	if !ok {
		c.sendJSON(map[string]string{"type": "error", "error": "unauthorized"})
	}
	return ok
}

func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.closeOnce.Do(func() { close(c.send) })
}

func (h *Hub) replay(c *client, taskID string, lastSeq int) {
	h.mu.RLock()
	hist := append([]Envelope(nil), h.buf[taskID]...)
	h.mu.RUnlock()
	for _, env := range hist {
		if env.Seq <= lastSeq {
			continue
		}
		data, _ := json.Marshal(env)
		select {
		case c.send <- data:
		default:
		}
	}
}

// EventsAfter returns buffered task.event envelopes with seq > afterSeq, plus latestSeq.
func (h *Hub) EventsAfter(taskID string, afterSeq int) (events []Envelope, latestSeq int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	latestSeq = h.seqs[taskID]
	for _, env := range h.buf[taskID] {
		if env.Seq > afterSeq {
			events = append(events, env)
		}
	}
	if events == nil {
		events = []Envelope{}
	}
	return events, latestSeq
}

// EventsAfterMaps implements task.EventSource for HTTP snapshots.
func (h *Hub) EventsAfterMaps(taskID string, afterSeq int) ([]map[string]any, int) {
	envs, latest := h.EventsAfter(taskID, afterSeq)
	out := make([]map[string]any, 0, len(envs))
	for _, env := range envs {
		raw, err := json.Marshal(env)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, latest
}
