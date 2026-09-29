// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Package projectsync implements M08 Gateway APIs for engineering-state sync.
package projectsync

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/audit"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
)

const maxPayloadBytes = 256 * 1024

// SyncDispatcher pushes project.sync to an online Slave.
type SyncDispatcher interface {
	AssignProjectSync(slaveID, requestID, repoID string) bool
}

// Service wires HTTP handlers for project sync.
type Service struct {
	reg   *slaves.Registry
	hub   SyncDispatcher
	store persist.ProjectSyncStore
	audit *audit.Logger
	now   func() time.Time
}

func NewService(
	reg *slaves.Registry,
	hub SyncDispatcher,
	store persist.ProjectSyncStore,
	auditLog *audit.Logger,
) *Service {
	if store == nil {
		store = persist.NewMemoryProjectSync()
	}
	return &Service{
		reg:   reg,
		hub:   hub,
		store: store,
		audit: auditLog,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	if wrap == nil {
		wrap = func(h http.Handler) http.Handler { return h }
	}
	mux.Handle("POST /v1/slaves/{slaveId}/projects/{repoId}/sync", wrap(http.HandlerFunc(s.handleTrigger)))
	mux.Handle("GET /v1/slaves/{slaveId}/projects/{repoId}/sync", wrap(http.HandlerFunc(s.handleGet)))
	mux.Handle("POST /v1/project-sync", wrap(http.HandlerFunc(s.handleReport)))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "req_" + hex.EncodeToString(b[:])
}

func (s *Service) record(action, method, path, ip string, status int, meta map[string]string) {
	if s.audit == nil {
		return
	}
	s.audit.Record(action, method, path, ip, status, meta)
}

type triggerBody struct {
	RequestID string `json:"requestId"`
}

func (s *Service) handleTrigger(w http.ResponseWriter, r *http.Request) {
	slaveID := strings.TrimSpace(r.PathValue("slaveId"))
	repoID := strings.TrimSpace(r.PathValue("repoId"))
	ip := audit.ClientIP(r)
	if slaveID == "" || repoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slaveId and repoId required"})
		s.record("project.sync", r.Method, r.URL.Path, ip, http.StatusBadRequest, nil)
		return
	}
	if !s.reg.IsOnline(slaveID) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "slave offline"})
		s.record("project.sync", r.Method, r.URL.Path, ip, http.StatusConflict, map[string]string{
			"slaveId": slaveID,
			"repoId":  repoID,
			"reason":  "offline",
		})
		return
	}
	if !s.reg.HasProject(slaveID, repoID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repoId not registered on slave"})
		s.record("project.sync", r.Method, r.URL.Path, ip, http.StatusBadRequest, map[string]string{
			"slaveId": slaveID,
			"repoId":  repoID,
			"reason":  "unknown_repo",
		})
		return
	}

	var body triggerBody
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
	}
	reqID := strings.TrimSpace(body.RequestID)
	if reqID == "" {
		reqID = newRequestID()
	}

	if s.hub == nil || !s.hub.AssignProjectSync(slaveID, reqID, repoID) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "slave connection unavailable"})
		s.record("project.sync", r.Method, r.URL.Path, ip, http.StatusServiceUnavailable, map[string]string{
			"slaveId":   slaveID,
			"repoId":    repoID,
			"requestId": reqID,
			"reason":    "no_conn",
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"requestId": reqID,
		"status":    "accepted",
		"slaveId":   slaveID,
		"repoId":    repoID,
	})
	s.record("project.sync", r.Method, r.URL.Path, ip, http.StatusAccepted, map[string]string{
		"slaveId":   slaveID,
		"repoId":    repoID,
		"requestId": reqID,
		"result":    "accepted",
	})
}

