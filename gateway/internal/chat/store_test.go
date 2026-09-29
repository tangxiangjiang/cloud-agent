// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package chat_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/chat"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

func TestCreateSessionAndMessageCreatesTask(t *testing.T) {
	tasks := task.NewStore()
	chats := chat.NewStore(tasks)
	h := chats.Handler()

	createBody := `{"slaveId":"slave_devpc","repoId":"r_cloud_agent","mode":"agent","model":"auto"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chats", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create chat status=%d body=%s", rec.Code, rec.Body.String())
	}
	var sess chat.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}
	if sess.ID == "" || sess.Model != "auto" || sess.Mode != "agent" {
		t.Fatalf("session: %+v", sess)
	}

	msgBody := `{"text":"add loading to login","mode":"ask"}`
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(
		http.MethodPost,
		"/v1/chats/"+sess.ID+"/messages",
		bytes.NewReader([]byte(msgBody)),
	))
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("message status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	var out struct {
		TaskID string `json:"taskId"`
		ChatID string `json:"chatId"`
		Mode   string `json:"mode"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.TaskID == "" || out.ChatID != sess.ID || out.Mode != "ask" {
		t.Fatalf("message resp: %+v", out)
	}

	getRec := httptest.NewRecorder()
	tasks.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/tasks/"+out.TaskID, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get task status=%d", getRec.Code)
	}
	var tsk task.Task
	if err := json.Unmarshal(getRec.Body.Bytes(), &tsk); err != nil {
		t.Fatal(err)
	}
	if tsk.ChatID == nil || *tsk.ChatID != sess.ID {
		t.Fatalf("task chatId: %+v", tsk.ChatID)
	}
	if tsk.Mode == nil || *tsk.Mode != "ask" {
		t.Fatalf("task mode: %+v", tsk.Mode)
	}
	if tsk.Model == nil || *tsk.Model != "auto" {
		t.Fatalf("task model: %+v", tsk.Model)
	}
	if tsk.Prompt == nil || !strings.Contains(*tsk.Prompt, "READ-ONLY") {
		t.Fatalf("ask prompt missing prefix: %v", tsk.Prompt)
	}
	if tsk.Prompt != nil && strings.Contains(*tsk.Prompt, "Bearer") {
		t.Fatal("prompt must not contain Bearer")
	}
}

func TestBuildPromptModes(t *testing.T) {
	ask := chat.BuildPrompt("ask", "explain main")
	if !strings.Contains(ask, "READ-ONLY") || !strings.Contains(ask, "explain main") {
		t.Fatalf("ask: %s", ask)
	}
	plan := chat.BuildPrompt("plan", "ship M09")
	if !strings.Contains(plan, "Plan") {
		t.Fatalf("plan: %s", plan)
	}
	agent := chat.BuildPrompt("agent", "fix bug")
	if agent != "fix bug" {
		t.Fatalf("agent: %s", agent)
	}
}
