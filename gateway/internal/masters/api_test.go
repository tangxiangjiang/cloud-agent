// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package masters_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/masters"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

func TestMasterRegisterGetAndControl(t *testing.T) {
	authStore := auth.NewStore("PAIR")
	tasks := task.NewStore()
	slaveReg := slaves.NewRegistry(nil)
	reg := masters.NewRegistry()
	hub := masters.NewOutboundHub(authStore, reg)
	api := masters.NewAPI(reg, hub, slaveReg, tasks, nil)
	api.Timeout = 3 * time.Second

	pairRec := httptest.NewRecorder()
	authStore.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pair struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(pairRec.Body.Bytes(), &pair)

	mux := http.NewServeMux()
	api.Mount(mux, authStore.Middleware)
	mux.HandleFunc("GET /v1/master/ws", hub.HandleWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/master/ws"
	mconn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mconn.Close()

	read := func() map[string]any {
		t.Helper()
		_ = mconn.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, data, err := mconn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}

	_ = mconn.WriteJSON(map[string]string{"type": "auth", "token": pair.Token})
	if read()["type"] != "auth.ok" {
		t.Fatal("auth")
	}
	_ = mconn.WriteJSON(map[string]any{
		"type":     "master.register",
		"masterId": "master_test",
		"name":     "pc",
		"slaves": []map[string]any{
			{
				"id":      "slave_a",
				"enabled": true,
				"process": "stopped",
				"project": map[string]string{"id": "r_a", "name": "a", "cwd": "/tmp/a"},
			},
		},
	})
	if read()["type"] != "master.registered" {
		t.Fatal("register")
	}

	// GET list
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/masters", nil)
	req.Header.Set("Authorization", "Bearer "+pair.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("list status %d", res.StatusCode)
	}
	var listBody struct {
		Masters []masters.MasterMirror `json:"masters"`
	}
	_ = json.NewDecoder(res.Body).Decode(&listBody)
	if len(listBody.Masters) != 1 || listBody.Masters[0].MasterID != "master_test" {
		t.Fatalf("list: %+v", listBody)
	}
	if listBody.Masters[0].Slaves[0].Process != masters.ProcessStopped {
		t.Fatalf("expected stopped, got %s", listBody.Masters[0].Slaves[0].Process)
	}
	if listBody.Masters[0].Slaves[0].GatewayOnline {
		t.Fatal("gatewayOnline should be false without slave WS")
	}

	// Control start: Master replies ok and reports running
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_ = mconn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, data, err := mconn.ReadMessage()
			if err != nil {
				return
			}
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			if m["type"] == "master.control" {
				reqID, _ := m["requestId"].(string)
				_ = mconn.WriteJSON(map[string]any{
					"type": "master.slaves.report",
					"slaves": []map[string]any{
						{"id": "slave_a", "process": "running", "pid": 1234},
					},
				})
				_ = mconn.WriteJSON(map[string]any{
					"type": "master.control.ok", "requestId": reqID,
				})
				return
			}
		}
	}()

	creq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/masters/master_test/slaves/slave_a/start", nil)
	creq.Header.Set("Authorization", "Bearer "+pair.Token)
	cres, err := http.DefaultClient.Do(creq)
	if err != nil {
		t.Fatal(err)
	}
	defer cres.Body.Close()
	if cres.StatusCode != 200 {
		t.Fatalf("start status %d", cres.StatusCode)
	}
	<-done

	getReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/masters/master_test", nil)
	getReq.Header.Set("Authorization", "Bearer "+pair.Token)
	getRes, _ := http.DefaultClient.Do(getReq)
	defer getRes.Body.Close()
	var detail masters.MasterMirror
	_ = json.NewDecoder(getRes.Body).Decode(&detail)
	if detail.Slaves[0].Process != masters.ProcessRunning {
		t.Fatalf("expected running after start report, got %s", detail.Slaves[0].Process)
	}
}

func TestMasterControlTimeout504(t *testing.T) {
	authStore := auth.NewStore("PAIR")
	tasks := task.NewStore()
	slaveReg := slaves.NewRegistry(nil)
	reg := masters.NewRegistry()
	hub := masters.NewOutboundHub(authStore, reg)
	api := masters.NewAPI(reg, hub, slaveReg, tasks, nil)
	api.Timeout = 200 * time.Millisecond

	pairRec := httptest.NewRecorder()
	authStore.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pair struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(pairRec.Body.Bytes(), &pair)

	mux := http.NewServeMux()
	api.Mount(mux, authStore.Middleware)
	mux.HandleFunc("GET /v1/master/ws", hub.HandleWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/master/ws"
	mconn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mconn.Close()

	read := func() map[string]any {
		t.Helper()
		_ = mconn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := mconn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	_ = mconn.WriteJSON(map[string]string{"type": "auth", "token": pair.Token})
	_ = read()
	_ = mconn.WriteJSON(map[string]any{
		"type": "master.register", "masterId": "master_to", "slaves": []any{},
	})
	_ = read()

	// Drain control msg but do not reply
	go func() {
		_, _, _ = mconn.ReadMessage()
	}()

	creq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/masters/master_to/slaves/x/start", nil)
	creq.Header.Set("Authorization", "Bearer "+pair.Token)
	cres, err := http.DefaultClient.Do(creq)
	if err != nil {
		t.Fatal(err)
	}
	defer cres.Body.Close()
	if cres.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("want 504, got %d", cres.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(cres.Body).Decode(&body)
	if body["code"] != "master_control_timeout" {
		t.Fatalf("body %+v", body)
	}
}

func TestMasterOffline409(t *testing.T) {
	authStore := auth.NewStore("PAIR")
	reg := masters.NewRegistry()
	hub := masters.NewOutboundHub(authStore, reg)
	api := masters.NewAPI(reg, hub, slaves.NewRegistry(nil), task.NewStore(), nil)

	pairRec := httptest.NewRecorder()
	authStore.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pair struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(pairRec.Body.Bytes(), &pair)

	mux := http.NewServeMux()
	api.Mount(mux, authStore.Middleware)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	creq, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/masters/missing/slaves/x/start", nil)
	creq.Header.Set("Authorization", "Bearer "+pair.Token)
	cres, err := http.DefaultClient.Do(creq)
	if err != nil {
		t.Fatal(err)
	}
	defer cres.Body.Close()
	if cres.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", cres.StatusCode)
	}
}
