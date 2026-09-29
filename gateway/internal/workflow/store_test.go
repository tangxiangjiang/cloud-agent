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

func TestReviseOnlyWhenAwaitingReview(t *testing.T) {
	store := workflow.NewStore()
	h := store.Handler()
	body := `{"bundleId":"b","repoId":"r","slaveId":"s1","nodes":[{"id":"N1","dependsOn":[]}]}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	// Not awaiting_review yet
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/N1/revise", strings.NewReader(`{"instruction":"fix it"}`)))
	if bad.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d %s", bad.Code, bad.Body.String())
	}

	patchRec := httptest.NewRecorder()
	h.ServeHTTP(patchRec, httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/N1", strings.NewReader(`{"status":"awaiting_review","taskId":"tsk_1"}`)))
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: %d", patchRec.Code)
	}

	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/N1/revise", strings.NewReader(`{"instruction":"把错误码改掉"}`)))
	if okRec.Code != http.StatusOK {
		t.Fatalf("revise: %d %s", okRec.Code, okRec.Body.String())
	}
	var resp struct {
		Workflow workflow.Run           `json:"workflow"`
		Revise   workflow.ReviseEntry   `json:"revise"`
	}
	_ = json.NewDecoder(okRec.Body).Decode(&resp)
	if resp.Workflow.Nodes[0].Status != workflow.NodeRunning {
		t.Fatalf("status want running, got %s", resp.Workflow.Nodes[0].Status)
	}
	if len(resp.Workflow.ReviseHistory) != 1 || resp.Workflow.ReviseHistory[0].Instruction != "把错误码改掉" {
		t.Fatalf("audit: %+v", resp.Workflow.ReviseHistory)
	}
	if resp.Revise.Instruction != "把错误码改掉" {
		t.Fatalf("revise entry: %+v", resp.Revise)
	}
	// Must not be approved
	for _, n := range resp.Workflow.Nodes {
		if n.Status == workflow.NodeApproved {
			t.Fatal("revise must not approve")
		}
	}

	// Slave patching taskId should attach to reviseHistory audit.
	tid := "tsk_revise_1"
	attach := httptest.NewRecorder()
	h.ServeHTTP(attach, httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/N1", strings.NewReader(`{"taskId":"`+tid+`"}`)))
	var after workflow.Run
	_ = json.NewDecoder(attach.Body).Decode(&after)
	if len(after.ReviseHistory) != 1 || after.ReviseHistory[0].TaskID == nil || *after.ReviseHistory[0].TaskID != tid {
		t.Fatalf("reviseHistory taskId: %+v", after.ReviseHistory)
	}
}

func TestReviewApproveUnlocksDownstream(t *testing.T) {
	store := workflow.NewStore()
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1","progressDoc":"ai/progress.md",
		"nodes":[
			{"id":"A","dependsOn":[]},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A", strings.NewReader(`{"status":"awaiting_review"}`)))

	// reject must not approve
	badDec := httptest.NewRecorder()
	h.ServeHTTP(badDec, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review", strings.NewReader(`{"decision":"nope"}`)))
	if badDec.Code != http.StatusBadRequest {
		t.Fatalf("bad decision: %d", badDec.Code)
	}

	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review", strings.NewReader(`{"decision":"approve","comment":"lgtm"}`)))
	if okRec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", okRec.Code, okRec.Body.String())
	}
	var resp struct {
		Workflow workflow.Run `json:"workflow"`
		Decision string       `json:"decision"`
	}
	_ = json.NewDecoder(okRec.Body).Decode(&resp)
	if resp.Decision != "approve" {
		t.Fatalf("decision: %s", resp.Decision)
	}
	if resp.Workflow.Nodes[0].Status != workflow.NodeApproved {
		t.Fatalf("A want approved, got %s", resp.Workflow.Nodes[0].Status)
	}
	if resp.Workflow.Nodes[1].Status != workflow.NodeReady {
		t.Fatalf("B want ready after approve, got %s", resp.Workflow.Nodes[1].Status)
	}
}

func TestReviewRejectDoesNotUnlock(t *testing.T) {
	store := workflow.NewStore()
	h := store.Handler()
	body := `{"bundleId":"b","repoId":"r","nodes":[{"id":"A","dependsOn":[]},{"id":"B","dependsOn":["A"]}]}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A", strings.NewReader(`{"status":"awaiting_review"}`)))

	rej := httptest.NewRecorder()
	h.ServeHTTP(rej, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review", strings.NewReader(`{"decision":"reject"}`)))
	if rej.Code != http.StatusOK {
		t.Fatalf("reject: %d", rej.Code)
	}
	var resp struct {
		Workflow workflow.Run `json:"workflow"`
	}
	_ = json.NewDecoder(rej.Body).Decode(&resp)
	if resp.Workflow.Nodes[0].Status != workflow.NodeRejected {
		t.Fatalf("A want rejected, got %s", resp.Workflow.Nodes[0].Status)
	}
	if resp.Workflow.Nodes[1].Status == workflow.NodeReady || resp.Workflow.Nodes[1].Status == workflow.NodeApproved {
		t.Fatalf("B must not unlock on reject, got %s", resp.Workflow.Nodes[1].Status)
	}
}

func TestPutAndGetDiff(t *testing.T) {
	authStore := auth.NewStore("PAIR")
	wfStore := workflow.NewStore()
	pairRec := httptest.NewRecorder()
	authStore.HandlePair(pairRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pair", strings.NewReader(`{"pairCode":"PAIR"}`)))
	var pairResp struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(pairRec.Body).Decode(&pairResp)

	mux := http.NewServeMux()
	mux.Handle("/v1/workflows", authStore.Middleware(wfStore.Handler()))
	mux.Handle("/v1/workflows/", authStore.Middleware(wfStore.Handler()))

	createBody := `{"bundleId":"b","repoId":"r","nodes":[{"id":"N1","dependsOn":[]}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	unauth := httptest.NewRecorder()
	mux.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/v1/workflows/"+run.ID+"/nodes/N1/diff", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", unauth.Code)
	}

	putBody := `{
		"workflowId":"` + run.ID + `",
		"nodeId":"N1",
		"baseline":"git:abc1234",
		"files":[{"path":"README.md","status":"added","additions":1,"deletions":0,"unifiedDiff":"--- /dev/null\n+++ b/README.md\n@@ -0,0 +1 @@\n+# hi\n"}]
	}`
	putReq := httptest.NewRequest(http.MethodPut, "/v1/workflows/"+run.ID+"/nodes/N1/diff", strings.NewReader(putBody))
	putReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/workflows/"+run.ID+"/nodes/N1/diff", nil)
	getReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", getRec.Code, getRec.Body.String())
	}
	var diff workflow.NodeDiff
	if err := json.NewDecoder(getRec.Body).Decode(&diff); err != nil {
		t.Fatal(err)
	}
	if diff.WorkflowID != run.ID || diff.NodeID != "N1" {
		t.Fatalf("ids: %+v", diff)
	}
	if diff.Baseline == nil || *diff.Baseline != "git:abc1234" {
		t.Fatalf("baseline: %+v", diff.Baseline)
	}
	if len(diff.Files) != 1 || diff.Files[0].Path != "README.md" || diff.Files[0].UnifiedDiff == "" {
		t.Fatalf("files: %+v", diff.Files)
	}

	// No apply-patch endpoint
	applyReq := httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/N1/apply-patch", strings.NewReader(`{}`))
	applyReq.Header.Set("Authorization", "Bearer "+pairResp.Token)
	applyRec := httptest.NewRecorder()
	mux.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusNotFound && applyRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("apply-patch must not be supported, got %d", applyRec.Code)
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

type recordingStarter struct {
	assignN      int
	reviewN      int
	reviseN      int
	lastRun      *workflow.Run
	lastAutoAppr bool
	lastDecision string
}

func (r *recordingStarter) AssignWorkflow(run *workflow.Run) bool {
	r.assignN++
	r.lastRun = run
	return true
}
func (r *recordingStarter) AssignRevise(slaveID, workflowID, nodeID, instruction string) bool {
	r.reviseN++
	return true
}
func (r *recordingStarter) AssignReview(slaveID, workflowID, nodeID, decision, comment string, autoApprove bool) bool {
	r.reviewN++
	r.lastDecision = decision
	r.lastAutoAppr = autoApprove
	return true
}

func TestApproveDoesNotAssignWithoutAutoStartNext(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	store.SetStarter(st)
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1",
		"nodes":[
			{"id":"A","dependsOn":[]},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)
	if run.Nodes[0].Policy == nil || run.Nodes[0].Policy.AutoStartNext {
		t.Fatalf("default policy must be autoStartNext=false: %+v", run.Nodes[0].Policy)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A", strings.NewReader(`{"status":"awaiting_review"}`)))

	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review", strings.NewReader(`{"decision":"approve"}`)))
	if okRec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", okRec.Code, okRec.Body.String())
	}
	var resp struct {
		Workflow      workflow.Run `json:"workflow"`
		AutoStartNext bool         `json:"autoStartNext"`
	}
	_ = json.NewDecoder(okRec.Body).Decode(&resp)
	if resp.AutoStartNext {
		t.Fatal("autoStartNext should be false")
	}
	if resp.Workflow.Nodes[1].Status != workflow.NodeReady {
		t.Fatalf("B want ready, got %s", resp.Workflow.Nodes[1].Status)
	}
	if st.reviewN != 1 {
		t.Fatalf("review delivered want 1, got %d", st.reviewN)
	}
	if st.assignN != 0 {
		t.Fatalf("approve must not AssignWorkflow by default, got %d", st.assignN)
	}

	cont := httptest.NewRecorder()
	h.ServeHTTP(cont, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/continue", nil))
	if cont.Code != http.StatusOK {
		t.Fatalf("continue: %d %s", cont.Code, cont.Body.String())
	}
	if st.assignN != 1 {
		t.Fatalf("continue should AssignWorkflow once, got %d", st.assignN)
	}
}

func TestApproveAssignsWhenAutoStartNext(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	store.SetStarter(st)
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1",
		"nodes":[
			{"id":"A","dependsOn":[],"policy":{"autoApprove":false,"autoStartNext":true}},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)
	if run.Nodes[0].Policy == nil || !run.Nodes[0].Policy.AutoStartNext {
		t.Fatalf("want autoStartNext true: %+v", run.Nodes[0].Policy)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A", strings.NewReader(`{"status":"awaiting_review"}`)))

	okRec := httptest.NewRecorder()
	h.ServeHTTP(okRec, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review", strings.NewReader(`{"decision":"approve"}`)))
	if okRec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", okRec.Code, okRec.Body.String())
	}
	var resp struct {
		AutoStartNext bool `json:"autoStartNext"`
	}
	_ = json.NewDecoder(okRec.Body).Decode(&resp)
	if !resp.AutoStartNext {
		t.Fatal("autoStartNext flag in response")
	}
	if st.assignN != 1 {
		t.Fatalf("approve with autoStartNext must AssignWorkflow once, got %d", st.assignN)
	}
}

func TestStartReadyNode(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	store.SetStarter(st)
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1",
		"nodes":[
			{"id":"A","dependsOn":[]},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	// A is ready at create; start node A
	startA := httptest.NewRecorder()
	h.ServeHTTP(startA, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/start", nil))
	if startA.Code != http.StatusOK {
		t.Fatalf("start A: %d %s", startA.Code, startA.Body.String())
	}
	if st.assignN != 1 {
		t.Fatalf("start node assign=%d", st.assignN)
	}

	// B still pending — must conflict
	badB := httptest.NewRecorder()
	h.ServeHTTP(badB, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/B/start", nil))
	if badB.Code != http.StatusConflict {
		t.Fatalf("start B want 409, got %d", badB.Code)
	}

	// PATCH policy on B while pending
	pol := httptest.NewRecorder()
	h.ServeHTTP(pol, httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/B",
		strings.NewReader(`{"policy":{"autoApprove":false,"autoStartNext":true},"model":"composer-2.5"}`)))
	if pol.Code != http.StatusOK {
		t.Fatalf("patch policy: %d %s", pol.Code, pol.Body.String())
	}
	var after workflow.Run
	_ = json.NewDecoder(pol.Body).Decode(&after)
	if after.Nodes[1].Policy == nil || !after.Nodes[1].Policy.AutoStartNext {
		t.Fatalf("B policy: %+v", after.Nodes[1].Policy)
	}
	if after.Nodes[1].Model == nil || *after.Nodes[1].Model != "composer-2.5" {
		t.Fatalf("B model: %+v", after.Nodes[1].Model)
	}
}

type recordingAuditor struct {
	events []struct {
		wf, node, decision string
		autoApprove        bool
		autoStartNext      bool
	}
}

func (a *recordingAuditor) AuditReview(workflowID, nodeID, decision string, autoApprove, autoStartNext bool) {
	a.events = append(a.events, struct {
		wf, node, decision string
		autoApprove        bool
		autoStartNext      bool
	}{workflowID, nodeID, decision, autoApprove, autoStartNext})
}

func TestAutoApproveWithoutAutoStartNext(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	aud := &recordingAuditor{}
	store.SetStarter(st)
	store.SetReviewAuditor(aud)
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1",
		"nodes":[
			{"id":"A","dependsOn":[],"policy":{"autoApprove":true,"autoStartNext":false}},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	patch := httptest.NewRecorder()
	h.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A",
		strings.NewReader(`{"status":"awaiting_review","taskId":"tsk_1"}`)))
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patch.Code, patch.Body.String())
	}
	var after workflow.Run
	_ = json.NewDecoder(patch.Body).Decode(&after)
	if after.Nodes[0].Status != workflow.NodeApproved {
		t.Fatalf("A want approved, got %s", after.Nodes[0].Status)
	}
	if after.Nodes[1].Status != workflow.NodeReady {
		t.Fatalf("B want ready, got %s", after.Nodes[1].Status)
	}
	if st.reviewN != 1 || !st.lastAutoAppr || st.lastDecision != "approve" {
		t.Fatalf("review: n=%d auto=%v dec=%s", st.reviewN, st.lastAutoAppr, st.lastDecision)
	}
	if st.assignN != 0 {
		t.Fatalf("must not AssignWorkflow when autoStartNext=false, got %d", st.assignN)
	}
	if len(aud.events) != 1 || !aud.events[0].autoApprove || aud.events[0].autoStartNext {
		t.Fatalf("audit: %+v", aud.events)
	}
}

func TestAutoApproveWithAutoStartNext(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	aud := &recordingAuditor{}
	store.SetStarter(st)
	store.SetReviewAuditor(aud)
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1",
		"nodes":[
			{"id":"A","dependsOn":[],"policy":{"autoApprove":true,"autoStartNext":true}},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	patch := httptest.NewRecorder()
	h.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A",
		strings.NewReader(`{"status":"awaiting_review"}`)))
	if patch.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", patch.Code, patch.Body.String())
	}
	var after workflow.Run
	_ = json.NewDecoder(patch.Body).Decode(&after)
	if after.Nodes[0].Status != workflow.NodeApproved {
		t.Fatalf("want approved got %s", after.Nodes[0].Status)
	}
	if st.reviewN != 1 || !st.lastAutoAppr {
		t.Fatalf("review auto: %+v", st)
	}
	if st.assignN != 1 {
		t.Fatalf("autoStartNext must AssignWorkflow, got %d", st.assignN)
	}
	if len(aud.events) != 1 || !aud.events[0].autoApprove || !aud.events[0].autoStartNext {
		t.Fatalf("audit: %+v", aud.events)
	}
}

func TestRejectIgnoresAutoApprove(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	store.SetStarter(st)
	h := store.Handler()
	// Reject is only via review API; autoApprove must not apply and must not assign.
	body := `{
		"bundleId":"b","repoId":"r","slaveId":"s1",
		"nodes":[
			{"id":"A","dependsOn":[],"policy":{"autoApprove":false,"autoStartNext":true}},
			{"id":"B","dependsOn":["A"]}
		]
	}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A",
		strings.NewReader(`{"status":"awaiting_review"}`)))
	st.reviewN = 0
	st.assignN = 0
	st.lastAutoAppr = true

	rej := httptest.NewRecorder()
	h.ServeHTTP(rej, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review",
		strings.NewReader(`{"decision":"reject"}`)))
	if rej.Code != http.StatusOK {
		t.Fatalf("reject: %d", rej.Code)
	}
	var resp struct {
		Workflow    workflow.Run `json:"workflow"`
		AutoApprove bool         `json:"autoApprove"`
	}
	_ = json.NewDecoder(rej.Body).Decode(&resp)
	if resp.AutoApprove {
		t.Fatal("reject must not be autoApprove")
	}
	if resp.Workflow.Nodes[0].Status != workflow.NodeRejected {
		t.Fatalf("want rejected, got %s", resp.Workflow.Nodes[0].Status)
	}
	if st.lastAutoAppr {
		t.Fatal("AssignReview autoApprove must be false for human reject")
	}
	if st.assignN != 0 {
		t.Fatalf("reject must not assign, got %d", st.assignN)
	}
}

