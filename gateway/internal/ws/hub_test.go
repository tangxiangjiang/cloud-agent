// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ws"
)

func TestWSAuthSubscribePingAndEvents(t *testing.T) {
	store := auth.NewStore("PAIR")
	hub := ws.NewHub(store)

	// issue token via pair HTTP
	pairBody := strings.NewReader(`{"pairCode":"PAIR"}`)
	pairReq := httptest.NewRequest(http.MethodPost, "/v1/auth/pair", pairBody)
	pairRec := httptest.NewRecorder()
	store.HandlePair(pairRec, pairReq)
	if pairRec.Code != http.StatusOK {
		t.Fatalf("pair: %d %s", pairRec.Code, pairRec.Body.String())
	}
	var pairResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(pairRec.Body.Bytes(), &pairResp)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	t.Run("bad token rejected", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?token=bad", nil)
		if err != nil {
			// some stacks close during handshake; either way must not stay open usefully
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err == nil {
			var m map[string]string
			_ = json.Unmarshal(msg, &m)
			if m["error"] != "unauthorized" && m["type"] != "error" {
				t.Fatalf("unexpected msg: %s", msg)
			}
		}
	})

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mustWrite := func(v any) {
		t.Helper()
		if err := conn.WriteJSON(v); err != nil {
			t.Fatal(err)
		}
	}
	readJSON := func() map[string]any {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	mustWrite(map[string]string{"type": "auth", "token": "nope"})
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err == nil {
		var errMsg map[string]any
		_ = json.Unmarshal(data, &errMsg)
		if errMsg["type"] != "error" {
			t.Fatalf("want auth error, got %v", errMsg)
		}
	}
	_ = conn.Close()

	// reconnect after auth failure closed the socket
	conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	mustWrite(map[string]string{"type": "auth", "token": pairResp.Token})
	m := readJSON()
	if m["type"] != "auth.ok" {
		t.Fatalf("want auth.ok, got %v", m)
	}

	mustWrite(map[string]any{"type": "subscribe", "taskId": "tsk_1", "lastSeq": 0})
	// may receive subscribed and/or replayed events; drain until subscribed
	gotSub := false
	for i := 0; i < 5; i++ {
		m = readJSON()
		if m["type"] == "subscribed" {
			gotSub = true
			break
		}
	}
	if !gotSub {
		t.Fatalf("missing subscribed")
	}

	mustWrite(map[string]string{"type": "ping"})
	m = readJSON()
	if m["type"] != "pong" {
		t.Fatalf("want pong, got %v", m)
	}

	hub.Publish("tsk_1", "status", map[string]any{"status": "running"})
	hub.Publish("tsk_1", "assistant.delta", map[string]any{"text": "hi"})

	var seqs []float64
	for len(seqs) < 2 {
		m = readJSON()
		if m["type"] != "task.event" {
			continue
		}
		seqs = append(seqs, m["seq"].(float64))
		if int(m["seq"].(float64)) != len(seqs) {
			t.Fatalf("seq want %d got %v", len(seqs), m["seq"])
		}
	}
	if seqs[0] != 1 || seqs[1] != 2 {
		t.Fatalf("seqs=%v", seqs)
	}
}
