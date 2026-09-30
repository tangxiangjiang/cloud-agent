// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package masters

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
)

var masterUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type masterClientMsg struct {
	Type     string          `json:"type"`
	Token    string          `json:"token"`
	MasterID string          `json:"masterId"`
	Name     string          `json:"name"`
	Slaves   []SlaveReport   `json:"slaves"`
	RequestID string         `json:"requestId"`
	Error    string          `json:"error"`
	Code     string          `json:"code"`
	Payload  json.RawMessage `json:"payload"`
}

// OutboundHub accepts Master → Gateway control-plane WebSocket connections.
type OutboundHub struct {
	auth *auth.Store
	reg  *Registry
	pend *Pending

	mu    sync.Mutex
	conns map[string]*masterConn
}

func NewOutboundHub(store *auth.Store, reg *Registry) *OutboundHub {
	return &OutboundHub{
		auth:  store,
		reg:   reg,
		pend:  NewPending(),
		conns: make(map[string]*masterConn),
	}
}

func (h *OutboundHub) Pending() *Pending { return h.pend }

func (h *OutboundHub) IsConnected(masterID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conns[masterID] != nil
}

func (h *OutboundHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := masterUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("master ws upgrade: %v", err)
		return
	}
	c := &masterConn{
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

// SendJSON delivers a control message to an online Master. false if no conn.
func (h *OutboundHub) SendJSON(masterID string, msg map[string]any) bool {
	h.mu.Lock()
	c := h.conns[masterID]
	h.mu.Unlock()
	if c == nil {
		return false
	}
	c.sendJSON(msg)
	return true
}

type masterConn struct {
	hub    *OutboundHub
	conn   *websocket.Conn
	send   chan []byte
	mu     sync.Mutex
	authed bool
	id     string
	once   sync.Once
}

func (c *masterConn) readPump() {
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
		var msg masterClientMsg
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
		case "master.register", "register":
			if !c.requireAuth() {
				continue
			}
			if msg.MasterID == "" {
				c.sendJSON(map[string]string{"type": "error", "error": "masterId required"})
				continue
			}
			c.hub.reg.UpsertRegister(msg.MasterID, msg.Name, msg.Slaves)
			c.id = msg.MasterID
			c.hub.mu.Lock()
			c.hub.conns[msg.MasterID] = c
			c.hub.mu.Unlock()
			c.sendJSON(map[string]any{"type": "master.registered", "masterId": msg.MasterID})
			log.Printf("master registered: %s (%d slaves)", msg.MasterID, len(msg.Slaves))
		case "master.heartbeat", "heartbeat":
			if !c.requireAuth() {
				continue
			}
			if c.id != "" {
				c.hub.reg.Touch(c.id)
			}
		case "master.slaves.report":
			if !c.requireAuth() {
				continue
			}
			id := msg.MasterID
			if id == "" {
				id = c.id
			}
			if id != "" {
				c.hub.reg.ApplyReports(id, msg.Slaves)
			}
		case "master.config.ok", "master.control.ok":
			if !c.requireAuth() {
				continue
			}
			c.hub.pend.Complete(msg.RequestID, true, "", "")
		case "master.config.error", "master.control.error":
			if !c.requireAuth() {
				continue
			}
			c.hub.pend.Complete(msg.RequestID, false, msg.Error, msg.Code)
		case "ping":
			c.sendJSON(map[string]string{"type": "pong"})
		default:
			// ignore unknown
		}
	}
}

func (c *masterConn) requireAuth() bool {
	c.mu.Lock()
	ok := c.authed
	c.mu.Unlock()
	if !ok {
		c.sendJSON(map[string]string{"type": "error", "error": "unauthorized"})
	}
	return ok
}

func (c *masterConn) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		log.Printf("master ws send buffer full id=%s", c.id)
	}
}

func (c *masterConn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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

func (c *masterConn) closeSend() {
	c.once.Do(func() { close(c.send) })
}
