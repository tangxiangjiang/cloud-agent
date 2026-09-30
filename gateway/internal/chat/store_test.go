// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package chat_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/chat"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
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

func TestBuildPromptWithPhaseRef(t *testing.T) {
	got := chat.BuildPrompt("agent", "fix the DoD", chat.ChatRef{
		Kind:        "phase",
		ID:          "M11-P06",
		Title:       "文档硬化",
		MilestoneID: "M11",
		PhaseRef:    "doc/roadmaps/cloud-agent/phases/M11-P06-docs-harden.md",
	})
	if !strings.Contains(got, "fix the DoD") {
		t.Fatalf("missing user text: %s", got)
	}
	if !strings.Contains(got, "[Referenced plan phase]") {
		t.Fatalf("missing ref block: %s", got)
	}
	if !strings.Contains(got, "phaseRef: doc/roadmaps/cloud-agent/phases/M11-P06-docs-harden.md") {
		t.Fatalf("missing phaseRef: %s", got)
	}
	if !strings.Contains(got, "id: M11-P06") {
		t.Fatalf("missing id: %s", got)
	}
	// invalid refs dropped
	empty := chat.BuildPrompt("agent", "hi", chat.ChatRef{Kind: "phase", ID: "x"})
	if empty != "hi" {
		t.Fatalf("invalid ref should be ignored: %q", empty)
	}
}

func TestMessageRefsRoundTrip(t *testing.T) {
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

	msgBody := `{
		"text":"这段 DoD 不一致，帮我改 plan",
		"mode":"agent",
		"refs":[{
			"kind":"phase",
			"id":"M11-P06",
			"title":"文档硬化",
			"milestoneId":"M11",
			"phaseRef":"doc/roadmaps/cloud-agent/phases/M11-P06-docs-harden.md"
		}]
	}`
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
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/chats/"+sess.ID, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get chat status=%d", getRec.Code)
	}
	var loaded chat.Session
	if err := json.Unmarshal(getRec.Body.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Messages) == 0 || len(loaded.Messages[0].Refs) != 1 {
		t.Fatalf("expected refs on user message: %+v", loaded.Messages)
	}
	ref := loaded.Messages[0].Refs[0]
	if ref.Kind != "phase" || ref.ID != "M11-P06" || ref.PhaseRef == "" {
		t.Fatalf("ref: %+v", ref)
	}

	taskRec := httptest.NewRecorder()
	tasks.Handler().ServeHTTP(taskRec, httptest.NewRequest(http.MethodGet, "/v1/tasks/"+out.TaskID, nil))
	var tsk task.Task
	if err := json.Unmarshal(taskRec.Body.Bytes(), &tsk); err != nil {
		t.Fatal(err)
	}
	if tsk.Prompt == nil || !strings.Contains(*tsk.Prompt, "phaseRef: doc/roadmaps/cloud-agent/phases/M11-P06-docs-harden.md") {
		t.Fatalf("task prompt missing phaseRef: %v", tsk.Prompt)
	}
}

func TestTruncateContent(t *testing.T) {
	short := chat.TruncateContent("hi")
	if short != "hi" {
		t.Fatalf("%q", short)
	}
	long := strings.Repeat("字", chat.MaxMessageChars+10)
	got := chat.TruncateContent(long)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis: %d runes", len([]rune(got)))
	}
}