// StoreReport UPSERTs a Slave-reported payload (HTTP or WS).
func (s *Service) StoreReport(slaveID, requestID, repoID string, payload json.RawMessage) error {
	slaveID = strings.TrimSpace(slaveID)
	repoID = strings.TrimSpace(repoID)
	if slaveID == "" || repoID == "" || len(payload) == 0 {
		return errBadReport
	}
	if !json.Valid(payload) {
		return errBadReport
	}
	syncedAt := s.now().Format(time.RFC3339Nano)
	var peek struct {
		SyncedAt string `json:"syncedAt"`
	}
	if json.Unmarshal(payload, &peek) == nil && strings.TrimSpace(peek.SyncedAt) != "" {
		syncedAt = strings.TrimSpace(peek.SyncedAt)
	}
	return s.store.UpsertProjectSync(persist.ProjectSyncRow{
		SlaveID:     slaveID,
		RepoID:      repoID,
		SyncedAt:    syncedAt,
		SummaryJSON: payload,
	})
}

var errBadReport = errString("invalid project sync report")

type errString string

func (e errString) Error() string { return string(e) }

type reportBody struct {
	RequestID string          `json:"requestId"`
	SlaveID   string          `json:"slaveId"`
	RepoID    string          `json:"repoId"`
	Payload   json.RawMessage `json:"payload"`
}

func (s *Service) handleReport(w http.ResponseWriter, r *http.Request) {
	ip := audit.ClientIP(r)
	limited := io.LimitReader(r.Body, maxPayloadBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusBadRequest, nil)
		return
	}
	if len(raw) > maxPayloadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload too large"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusRequestEntityTooLarge, nil)
		return
	}
	var body reportBody
	if err := json.Unmarshal(raw, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusBadRequest, nil)
		return
	}
	slaveID := strings.TrimSpace(body.SlaveID)
	repoID := strings.TrimSpace(body.RepoID)
	if slaveID == "" || repoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slaveId and repoId required"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusBadRequest, nil)
		return
	}
	if len(body.Payload) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload required"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusBadRequest, map[string]string{
			"slaveId": slaveID,
			"repoId":  repoID,
		})
		return
	}
	if !json.Valid(body.Payload) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payload must be json"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusBadRequest, map[string]string{
			"slaveId": slaveID,
			"repoId":  repoID,
		})
		return
	}

	syncedAt := s.now().Format(time.RFC3339Nano)
	if err := s.StoreReport(slaveID, strings.TrimSpace(body.RequestID), repoID, body.Payload); err != nil {
		if err == errBadReport {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid report"})
			s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusBadRequest, map[string]string{
				"slaveId": slaveID,
				"repoId":  repoID,
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist failed"})
		s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusInternalServerError, map[string]string{
			"slaveId":   slaveID,
			"repoId":    repoID,
			"requestId": strings.TrimSpace(body.RequestID),
		})
		return
	}
	// Refresh syncedAt from store for response.
	if row, _ := s.store.GetProjectSync(slaveID, repoID); row != nil {
		syncedAt = row.SyncedAt
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"slaveId":   slaveID,
		"repoId":    repoID,
		"syncedAt":  syncedAt,
		"requestId": strings.TrimSpace(body.RequestID),
	})
	s.record("project.sync.result", r.Method, r.URL.Path, ip, http.StatusOK, map[string]string{
		"slaveId":   slaveID,
		"repoId":    repoID,
		"requestId": strings.TrimSpace(body.RequestID),
		"result":    "stored",
	})
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) {
	slaveID := strings.TrimSpace(r.PathValue("slaveId"))
	repoID := strings.TrimSpace(r.PathValue("repoId"))
	ip := audit.ClientIP(r)
	if slaveID == "" || repoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slaveId and repoId required"})
		return
	}
	row, err := s.store.GetProjectSync(slaveID, repoID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load failed"})
		return
	}
	if row == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		s.record("project.sync.get", r.Method, r.URL.Path, ip, http.StatusNotFound, map[string]string{
			"slaveId": slaveID,
			"repoId":  repoID,
		})
		return
	}
	var payload any
	if err := json.Unmarshal(row.SummaryJSON, &payload); err != nil {
		payload = json.RawMessage(row.SummaryJSON)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"slaveId":  row.SlaveID,
		"repoId":   row.RepoID,
		"syncedAt": row.SyncedAt,
		"payload":  payload,
		"warnings": []any{}, // M08-P03 fills reconcile warnings
	})
}
