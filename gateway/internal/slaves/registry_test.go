// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package slaves_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
)

func TestListFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
slaves:
  - id: slave_devpc
    name: "dev machine"
    online: false
    repos:
      - id: r_cloud_agent
        name: cloud-agent
        cwd: /path/to/your/cloud-agent
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := slaves.LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reg := slaves.NewRegistry(cfg.Slaves)

	req := httptest.NewRequest(http.MethodGet, "/v1/slaves", nil)
	rec := httptest.NewRecorder()
	reg.HandleList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var resp struct {
		Slaves []slaves.Slave `json:"slaves"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Slaves) != 1 {
		t.Fatalf("len=%d", len(resp.Slaves))
	}
	s := resp.Slaves[0]
	if s.ID != "slave_devpc" || s.Online != false || len(s.Repos) != 1 {
		t.Fatalf("unexpected slave: %+v", s)
	}
	if s.Repos[0].ID != "r_cloud_agent" || s.Repos[0].Cwd != "/path/to/your/cloud-agent" {
		t.Fatalf("unexpected repo: %+v", s.Repos[0])
	}
}

func TestEmptyWithoutConfig(t *testing.T) {
	reg := slaves.NewRegistry(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/slaves", nil)
	rec := httptest.NewRecorder()
	reg.HandleList(rec, req)
	var resp struct {
		Slaves []slaves.Slave `json:"slaves"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Slaves == nil || len(resp.Slaves) != 0 {
		t.Fatalf("want empty list, got %#v", resp.Slaves)
	}
}
