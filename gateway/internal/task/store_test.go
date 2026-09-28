// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package task_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

func TestCreateIdempotentAndCancel(t *testing.T) {
	store := task.NewStore()
	h := store.Handler()

	body := []byte(`{"slaveId":"slave_devpc","repoId":"r_cloud_agent","prompt":"hello","model":"composer-2.5"}`)

	req1 := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(body))
	req1.Header.Set("Idempotency-Key", "idem-1")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	var t1 task.Task
	if err := json.Unmarshal(rec1.Body.Bytes(), &t1); err != nil {
		t.Fatal(err)
	}
	if t1.ID == "" || t1.Status != task.StatusQueued {
		t.Fatalf("unexpected task: %+v", t1)
	}
	if t1.Prompt == nil || *t1.Prompt != "hello" {
		t.Fatalf("prompt mismatch: %+v", t1.Prompt)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/tasks", bytes.NewReader(body))
	req2.Header.Set("Idempotency-Key", "idem-1")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("idempotent replay status=%d", rec2.Code)
	}
	var t2 task.Task
	if err := json.Unmarshal(rec2.Body.Bytes(), &t2); err != nil {
		t.Fatal(err)
	}
	if t2.ID != t1.ID {
		t.Fatalf("idempotency created duplicate: %s vs %s", t1.ID, t2.ID)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/tasks/"+t1.ID, nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d", getRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/tasks?status=queued", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d", listRec.Code)
	}
	var list struct {
		Tasks []task.Task `json:"tasks"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Tasks) != 1 {
		t.Fatalf("list len=%d", len(list.Tasks))
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/tasks/"+t1.ID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	h.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	var cancelResp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(cancelRec.Body.Bytes(), &cancelResp); err != nil {
		t.Fatal(err)
	}
	if cancelResp.Status != task.StatusCancelled {
		t.Fatalf("cancel status=%s want cancelled", cancelResp.Status)
	}

	get2 := httptest.NewRequest(http.MethodGet, "/v1/tasks/"+t1.ID, nil)
	get2Rec := httptest.NewRecorder()
	h.ServeHTTP(get2Rec, get2)
	var after task.Task
	_ = json.Unmarshal(get2Rec.Body.Bytes(), &after)
	if after.Status != task.StatusCancelled {
		t.Fatalf("task status after cancel=%s", after.Status)
	}
}
