// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package projectsync_test

import (
	"encoding/json"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/projectsync"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
)

func strPtr(s string) *string { return &s }

func TestReconcileProgressAheadAndGatewayAhead(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"dirty":         false,
		"branch":        "main",
		"phases": []map[string]any{
			{"id": "M01-P01", "progressStatus": "approved", "title": "First"},
			{"id": "M01-P02", "progressStatus": "pending", "title": "Second"},
		},
	})
	run := &workflow.Run{
		ID:       "wf_1",
		BundleID: "milestone:M01",
		RepoID:   "r1",
		SlaveID:  strPtr("slave_a"),
		Status:   workflow.StatusRunning,
		Nodes: []workflow.Node{
			{ID: "M01-P01", Status: workflow.NodeReady, Title: strPtr("First"), DependsOn: []string{}},
			{ID: "M01-P02", Status: workflow.NodeApproved, Title: strPtr("Second"), DependsOn: []string{"M01-P01"}},
		},
	}

	warnings, report := projectsync.Reconcile(payload, []*workflow.Run{run}, run, true)
	codes := map[string]bool{}
	for _, w := range warnings {
		codes[w.Code] = true
	}
	if !codes["progress_ahead"] {
		t.Fatalf("expected progress_ahead: %#v", warnings)
	}
	if !codes["gateway_ahead"] {
		t.Fatalf("expected gateway_ahead: %#v", warnings)
	}
	for _, w := range warnings {
		if w.Code == "gateway_ahead" && w.Suggestion != "align_progress" {
			t.Fatalf("gateway_ahead suggestion=%q want align_progress", w.Suggestion)
		}
		if w.Code == "progress_ahead" && w.Suggestion != "align_workflow" {
			t.Fatalf("progress_ahead suggestion=%q want align_workflow", w.Suggestion)
		}
	}
	keys := projectsync.ApprovedPhaseKeys([]*workflow.Run{run})
	if len(keys) != 1 || keys[0] != "M01-P02" {
		t.Fatalf("ApprovedPhaseKeys: %#v", keys)
	}
	if report.WorkflowID != "wf_1" || len(report.Phases) != 2 {
		t.Fatalf("report: %#v", report)
	}
	var p01 *projectsync.PhaseRow
	for i := range report.Phases {
		if report.Phases[i].ID == "M01-P01" {
			p01 = &report.Phases[i]
		}
	}
	if p01 == nil || p01.NodeStatus == nil || *p01.NodeStatus != workflow.NodeReady {
		t.Fatalf("phase row: %#v", p01)
	}
}

func TestReconcileDirtyAndNoActive(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"dirty": true,
		"phases": []map[string]any{
			{"id": "M01-P01", "progressStatus": "pending"},
		},
	})
	completed := &workflow.Run{
		ID:       "wf_old",
		BundleID: "milestone:M01",
		Status:   workflow.StatusCompleted,
		Nodes: []workflow.Node{
			{ID: "M01-P01", Status: workflow.NodeApproved, DependsOn: []string{}},
		},
	}
	warnings, report := projectsync.Reconcile(payload, []*workflow.Run{completed}, completed, false)
	codes := map[string]bool{}
	for _, w := range warnings {
		codes[w.Code] = true
	}
	if !codes["dirty_worktree"] {
		t.Fatalf("expected dirty: %#v", warnings)
	}
	if !codes["no_active_workflow"] {
		t.Fatalf("expected no_active_workflow: %#v", warnings)
	}
	if report.Active {
		t.Fatal("expected inactive report")
	}
	// Also gateway_ahead: node approved vs progress pending
	if !codes["gateway_ahead"] {
		t.Fatalf("expected gateway_ahead on completed run: %#v", warnings)
	}
}

