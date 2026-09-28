// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package slaves_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ws"
)

func TestDispatchFanoutAndCancel(t *testing.T) {
	authStore := auth.NewStore("PAIR")
	tasks := task.NewStore()
	appHub := ws.NewHub(authStore)
	tasks.SetEventSource(appHub)
	reg := slaves.NewRegistry(nil)
	slaveHub := slaves.NewOutboundHub(authStore, reg, tasks, appHub)
	tasks.SetDispatcher(slaveHub)

	pairRec := httptest.NewRecorder()
	authStore.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pair struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(pairRec.Body.Bytes(), &pair)

	slaveSrv := httptest.NewServer(http.HandlerFunc(slaveHub.HandleWS))
	defer slaveSrv.Close()
	appSrv := httptest.NewServer(http.HandlerFunc(appHub.HandleWS))
	defer appSrv.Close()

	slaveWS := "ws" + strings.TrimPrefix(slaveSrv.URL, "http")
	appWS := "ws" + strings.TrimPrefix(appSrv.URL, "http")

	// mock slave connection
	sconn, _, err := websocket.DefaultDialer.Dial(slaveWS, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sconn.Close()

	sread := func() map[string]any {
		t.Helper()
		_ = sconn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := sconn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	_ = sconn.WriteJSON(map[string]string{"type": "auth", "token": pair.Token})
	if sread()["type"] != "auth.ok" {
		t.Fatal("slave auth")
	}
	_ = sconn.WriteJSON(map[string]any{
		"type": "register", "slaveId": "slave_devpc", "name": "mock",
		"repos": []map[string]string{{"id": "r1", "name": "r", "cwd": "/tmp"}},
	})
	if sread()["type"] != "registered" {
		t.Fatal("slave register")
	}

	// app subscriber
	aconn, _, err := websocket.DefaultDialer.Dial(appWS, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer aconn.Close()
	aread := func() map[string]any {
		t.Helper()
		_ = aconn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := aconn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	_ = aconn.WriteJSON(map[string]string{"type": "auth", "token": pair.Token})
	if aread()["type"] != "auth.ok" {
		t.Fatal("app auth")
	}

	// create task via HTTP handler
	th := tasks.Handler()
	body := []byte(`{"slaveId":"slave_devpc","repoId":"r1","prompt":"hi","model":"composer-2.5"}`)
	creq := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(body))
	crec := httptest.NewRecorder()
	th.ServeHTTP(crec, creq)
	if crec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", crec.Code, crec.Body.String())
	}
	var created task.Task
	_ = json.Unmarshal(crec.Body.Bytes(), &created)

	_ = aconn.WriteJSON(map[string]any{"type": "subscribe", "taskId": created.ID, "lastSeq": 0})
	for i := 0; i < 5; i++ {
		if aread()["type"] == "subscribed" {
			break
		}
	}

	// slave should receive assign
	var assign map[string]any
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m := sread()
		if m["type"] == "task.assign" {
			assign = m
			break
		}
	}
	if assign == nil {
		t.Fatal("slave did not receive task.assign")
	}

	// mock slave emits events
	emit := func(kind string, payload map[string]any) {
		_ = sconn.WriteJSON(map[string]any{
			"type": "task.event", "taskId": created.ID,
			"event": map[string]any{"kind": kind, "payload": payload},
		})
	}
	emit("status", map[string]any{"status": "running"})
	emit("assistant.delta", map[string]any{"text": "hello"})
	emit("done", map[string]any{"status": "finished"})

	gotKinds := map[string]bool{}
	for i := 0; i < 10 && len(gotKinds) < 3; i++ {
		m := aread()
		if m["type"] != "task.event" {
			continue
		}
		ev := m["event"].(map[string]any)
		gotKinds[ev["kind"].(string)] = true
		if m["seq"].(float64) < 1 {
			t.Fatalf("bad seq %v", m["seq"])
		}
	}
	if !gotKinds["status"] || !gotKinds["assistant.delta"] || !gotKinds["done"] {
		t.Fatalf("app missing events: %v", gotKinds)
	}

	// events HTTP snapshot
	ereq := httptest.NewRequest(http.MethodGet, "/v1/tasks/"+created.ID+"/events?afterSeq=0", nil)
	erec := httptest.NewRecorder()
	th.ServeHTTP(erec, ereq)
	if erec.Code != http.StatusOK {
		t.Fatalf("events %d", erec.Code)
	}
	var snap struct {
		Events    []map[string]any `json:"events"`
		LatestSeq int              `json:"latestSeq"`
	}
	_ = json.Unmarshal(erec.Body.Bytes(), &snap)
	if snap.LatestSeq < 3 || len(snap.Events) < 3 {
		t.Fatalf("snapshot %#v", snap)
	}

	// cancel path with a new task
	body2 := []byte(`{"slaveId":"slave_devpc","repoId":"r1","prompt":"c","model":"composer-2.5"}`)
	creq2 := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(body2))
	crec2 := httptest.NewRecorder()
	th.ServeHTTP(crec2, creq2)
	var t2 task.Task
	_ = json.Unmarshal(crec2.Body.Bytes(), &t2)

	// drain assign
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m := sread()
		if m["type"] == "task.assign" {
			break
		}
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/tasks/"+t2.ID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	th.ServeHTTP(cancelRec, cancelReq)
	var cancelResp struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(cancelRec.Body.Bytes(), &cancelResp)
	if cancelResp.Status != task.StatusCancelling {
		t.Fatalf("cancel status=%s want cancelling", cancelResp.Status)
	}

	deadline = time.Now().Add(3 * time.Second)
	gotCancel := false
	for time.Now().Before(deadline) {
		m := sread()
		if m["type"] == "task.cancel" && m["taskId"] == t2.ID {
			gotCancel = true
			break
		}
	}
	if !gotCancel {
		t.Fatal("slave did not receive task.cancel")
	}
	_ = sconn.WriteJSON(map[string]any{
		"type": "task.event", "taskId": t2.ID,
		"event": map[string]any{"kind": "done", "payload": map[string]any{"status": "cancelled"}},
	})
	time.Sleep(50 * time.Millisecond)
	getReq := httptest.NewRequest(http.MethodGet, "/v1/tasks/"+t2.ID, nil)
	getRec := httptest.NewRecorder()
	th.ServeHTTP(getRec, getReq)
	var after task.Task
	_ = json.Unmarshal(getRec.Body.Bytes(), &after)
	if after.Status != task.StatusCancelled {
		t.Fatalf("task status=%s", after.Status)
	}
}
