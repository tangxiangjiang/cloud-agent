// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package persist_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
)

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	fs := persist.NewFileStore(path)

	wfStore := workflow.NewStore()
	// create via import
	payload := workflow.SnapshotPayload{
		Workflows: []workflow.Run{{
			ID:       "wf_1",
			BundleID: "b",
			RepoID:   "r1",
			Status:   workflow.StatusRunning,
			Nodes: []workflow.Node{{
				ID:        "M01-P01",
				DependsOn: nil,
				Status:    workflow.NodeApproved,
			}, {
				ID:        "M01-P02",
				DependsOn: []string{"M01-P01"},
				Status:    workflow.NodeAwaitingReview,
			}},
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}},
		Diffs: map[string]workflow.NodeDiff{},
	}
	wfStore.ImportSnapshot(payload)

	wfJSON, diffJSON, err := wfStore.MarshalSnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.SaveNow(&persist.Snapshot{
		Tokens:    map[string]string{"tok1": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)},
		Workflows: wfJSON,
		Diffs:     diffJSON,
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := fs.Load()
	if err != nil || loaded == nil {
		t.Fatalf("load: %v %#v", err, loaded)
	}
	if loaded.Tokens["tok1"] == "" {
		t.Fatal("token missing")
	}
	wf2 := workflow.NewStore()
	if err := wf2.UnmarshalSnapshotJSON(loaded.Workflows, loaded.Diffs); err != nil {
		t.Fatal(err)
	}
	snap := wf2.ExportSnapshot()
	if len(snap.Workflows) != 1 || snap.Workflows[0].Nodes[0].Status != workflow.NodeApproved {
		b, _ := json.Marshal(snap)
		t.Fatalf("bad restore: %s", b)
	}
	_ = os.Remove(path)
}

func TestSQLiteStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	st, err := persist.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	wfStore := workflow.NewStore()
	payload := workflow.SnapshotPayload{
		Workflows: []workflow.Run{{
			ID:       "wf_1",
			BundleID: "milestone:M01",
			RepoID:   "r1",
			SlaveID:  strPtr("slave_devpc"),
			Status:   workflow.StatusRunning,
			Nodes: []workflow.Node{{
				ID:     "M01-P01",
				Status: workflow.NodeApproved,
			}, {
				ID:        "M01-P02",
				DependsOn: []string{"M01-P01"},
				Status:    workflow.NodeReady,
			}},
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}},
		Diffs: map[string]workflow.NodeDiff{},
	}
	wfStore.ImportSnapshot(payload)
	wfJSON, diffJSON, err := wfStore.MarshalSnapshotJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveNow(&persist.Snapshot{
		Tokens:    map[string]string{"tok1": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)},
		Workflows: wfJSON,
		Diffs:     diffJSON,
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := st.Load()
	if err != nil || loaded == nil {
		t.Fatalf("load: %v %#v", err, loaded)
	}
	if loaded.Tokens["tok1"] == "" {
		t.Fatal("token missing")
	}
	wf2 := workflow.NewStore()
	if err := wf2.UnmarshalSnapshotJSON(loaded.Workflows, loaded.Diffs); err != nil {
		t.Fatal(err)
	}
	snap := wf2.ExportSnapshot()
	if len(snap.Workflows) != 1 || snap.Workflows[0].Nodes[0].Status != workflow.NodeApproved {
		b, _ := json.Marshal(snap)
		t.Fatalf("bad restore: %s", b)
	}
}

func TestSQLiteMigratesLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "state.json")
	dbPath := filepath.Join(dir, "state.db")
	fs := persist.NewFileStore(legacy)
	if err := fs.SaveNow(&persist.Snapshot{
		Tokens:    map[string]string{"tok_legacy": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)},
		Workflows: json.RawMessage(`[{"id":"wf_x","bundleId":"b","repoId":"r","status":"pending","nodes":[{"id":"A","status":"ready","dependsOn":[]}],"createdAt":"2026-09-29T00:00:00Z"}]`),
		Diffs:     json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	st, err := persist.OpenSQLite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	loaded, err := st.Load()
	if err != nil || loaded == nil {
		t.Fatalf("load after migrate: %v", err)
	}
	if loaded.Tokens["tok_legacy"] == "" {
		t.Fatal("legacy token not imported")
	}
	if _, err := os.Stat(legacy + ".migrated"); err != nil {
		t.Fatalf("expected migrated rename: %v", err)
	}
}

func strPtr(s string) *string { return &s }

func TestNormalizeRunning(t *testing.T) {
	payload := workflow.SnapshotPayload{
		Workflows: []workflow.Run{{
			ID:       "wf_1",
			BundleID: "b",
			RepoID:   "r1",
			Status:   workflow.StatusRunning,
			Nodes: []workflow.Node{{
				ID:     "N1",
				Status: workflow.NodeRunning,
			}},
			CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}},
		Diffs: map[string]workflow.NodeDiff{
			"wf_1\x00N1": {WorkflowID: "wf_1", NodeID: "N1", Files: []workflow.NodeDiffFile{}},
		},
	}
	st := workflow.NewStore()
	st.ImportSnapshot(payload)
	snap := st.ExportSnapshot()
	if snap.Workflows[0].Nodes[0].Status != workflow.NodeAwaitingReview {
		t.Fatalf("want awaiting_review got %s", snap.Workflows[0].Nodes[0].Status)
	}
}
