// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Package models serves GET /v1/models for App chat model picker (M09-P03).
package models

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

// Entry is one selectable model in the App dropdown.
type Entry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Catalog is the GET /v1/models response.
type Catalog struct {
	Default string  `json:"default"`
	Models  []Entry `json:"models"`
}

// DefaultCatalog is used when GATEWAY_MODELS is unset.
func DefaultCatalog() Catalog {
	return Catalog{
		Default: "auto",
		Models: []Entry{
			{ID: "auto", Label: "Auto"},
			{ID: "composer-2.5", Label: "Composer 2.5"},
			{ID: "composer-2", Label: "Composer 2"},
			{ID: "gpt-5.6-sol-medium", Label: "GPT 5.6"},
			{ID: "claude-sonnet-5-5-high", Label: "Claude Sonnet 5.5"},
		},
	}
}

// FromEnv builds catalog from GATEWAY_MODELS (comma-separated ids).
// Example: auto,composer-2.5,gpt-5.6-sol-medium
// Empty → DefaultCatalog.
func FromEnv() Catalog {
	raw := strings.TrimSpace(os.Getenv("GATEWAY_MODELS"))
	if raw == "" {
		return DefaultCatalog()
	}
	parts := strings.Split(raw, ",")
	out := Catalog{Default: "auto", Models: make([]Entry, 0, len(parts)+1)}
	seen := map[string]bool{}
	for _, p := range parts {
		id := strings.TrimSpace(p)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		label := id
		if id == "auto" {
			label = "Auto"
		}
		out.Models = append(out.Models, Entry{ID: id, Label: label})
	}
	if len(out.Models) == 0 {
		return DefaultCatalog()
	}
	if !seen["auto"] {
		out.Models = append([]Entry{{ID: "auto", Label: "Auto"}}, out.Models...)
	}
	return out
}

// Handler returns GET /v1/models.
func Handler(catalog Catalog) http.HandlerFunc {
	body, _ := json.Marshal(catalog)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}
