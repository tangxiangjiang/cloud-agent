// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package models

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// RefreshFunc asks an online Slave to re-fetch Cursor.models.list.
// Returns slaveId used, or empty if none online.
type RefreshFunc func() (slaveID string, ok bool)

// API serves model picker + manage endpoints.
type API struct {
	Store   *Store
	Refresh RefreshFunc
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Mount registers:
//
//	GET    /v1/models
//	GET    /v1/models/manage
//	POST   /v1/models
//	DELETE /v1/models/{id}
//	PUT    /v1/models/default
//	POST   /v1/models/refresh
func (a *API) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	if wrap == nil {
		wrap = func(h http.Handler) http.Handler { return h }
	}
	mux.Handle("GET /v1/models", wrap(http.HandlerFunc(a.handleGetCatalog)))
	mux.Handle("GET /v1/models/manage", wrap(http.HandlerFunc(a.handleManage)))
	mux.Handle("POST /v1/models", wrap(http.HandlerFunc(a.handleAdd)))
	mux.Handle("DELETE /v1/models/{id}", wrap(http.HandlerFunc(a.handleDelete)))
	mux.Handle("PUT /v1/models/default", wrap(http.HandlerFunc(a.handleSetDefault)))
	mux.Handle("POST /v1/models/refresh", wrap(http.HandlerFunc(a.handleRefresh)))
}

func (a *API) handleGetCatalog(w http.ResponseWriter, r *http.Request) {
	c, err := a.Store.Catalog()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) handleManage(w http.ResponseWriter, r *http.Request) {
	v, err := a.Store.Manage()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.Store.AddSelected(body.ID, body.Label); err != nil {
		status := http.StatusBadRequest
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	v, err := a.Store.Manage()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if err := a.Store.RemoveSelected(id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errKeepAuto) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	v, err := a.Store.Manage()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleSetDefault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.Store.SetDefault(body.ID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	v, err := a.Store.Manage()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if a.Refresh == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no slave refresh"})
		return
	}
	slaveID, ok := a.Refresh()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no online slave"})
		return
	}
	v, err := a.Store.Manage()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"slaveId":   slaveID,
		"status":    "refresh_requested",
		"selected":  v.Selected,
		"available": v.Available,
		"default":   v.Default,
	})
}
