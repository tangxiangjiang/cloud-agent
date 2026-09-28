// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package slaves_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
)

func TestListFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
slaves:
  - id: slave_devpc
    name: "dev machine"
    online: true
    repos:
      - id: r_cloud_agent
        name: cloud-agent
        cwd: /path/to/your/cloud-agent
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := slaves.LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reg := slaves.NewRegistry(cfg.Slaves)

	req := httptest.NewRequest(http.MethodGet, "/v1/slaves", nil)
	rec := httptest.NewRecorder()
	reg.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var resp struct {
		Slaves []slaves.Slave `json:"slaves"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Slaves) != 1 {
		t.Fatalf("len=%d", len(resp.Slaves))
	}
	s := resp.Slaves[0]
	// config online ignored until outbound register
	if s.ID != "slave_devpc" || s.Online != false || len(s.Repos) != 1 {
		t.Fatalf("unexpected slave: %+v", s)
	}
	if s.Repos[0].ID != "r_cloud_agent" || s.Repos[0].Cwd != "/path/to/your/cloud-agent" {
		t.Fatalf("unexpected repo: %+v", s.Repos[0])
	}
}

func TestEmptyWithoutConfig(t *testing.T) {
	reg := slaves.NewRegistry(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/slaves", nil)
	rec := httptest.NewRecorder()
	reg.HandleList(rec, req)
	var resp struct {
		Slaves []slaves.Slave `json:"slaves"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Slaves == nil || len(resp.Slaves) != 0 {
		t.Fatalf("want empty list, got %#v", resp.Slaves)
	}
}

func TestOutboundRegisterHeartbeatAndOffline(t *testing.T) {
	store := auth.NewStore("PAIR")
	reg := slaves.NewRegistry(nil)
	reg.SetGrace(200 * time.Millisecond)
	hub := slaves.NewOutboundHub(store, reg, nil, nil)

	pairRec := httptest.NewRecorder()
	store.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pair struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(pairRec.Body.Bytes(), &pair)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	read := func() map[string]any {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}

	if err := conn.WriteJSON(map[string]string{"type": "auth", "token": pair.Token}); err != nil {
		t.Fatal(err)
	}
	if m := read(); m["type"] != "auth.ok" {
		t.Fatalf("auth: %v", m)
	}

	if err := conn.WriteJSON(map[string]any{
		"type":    "register",
		"slaveId": "slave_devpc",
		"name":    "dev",
		"repos": []map[string]string{
			{"id": "r1", "name": "repo", "cwd": "/tmp/repo"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if m := read(); m["type"] != "registered" {
		t.Fatalf("register: %v", m)
	}
	if !reg.IsOnline("slave_devpc") {
		t.Fatal("expected online after register")
	}

	if err := conn.WriteJSON(map[string]string{"type": "heartbeat"}); err != nil {
		t.Fatal(err)
	}
	if m := read(); m["type"] != "heartbeat.ok" {
		t.Fatalf("heartbeat: %v", m)
	}

	_ = conn.Close()
	time.Sleep(400 * time.Millisecond)
	if reg.IsOnline("slave_devpc") {
		t.Fatal("expected offline after grace")
	}
}
