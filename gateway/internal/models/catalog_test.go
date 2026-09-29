// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package models_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/models"
)

func TestDefaultCatalogHasAuto(t *testing.T) {
	c := models.DefaultCatalog()
	if c.Default != "auto" || len(c.Models) < 2 {
		t.Fatalf("%+v", c)
	}
	if c.Models[0].ID != "auto" {
		t.Fatalf("first should be auto: %+v", c.Models[0])
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("GATEWAY_MODELS", "composer-2.5, gpt-5.6-sol-medium")
	c := models.FromEnv()
	if c.Models[0].ID != "auto" {
		t.Fatalf("auto prepended: %+v", c.Models)
	}
	found := false
	for _, m := range c.Models {
		if m.ID == "composer-2.5" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing composer: %+v", c.Models)
	}
}

func TestHandler(t *testing.T) {
	h := models.Handler(models.DefaultCatalog())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var body models.Catalog
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Default != "auto" {
		t.Fatalf("%+v", body)
	}
}