func TestReconcileCleanAligned(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"dirty": false,
		"phases": []map[string]any{
			{"id": "M01-P01", "progressStatus": "approved"},
		},
	})
	run := &workflow.Run{
		ID:       "wf_ok",
		BundleID: "milestone:M01",
		Status:   workflow.StatusRunning,
		Nodes: []workflow.Node{
			{ID: "M01-P01", Status: workflow.NodeApproved, DependsOn: []string{}},
		},
	}
	warnings, _ := projectsync.Reconcile(payload, []*workflow.Run{run}, run, true)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings: %#v", warnings)
	}
}

func TestReconcileCompletedApprovedBeatsNewerActiveReady(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"dirty": false,
		"phases": []map[string]any{
			{"id": "M12-P06", "progressStatus": "approved"},
		},
	})
	completed := &workflow.Run{
		ID:        "wf_done",
		BundleID:  "milestone:M12",
		RepoID:    "r1",
		Status:    workflow.StatusCompleted,
		CreatedAt: "2026-09-30T00:00:00Z",
		Nodes: []workflow.Node{
			{ID: "M12-P06", Status: workflow.NodeApproved, DependsOn: []string{}},
		},
	}
	newerActive := &workflow.Run{
		ID:        "wf_new",
		BundleID:  "milestone:M12",
		RepoID:    "r1",
		Status:    workflow.StatusRunning,
		CreatedAt: "2026-10-01T00:00:00Z",
		Nodes: []workflow.Node{
			{ID: "M12-P06", Status: workflow.NodeReady, DependsOn: []string{}},
		},
	}
	// newest-first order as ListMilestoneRuns
	warnings, report := projectsync.Reconcile(
		payload,
		[]*workflow.Run{newerActive, completed},
		newerActive,
		true,
	)
	for _, w := range warnings {
		if w.Code == "progress_ahead" {
			t.Fatalf("should not warn progress_ahead when a completed run already approved: %#v", warnings)
		}
	}
	var row *projectsync.PhaseRow
	for i := range report.Phases {
		if report.Phases[i].ID == "M12-P06" {
			row = &report.Phases[i]
		}
	}
	if row == nil || row.NodeStatus == nil || *row.NodeStatus != workflow.NodeApproved {
		t.Fatalf("want node=approved from completed run, got %#v", row)
	}
}

func TestPhaseKeyFromNode(t *testing.T) {
	if got := projectsync.PhaseKeyFromNode(workflow.Node{ID: "M02-P03"}); got != "M02-P03" {
		t.Fatalf("id: %s", got)
	}
	ref := "doc/roadmaps/phases/M05-P01-foo.md"
	if got := projectsync.PhaseKeyFromNode(workflow.Node{ID: "N1", PhaseRef: &ref}); got != "M05-P01" {
		t.Fatalf("phaseRef: %s", got)
	}
}

func TestFindLatestMilestoneRun(t *testing.T) {
	st := workflow.NewStore()
	st.ImportSnapshot(workflow.SnapshotPayload{
		Workflows: []workflow.Run{
			{
				ID:        "wf_old",
				BundleID:  "milestone:M01",
				RepoID:    "r1",
				SlaveID:   strPtr("s1"),
				Status:    workflow.StatusCompleted,
				Nodes:     []workflow.Node{{ID: "M01-P01", Status: workflow.NodeApproved, DependsOn: []string{}}},
				CreatedAt: "2026-09-28T00:00:00Z",
			},
			{
				ID:        "wf_new",
				BundleID:  "milestone:M01",
				RepoID:    "r1",
				SlaveID:   strPtr("s1"),
				Status:    workflow.StatusRunning,
				Nodes:     []workflow.Node{{ID: "M01-P01", Status: workflow.NodeReady, DependsOn: []string{}}},
				CreatedAt: "2026-09-29T00:00:00Z",
			},
		},
		Diffs: map[string]workflow.NodeDiff{},
	})
	run, active := st.FindLatestMilestoneRun("s1", "r1")
	if !active || run == nil || run.ID != "wf_new" {
		t.Fatalf("got active=%v run=%#v", active, run)
	}
}