func TestHumanApproveAuditNotAuto(t *testing.T) {
	store := workflow.NewStore()
	st := &recordingStarter{}
	aud := &recordingAuditor{}
	store.SetStarter(st)
	store.SetReviewAuditor(aud)
	h := store.Handler()
	body := `{"bundleId":"b","repoId":"r","slaveId":"s1","nodes":[{"id":"A","dependsOn":[]}]}`
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	var run workflow.Run
	_ = json.NewDecoder(createRec.Body).Decode(&run)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPatch, "/v1/workflows/"+run.ID+"/nodes/A",
		strings.NewReader(`{"status":"awaiting_review"}`)))
	ok := httptest.NewRecorder()
	h.ServeHTTP(ok, httptest.NewRequest(http.MethodPost, "/v1/workflows/"+run.ID+"/nodes/A/review",
		strings.NewReader(`{"decision":"approve"}`)))
	if ok.Code != http.StatusOK {
		t.Fatalf("approve: %d", ok.Code)
	}
	if st.lastAutoAppr {
		t.Fatal("human approve must pass autoApprove=false")
	}
	if len(aud.events) != 1 || aud.events[0].autoApprove {
		t.Fatalf("audit human: %+v", aud.events)
	}
}

func TestCreateInheritsDefaultModelAndPolicy(t *testing.T) {
	store := workflow.NewStore()
	h := store.Handler()
	body := `{
		"bundleId":"b","repoId":"r",
		"defaultModel":"composer-2.5",
		"defaultPolicy":{"autoApprove":false,"autoStartNext":true},
		"nodes":[
			{"id":"A","dependsOn":[]},
			{"id":"B","dependsOn":["A"],"model":"gpt-5.6-sol-medium","policy":{"autoApprove":true,"autoStartNext":false}}
		]
	}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/workflows", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var run workflow.Run
	_ = json.NewDecoder(rec.Body).Decode(&run)
	if run.Nodes[0].Model == nil || *run.Nodes[0].Model != "composer-2.5" {
		t.Fatalf("A inherits model: %+v", run.Nodes[0].Model)
	}
	if run.Nodes[0].Policy == nil || !run.Nodes[0].Policy.AutoStartNext || run.Nodes[0].Policy.AutoApprove {
		t.Fatalf("A inherits policy: %+v", run.Nodes[0].Policy)
	}
	if run.Nodes[1].Model == nil || *run.Nodes[1].Model != "gpt-5.6-sol-medium" {
		t.Fatalf("B explicit model: %+v", run.Nodes[1].Model)
	}
	if run.Nodes[1].Policy == nil || !run.Nodes[1].Policy.AutoApprove || run.Nodes[1].Policy.AutoStartNext {
		t.Fatalf("B explicit policy: %+v", run.Nodes[1].Policy)
	}
}

