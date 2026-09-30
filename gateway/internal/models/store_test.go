// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package models_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/models"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
)

func TestStoreAddRemoveAndCatalog(t *testing.T) {
	t.Setenv("GATEWAY_MODELS", "")
	s := models.NewStore(persist.NewMemoryModelCatalog())
	c, err := s.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if c.Default != "auto" || len(c.Models) < 1 || c.Models[0].ID != "auto" {
		t.Fatalf("seed: %+v", c)
	}
	if err := s.AddSelected("composer-2.5", "Composer 2.5"); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Catalog()
	found := false
	for _, m := range c.Models {
		if m.ID == "composer-2.5" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing composer: %+v", c.Models)
	}
	if err := s.RemoveSelected("auto"); err == nil {
		t.Fatal("expected keep auto")
	}
	if err := s.RemoveSelected("composer-2.5"); err != nil {
		t.Fatal(err)
	}
	_ = s.SetAvailable([]models.Entry{{ID: "default", Label: "Default"}}, "slave_a")
	v, err := s.Manage()
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Available) != 1 || v.Available[0].ID != "default" {
		t.Fatalf("available: %+v", v.Available)
	}
}

func TestAPIManageAdd(t *testing.T) {
	t.Setenv("GATEWAY_MODELS", "auto")
	api := &models.API{Store: models.NewStore(persist.NewMemoryModelCatalog())}
	mux := http.NewServeMux()
	api.Mount(mux, nil)

	body, _ := json.Marshal(map[string]string{"id": "composer-2.5", "label": "Composer"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/models", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var view models.ManageView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range view.Selected {
		if m.ID == "composer-2.5" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", view)
	}
}
