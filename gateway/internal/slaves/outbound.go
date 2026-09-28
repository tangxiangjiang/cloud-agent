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
)

var slaveUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type slaveClientMsg struct {
	Type    string `json:"type"`
	Token   string `json:"token"`
	SlaveID string `json:"slaveId"`
	Name    string `json:"name"`
	Repos   []Repo `json:"repos"`
}

// OutboundHub accepts Slave → Gateway WebSocket connections (slaves dial out).
type OutboundHub struct {
	auth *auth.Store
	reg  *Registry
}

func NewOutboundHub(store *auth.Store, reg *Registry) *OutboundHub {
	return &OutboundHub{auth: store, reg: reg}
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
		send: make(chan []byte, 16),
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
			c.sendJSON(map[string]any{"type": "registered", "slaveId": msg.SlaveID})
		case "heartbeat":
			if !c.requireAuth() {
				continue
			}
			if c.id != "" {
				c.hub.reg.Touch(c.id)
			}
			c.sendJSON(map[string]string{"type": "heartbeat.ok"})
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
