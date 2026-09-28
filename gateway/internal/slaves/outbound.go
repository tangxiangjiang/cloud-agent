// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package slaves

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ws"
)

var slaveUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type slaveClientMsg struct {
	Type    string        `json:"type"`
	Token   string        `json:"token"`
	SlaveID string        `json:"slaveId"`
	Name    string        `json:"name"`
	Repos   []Repo        `json:"repos"`
	TaskID  string        `json:"taskId"`
	Event   *slaveEventIn `json:"event"`
}

type slaveEventIn struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload"`
}

// OutboundHub accepts Slave → Gateway WebSocket connections (slaves dial out).
type OutboundHub struct {
	auth   *auth.Store
	reg    *Registry
	tasks  *task.Store
	appHub *ws.Hub

	mu    sync.Mutex
	conns map[string]*slaveConn // slaveId -> connection
}

func NewOutboundHub(store *auth.Store, reg *Registry, tasks *task.Store, appHub *ws.Hub) *OutboundHub {
	return &OutboundHub{
		auth:   store,
		reg:    reg,
		tasks:  tasks,
		appHub: appHub,
		conns:  make(map[string]*slaveConn),
	}
}

func (h *OutboundHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := slaveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("slave ws upgrade: %v", err)
		return
	}
	c := &slaveConn{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 32),
	}
	if tok := r.URL.Query().Get("token"); tok != "" {
		if !h.auth.ValidToken(tok) {
			_ = conn.WriteJSON(map[string]string{"type": "error", "error": "unauthorized"})
			_ = conn.Close()
			return
		}
		c.authed = true
	}
	go c.writePump()
	c.readPump()
}

// Assign implements task.Dispatcher.
func (h *OutboundHub) Assign(t *task.Task) bool {
	if t == nil || t.SlaveID == nil || *t.SlaveID == "" {
		return false
	}
	h.mu.Lock()
	c := h.conns[*t.SlaveID]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	c.sendJSON(map[string]any{
		"type": "task.assign",
		"task": t,
	})
	return true
}

// RequestCancel implements task.Dispatcher.
func (h *OutboundHub) RequestCancel(taskID, slaveID string) bool {
	h.mu.Lock()
	c := h.conns[slaveID]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	c.sendJSON(map[string]any{
		"type":   "task.cancel",
		"taskId": taskID,
	})
	return true
}

// AssignWorkflow implements workflow.Starter — push DAG run to the target slave.
func (h *OutboundHub) AssignWorkflow(run *workflow.Run) bool {
	if run == nil || run.SlaveID == nil || *run.SlaveID == "" {
		return false
	}
	h.mu.Lock()
	c := h.conns[*run.SlaveID]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	c.sendJSON(map[string]any{
		"type":     "workflow.assign",
		"workflow": run,
	})
	return true
}

// AssignRevise implements workflow.Starter — ask Slave to follow-up on a node.
func (h *OutboundHub) AssignRevise(slaveID, workflowID, nodeID, instruction string) bool {
	if slaveID == "" || workflowID == "" || nodeID == "" {
		return false
	}
	h.mu.Lock()
	c := h.conns[slaveID]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	c.sendJSON(map[string]any{
		"type":        "workflow.revise",
		"workflowId":  workflowID,
		"nodeId":      nodeID,
		"instruction": instruction,
	})
	return true
}

type slaveConn struct {
	hub    *OutboundHub
	conn   *websocket.Conn
	send   chan []byte
	mu     sync.Mutex
	authed bool
	id     string
	once   sync.Once
}

func (c *slaveConn) readPump() {
	defer func() {
		if c.id != "" {
			c.hub.mu.Lock()
			if cur, ok := c.hub.conns[c.id]; ok && cur == c {
				delete(c.hub.conns, c.id)
			}
			c.hub.mu.Unlock()
			c.hub.reg.ScheduleOffline(c.id)
		}
		c.closeSend()
		_ = c.conn.Close()
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		var msg slaveClientMsg
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
		case "register":
			if !c.requireAuth() {
				continue
			}
			if msg.SlaveID == "" {
				c.sendJSON(map[string]string{"type": "error", "error": "slaveId required"})
				continue
			}
			c.hub.reg.UpsertOnline(msg.SlaveID, msg.Name, msg.Repos)
			c.id = msg.SlaveID
			c.hub.mu.Lock()
			c.hub.conns[msg.SlaveID] = c
			c.hub.mu.Unlock()
			c.sendJSON(map[string]any{"type": "registered", "slaveId": msg.SlaveID})
			// claim queued tasks for this slave
			if c.hub.tasks != nil {
				for _, t := range c.hub.tasks.ListQueuedForSlave(msg.SlaveID) {
					c.sendJSON(map[string]any{"type": "task.assign", "task": t})
				}
			}
		case "heartbeat":
			if !c.requireAuth() {
				continue
			}
			if c.id != "" {
				c.hub.reg.Touch(c.id)
			}
			c.sendJSON(map[string]string{"type": "heartbeat.ok"})
		case "task.event":
			if !c.requireAuth() {
				continue
			}
			if msg.TaskID == "" || msg.Event == nil || msg.Event.Kind == "" {
				c.sendJSON(map[string]string{"type": "error", "error": "taskId and event.kind required"})
				continue
			}
			payload := msg.Event.Payload
			if payload == nil {
				payload = map[string]any{}
			}
			if c.hub.tasks != nil {
				c.hub.tasks.ApplySlaveEvent(msg.TaskID, msg.Event.Kind, payload)
			}
			if c.hub.appHub != nil {
				c.hub.appHub.Publish(msg.TaskID, msg.Event.Kind, payload)
			}
		case "ping":
			c.sendJSON(map[string]string{"type": "pong"})
		default:
			c.sendJSON(map[string]string{"type": "error", "error": "unknown type"})
		}
	}
}

func (c *slaveConn) requireAuth() bool {
	c.mu.Lock()
	ok := c.authed
	c.mu.Unlock()
	if !ok {
		c.sendJSON(map[string]string{"type": "error", "error": "unauthorized"})
	}
	return ok
}

func (c *slaveConn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
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

func (c *slaveConn) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func (c *slaveConn) closeSend() {
	c.once.Do(func() { close(c.send) })
}
