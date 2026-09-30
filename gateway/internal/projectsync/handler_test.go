// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package projectsync_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/audit"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/projectsync"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
)

type fakeHub struct {
	lastSlave string
	lastReq   string
	lastRepo  string
	lastDoc   string
	lastPh    []string
	ok        bool
}

func (f *fakeHub) AssignProjectSync(slaveID, requestID, repoID string) bool {
	f.lastSlave, f.lastReq, f.lastRepo = slaveID, requestID, repoID
	return f.ok
}

func (f *fakeHub) AssignProgressAlign(slaveID, requestID, repoID, progressDoc string, phases []string) bool {
	f.lastSlave, f.lastReq, f.lastRepo = slaveID, requestID, repoID
	f.lastDoc = progressDoc
	f.lastPh = append([]string(nil), phases...)
	return f.ok
}

func TestTriggerOffline(t *testing.T) {
	reg := slaves.NewRegistry(nil)
	store := persist.NewMemoryProjectSync()
	svc := projectsync.NewService(reg, &fakeHub{ok: true}, store, audit.NewLogger(100, nil))
	mux := http.NewServeMux()
	svc.Mount(mux, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/slaves/slave_x/projects/r1/sync", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTriggerAndReportRoundTrip(t *testing.T) {
	reg := slaves.NewRegistry(nil)
	reg.UpsertOnline("slave_devpc", "dev", nil, []slaves.Project{{
		ID: "r_flutter", Name: "flutter", Cwd: "/tmp/x",
	}})
	hub := &fakeHub{ok: true}
	store := persist.NewMemoryProjectSync()
	svc := projectsync.NewService(reg, hub, store, audit.NewLogger(100, nil))
	mux := http.NewServeMux()
	svc.Mount(mux, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/slaves/slave_devpc/projects/r_flutter/sync",
		strings.NewReader(`{"requestId":"req_test"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("trigger status=%d body=%s", rec.Code, rec.Body.String())
	}
	if hub.lastSlave != "slave_devpc" || hub.lastRepo != "r_flutter" || hub.lastReq != "req_test" {
		t.Fatalf("hub call: %+v", hub)
	}

	payload := map[string]any{
		"schemaVersion": 1,
		"branch":        "main",
		"head":          "abc",
		"dirty":         false,
		"syncedAt":      time.Now().UTC().Format(time.RFC3339Nano),
		"summary":       "ok",
	}
	body, _ := json.Marshal(map[string]any{
		"requestId": "req_test",
		"slaveId":   "slave_devpc",
		"repoId":    "r_flutter",
		"payload":   payload,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/v1/project-sync", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("report status=%d body=%s", rec2.Code, rec2.Body.String())
	}

	req3 := httptest.NewRequest(http.MethodGet, "/v1/slaves/slave_devpc/projects/r_flutter/sync", nil)
	rec3 := httptest.NewRecorder()
	mux.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", rec3.Code, rec3.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec3.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	pl, ok := got["payload"].(map[string]any)
	if !ok || pl["branch"] != "main" {
		t.Fatalf("payload: %#v", got["payload"])
	}
}

func TestSQLiteProjectSyncPersist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	st, err := persist.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	payload, _ := json.Marshal(map[string]any{"branch": "feat", "dirty": true})
	if err := st.UpsertProjectSync(persist.ProjectSyncRow{
		SlaveID: "s1", RepoID: "r1", SyncedAt: "2026-09-29T00:00:00Z", SummaryJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
	row, err := st.GetProjectSync("s1", "r1")
	if err != nil || row == nil {
		t.Fatalf("get: %v %#v", err, row)
	}
	var m map[string]any
	_ = json.Unmarshal(row.SummaryJSON, &m)
	if m["branch"] != "feat" {
		t.Fatalf("json: %s", row.SummaryJSON)
	}
}

func TestTriggerRequiresConnEvenIfOnline(t *testing.T) {
	reg := slaves.NewRegistry(nil)
	reg.UpsertOnline("slave_devpc", "dev", []slaves.Repo{{ID: "r1", Name: "n", Cwd: "/x"}}, nil)
	hub := &fakeHub{ok: false}
	svc := projectsync.NewService(reg, hub, persist.NewMemoryProjectSync(), nil)
	mux := http.NewServeMux()
	svc.Mount(mux, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/slaves/slave_devpc/projects/r1/sync", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestOutboundAssignProjectSyncWS(t *testing.T) {
	store := auth.NewStore("PAIR")
	reg := slaves.NewRegistry(nil)
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
	defer conn.Close()

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

	_ = conn.WriteJSON(map[string]string{"type": "auth", "token": pair.Token})
	if m := read(); m["type"] != "auth.ok" {
		t.Fatalf("auth: %v", m)
	}
	_ = conn.WriteJSON(map[string]any{
		"type": "register", "slaveId": "slave_devpc", "name": "dev",
		"projects": []map[string]string{{"id": "r1", "name": "n", "cwd": "/tmp"}},
	})
	if m := read(); m["type"] != "registered" {
		t.Fatalf("reg: %v", m)
	}

	if !hub.AssignProjectSync("slave_devpc", "req_1", "r1") {
		t.Fatal("assign failed")
	}
	m := read()
	if m["type"] != "project.sync" || m["repoId"] != "r1" || m["requestId"] != "req_1" {
		t.Fatalf("msg: %v", m)
	}
}