func TestPersistRoundTripAndList(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	st, err := persist.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	tasks := task.NewStore()
	chats := chat.NewStore(tasks)
	chats.SetPersist(st)
	h := chats.Handler()

	createBody := `{"slaveId":"slave_a","repoId":"r1","mode":"agent","model":"auto"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chats", strings.NewReader(createBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", rec.Code, rec.Body.String())
	}
	var sess chat.Session
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	msgBody := `{"text":"hello history"}`
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(
		http.MethodPost, "/v1/chats/"+sess.ID+"/messages", strings.NewReader(msgBody),
	))
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("msg=%d", rec2.Code)
	}
	var msgOut struct {
		TaskID string `json:"taskId"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &msgOut)

	asst := `{"taskId":"` + msgOut.TaskID + `","content":"sure, done"}`
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(
		http.MethodPost, "/v1/chats/"+sess.ID+"/assistant", strings.NewReader(asst),
	))
	if rec3.Code != http.StatusOK {
		t.Fatalf("assistant=%d %s", rec3.Code, rec3.Body.String())
	}

	// Simulate Gateway restart: new store loads from same DB.
	chats2 := chat.NewStore(tasks)
	chats2.SetPersist(st)
	if err := chats2.LoadFromPersist(); err != nil {
		t.Fatal(err)
	}
	h2 := chats2.Handler()

	listRec := httptest.NewRecorder()
	h2.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/chats?repoId=r1", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list=%d", listRec.Code)
	}
	var list struct {
		Chats []chat.Session `json:"chats"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Chats) != 1 || list.Chats[0].ID != sess.ID {
		t.Fatalf("list: %+v", list.Chats)
	}
	if list.Chats[0].Preview != "hello history" {
		t.Fatalf("preview: %q", list.Chats[0].Preview)
	}
	if list.Chats[0].Title != "hello history" {
		t.Fatalf("provisional title: %q", list.Chats[0].Title)
	}
	if len(list.Chats[0].Messages) != 0 {
		t.Fatalf("list should omit messages: %+v", list.Chats[0].Messages)
	}

	getRec := httptest.NewRecorder()
	h2.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/chats/"+sess.ID, nil))
	var loaded chat.Session
	_ = json.Unmarshal(getRec.Body.Bytes(), &loaded)
	if len(loaded.Messages) < 2 {
		t.Fatalf("expected user+assistant: %+v", loaded.Messages)
	}
	if loaded.Messages[0].Role != "user" || loaded.Messages[1].Role != "assistant" {
		t.Fatalf("roles: %+v", loaded.Messages)
	}
}

func TestObserveTaskEventPersistsWithoutApp(t *testing.T) {
	tasks := task.NewStore()
	chats := chat.NewStore(tasks)
	h := chats.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/chats",
		strings.NewReader(`{"slaveId":"slave_devpc","repoId":"r1"}`),
	))
	var sess chat.Session
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(
		http.MethodPost, "/v1/chats/"+sess.ID+"/messages",
		strings.NewReader(`{"text":"leave mid reply"}`),
	))
	var msgOut struct {
		TaskID string `json:"taskId"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &msgOut)
	if msgOut.TaskID == "" {
		t.Fatal("expected taskId")
	}

	// Simulate App leaving: no POST /assistant; Gateway observes Slave events.
	chats.ObserveTaskEvent(msgOut.TaskID, "assistant.delta", map[string]any{"text": "partial "})
	chats.ObserveTaskEvent(msgOut.TaskID, "assistant.delta", map[string]any{"text": "answer"})
	chats.ObserveTaskEvent(msgOut.TaskID, "done", map[string]any{"status": "finished"})

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/chats/"+sess.ID, nil))
	var loaded chat.Session
	_ = json.Unmarshal(getRec.Body.Bytes(), &loaded)
	if loaded.Status != chat.StatusIdle {
		t.Fatalf("status=%q", loaded.Status)
	}
	if len(loaded.Messages) < 2 || loaded.Messages[1].Role != "assistant" {
		t.Fatalf("messages: %+v", loaded.Messages)
	}
	if loaded.Messages[1].Content != "partial answer" {
		t.Fatalf("assistant content: %q", loaded.Messages[1].Content)
	}

	// App late POST must be idempotent (no duplicate).
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(
		http.MethodPost, "/v1/chats/"+sess.ID+"/assistant",
		strings.NewReader(`{"taskId":"`+msgOut.TaskID+`","content":"partial answer"}`),
	))
	if rec3.Code != http.StatusOK {
		t.Fatalf("assistant=%d %s", rec3.Code, rec3.Body.String())
	}
	getRec2 := httptest.NewRecorder()
	h.ServeHTTP(getRec2, httptest.NewRequest(http.MethodGet, "/v1/chats/"+sess.ID, nil))
	_ = json.Unmarshal(getRec2.Body.Bytes(), &loaded)
	asstCount := 0
	for _, m := range loaded.Messages {
		if m.Role == "assistant" {
			asstCount++
		}
	}
	if asstCount != 1 {
		t.Fatalf("expected 1 assistant msg, got %d: %+v", asstCount, loaded.Messages)
	}
}

func TestSuggestTitleAndPatch(t *testing.T) {
	if got := chat.SuggestTitle("  fix login loading  "); got != "fix login loading" {
		t.Fatalf("suggest: %q", got)
	}
	if got := chat.SanitizeTitle("  「AI 标题」  "); got != "AI 标题" {
		t.Fatalf("sanitize: %q", got)
	}

	tasks := task.NewStore()
	chats := chat.NewStore(tasks)
	autoDone := make(chan struct{}, 1)
	var gotSlave, gotText string
	chats.SetAutotitle(func(slaveID, chatID, text string) {
		gotSlave, gotText = slaveID, text
		_ = chatID
		select {
		case autoDone <- struct{}{}:
		default:
		}
	})
	h := chats.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/chats",
		strings.NewReader(`{"slaveId":"slave_devpc","repoId":"r1"}`),
	))
	var sess chat.Session
	_ = json.Unmarshal(rec.Body.Bytes(), &sess)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(
		http.MethodPost, "/v1/chats/"+sess.ID+"/messages",
		strings.NewReader(`{"text":"rename me please"}`),
	))
	if rec2.Code != http.StatusAccepted {
		t.Fatalf("msg=%d %s", rec2.Code, rec2.Body.String())
	}
	select {
	case <-autoDone:
	case <-time.After(2 * time.Second):
		t.Fatal("autotitle not called")
	}
	if gotSlave != "slave_devpc" || gotText != "rename me please" {
		t.Fatalf("autotitle args slave=%s text=%q", gotSlave, gotText)
	}

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/chats/"+sess.ID, nil))
	var after chat.Session
	_ = json.Unmarshal(getRec.Body.Bytes(), &after)
	if after.Title != "rename me please" {
		t.Fatalf("provisional title: %q", after.Title)
	}

	patchRec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPatch, "/v1/chats/"+sess.ID,
		strings.NewReader(`{"title":"  手工标题  "}`),
	)
	h.ServeHTTP(patchRec, req)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch=%d %s", patchRec.Code, patchRec.Body.String())
	}
	var patched chat.Session
	_ = json.Unmarshal(patchRec.Body.Bytes(), &patched)
	if patched.Title != "手工标题" {
		t.Fatalf("patched title: %q", patched.Title)
	}
}
