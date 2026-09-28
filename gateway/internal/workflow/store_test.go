// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package workflow_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
)

func TestValidateDAGCycleAndRefs(t *testing.T) {
	err := workflow.ValidateDAG([]workflow.Node{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"A"}},
	})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}

	err = workflow.ValidateDAG([]workflow.Node{
		{ID: "A", DependsOn: []string{"missing"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("expected unknown dep error, got %v", err)
	}

	err = workflow.ValidateDAG([]workflow.Node{
		{ID: "A", DependsOn: []string{}},
		{ID: "B", DependsOn: []string{"A"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateGetListAndAuth(t *testing.T) {
	authStore := auth.NewStore("PAIR")
	wfStore := workflow.NewStore()

	pairRec := httptest.NewRecorder()
	authStore.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pairResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(pairRec.Body).Decode(&pairResp); err != nil || pairResp.Token == "" {
		t.Fatalf("pair failed: %v %s", err, pairRec.Body.String())
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/workflows", authStore.Middleware(wfStore.Handler()))
	mux.Handle("/v1/workflows/", authStore.Middleware(wfStore.Handler()))

	// unauthorized
	unauth := httptest.NewRecorder()
	mux.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/v1/workflows", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", unauth.Code)
	}

	body := `{
		"bundleId":"bundle_m01_demo",
		"bundleRef":"ai/bundles/m01.json",
		"slaveId":"slave_devpc",
		"repoId":"r_cloud_agent",
		"progressDoc":"ai/progress.md",
		"nodes":[
			{"id":"M01-P01","phaseRef":"doc/a.md","title":"A","dependsOn":[],"model":"composer-2.5"},
			{"id":"M01-P02","phaseRef":"doc/b.md","title":"B","dependsOn":["M01-P01"]}
		]
	}`
	createReq := httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createRec.Code, createRec.Body.String())
	}

	var run workflow.Run
	if err := json.NewDecoder(createRec.Body).Decode(&run); err != nil {
		t.Fatal(err)
	}
	if run.ID == "" || run.Status != workflow.StatusPending {
		t.Fatalf("unexpected run: %+v", run)
	}
	if len(run.Nodes) != 2 {
		t.Fatalf("nodes: %+v", run.Nodes)
	}
	if run.Nodes[0].Status != workflow.NodeReady {
		t.Fatalf("root status want ready, got %s", run.Nodes[0].Status)
	}
	if run.Nodes[1].Status != workflow.NodePending {
		t.Fatalf("child status want pending, got %s", run.Nodes[1].Status)
	}
	for _, n := range run.Nodes {
		if n.DependsOn == nil {
			t.Fatalf("dependsOn must be present on %s", n.ID)
		}
		if n.Status == "" {
			t.Fatalf("status missing on %s", n.ID)
		}
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/"+run.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: %d", getRec.Code)
	}

	nodesReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/"+run.ID+"/nodes", nil)
	nodesReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	nodesRec := httptest.NewRecorder()
	mux.ServeHTTP(nodesRec, nodesReq)
	if nodesRec.Code != http.StatusOK {
		t.Fatalf("nodes: %d %s", nodesRec.Code, nodesRec.Body.String())
	}
	var nodesResp struct {
		Nodes []workflow.Node `json:"nodes"`
	}
	if err := json.NewDecoder(nodesRec.Body).Decode(&nodesResp); err != nil {
		t.Fatal(err)
	}
	if len(nodesResp.Nodes) != 2 {
		t.Fatalf("nodes list: %+v", nodesResp.Nodes)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/workflows?status=pending", nil)
	listReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: %d", listRec.Code)
	}
}

func TestRecomputeReadyRequiresApproved(t *testing.T) {
	nodes := []workflow.Node{
		{ID: "A", DependsOn: []string{}, Status: workflow.NodeAwaitingReview},
		{ID: "B", DependsOn: []string{"A"}, Status: workflow.NodePending},
	}
	workflow.RecomputeReady(nodes)
	if nodes[1].Status != workflow.NodePending {
		t.Fatalf("B must stay pending while A awaiting_review, got %s", nodes[1].Status)
	}
	nodes[0].Status = workflow.NodeApproved
	workflow.RecomputeReady(nodes)
	if nodes[1].Status != workflow.NodeReady {
		t.Fatalf("B should become ready after A approved, got %s", nodes[1].Status)
	}
}

func TestStartAndPatchNode(t *testing.T) {
	store := workflow.NewStore()
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"slave_devpc",
		"nodes":[
			{"id":"A","dependsOn":[],"prompt":{"mode":"inline","inline":"hi"}},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createReq := httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body))
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", createRec.Code, createRec.Body.String())
	}
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	startReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start: %d %s", startRec.Code, startRec.Body.String())
	}

	patchBody := `{"status":"awaiting_review","taskId":"tsk_1"}`
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A", strings.NewReader(patchBody))
	patchRec := httptest.NewRecorder()
	h.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patchRec.Code, patchRec.Body.String())
	}
	var after workflow.Run
	_ = json.NewDecoder(patchRec.Body).Decode(&after)
	if after.Nodes[0].Status != workflow.NodeAwaitingReview {
		t.Fatalf("A status: %s", after.Nodes[0].Status)
	}
	if after.Nodes[0].TaskID == nil || *after.Nodes[0].TaskID != "tsk_1" {
		t.Fatalf("taskId: %+v", after.Nodes[0].TaskID)
	}
	if after.Nodes[1].Status != workflow.NodePending {
		t.Fatalf("B must remain pending, got %s", after.Nodes[1].Status)
	}
}

func TestCreateRejectsCycle(t *testing.T) {
	store := workflow.NewStore()
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r",
		"nodes":[
			{"id":"A","dependsOn":["B"]},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "cycle") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}
