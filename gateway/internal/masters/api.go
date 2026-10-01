// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package masters

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/audit"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

const defaultControlTimeout = 30 * time.Second

// SlaveOnlineChecker reports whether a child Slave data-plane WS is online.
type SlaveOnlineChecker interface {
	IsOnline(slaveID string) bool
}

// WorkflowRecoverer recovers stuck "running" nodes after Slave stop/restart.
type WorkflowRecoverer interface {
	InterruptRunningForSlave(slaveID string) int
}

// API exposes /v1/masters* HTTP (sync wait for Master ok/error).
type API struct {
	Reg       *Registry
	Hub       *OutboundHub
	Slaves    SlaveOnlineChecker
	Tasks     *task.Store
	Workflows WorkflowRecoverer
	Audit     *audit.Logger
	Timeout   time.Duration
}

func NewAPI(reg *Registry, hub *OutboundHub, slaveReg *slaves.Registry, tasks *task.Store, auditLog *audit.Logger) *API {
	return &API{
		Reg:     reg,
		Hub:     hub,
		Slaves:  slaveReg,
		Tasks:   tasks,
		Audit:   auditLog,
		Timeout: defaultControlTimeout,
	}
}

func (a *API) Mount(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/masters", wrap(http.HandlerFunc(a.handleList)))
	mux.Handle("GET /v1/masters/{masterId}", wrap(http.HandlerFunc(a.handleGet)))
	mux.Handle("POST /v1/masters/{masterId}/slaves", wrap(http.HandlerFunc(a.handleUpsert)))
	mux.Handle("PUT /v1/masters/{masterId}/slaves/{slaveId}", wrap(http.HandlerFunc(a.handlePut)))
	mux.Handle("PATCH /v1/masters/{masterId}/slaves/{slaveId}", wrap(http.HandlerFunc(a.handlePut)))
	mux.Handle("DELETE /v1/masters/{masterId}/slaves/{slaveId}", wrap(http.HandlerFunc(a.handleDelete)))
	mux.Handle("POST /v1/masters/{masterId}/slaves/{slaveId}/start", wrap(http.HandlerFunc(a.handleStart)))
	mux.Handle("POST /v1/masters/{masterId}/slaves/{slaveId}/stop", wrap(http.HandlerFunc(a.handleStop)))
	mux.Handle("POST /v1/masters/{masterId}/slaves/{slaveId}/restart", wrap(http.HandlerFunc(a.handleRestart)))
}

func (a *API) onlineFn() func(string) bool {
	return func(slaveID string) bool {
		if a.Slaves == nil {
			return false
		}
		return a.Slaves.IsOnline(slaveID)
	}
}

func (a *API) handleList(w http.ResponseWriter, r *http.Request) {
	list := a.Reg.ListWithOnline(a.onlineFn())
	writeJSON(w, http.StatusOK, map[string]any{"masters": list})
}

func (a *API) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("masterId")
	m, ok := a.Reg.GetWithOnline(id, a.onlineFn())
	if !ok {
		writeErr(w, http.StatusNotFound, "master_not_found", "master not found")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (a *API) handleUpsert(w http.ResponseWriter, r *http.Request) {
	masterID := r.PathValue("masterId")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	// Ensure id present for create
	if _, ok := body["id"].(string); !ok || body["id"] == "" {
		writeErr(w, http.StatusBadRequest, "bad_request", "slave.id required")
		return
	}
	a.forwardConfig(w, r, masterID, "master.config.upsert", map[string]any{
		"type":   "master.config.upsert",
		"slave":  body,
	}, "slave.config.upsert")
}

func (a *API) handlePut(w http.ResponseWriter, r *http.Request) {
	masterID := r.PathValue("masterId")
	slaveID := r.PathValue("slaveId")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if id, ok := body["id"].(string); ok && id != "" && id != slaveID {
		writeErr(w, http.StatusBadRequest, "id_immutable", "slave id cannot be changed")
		return
	}
	body["id"] = slaveID
	a.forwardConfig(w, r, masterID, "master.config.upsert", map[string]any{
		"type":  "master.config.upsert",
		"slave": body,
	}, "slave.config.upsert")
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request) {
	masterID := r.PathValue("masterId")
	slaveID := r.PathValue("slaveId")
	// Best-effort stop then delete
	_, _, _, _ = a.controlWait(masterID, slaveID, "stop", true)
	a.forwardConfig(w, r, masterID, "master.config.delete", map[string]any{
		"type":    "master.config.delete",
		"slaveId": slaveID,
	}, "slave.config.delete")
}

func (a *API) handleStart(w http.ResponseWriter, r *http.Request) {
	a.handleControl(w, r, "start", false)
}

func (a *API) handleStop(w http.ResponseWriter, r *http.Request) {
	a.handleControl(w, r, "stop", true)
}

func (a *API) handleRestart(w http.ResponseWriter, r *http.Request) {
	a.handleControl(w, r, "restart", true)
}

func (a *API) handleControl(w http.ResponseWriter, r *http.Request, action string, cancelFirst bool) {
	masterID := r.PathValue("masterId")
	slaveID := r.PathValue("slaveId")
	status, code, errCode, errMsg := a.controlWait(masterID, slaveID, action, cancelFirst)
	a.audit(r, "slave.control."+action, status, map[string]string{
		"masterId": masterID,
		"slaveId":  slaveID,
		"code":     errCode,
	})
	if status >= 400 {
		writeErr(w, status, errCode, errMsg)
		return
	}
	m, _ := a.Reg.GetWithOnline(masterID, a.onlineFn())
	writeJSON(w, code, map[string]any{"ok": true, "master": m})
}

func (a *API) controlWait(masterID, slaveID, action string, cancelFirst bool) (httpStatus, jsonStatus int, errCode, errMsg string) {
	if !a.Reg.IsOnline(masterID) {
		return http.StatusConflict, 0, "master_offline", "master offline"
	}
	if !a.Hub.IsConnected(masterID) {
		return http.StatusServiceUnavailable, 0, "master_unreachable", "master connection unavailable"
	}
	if cancelFirst && a.Tasks != nil && (action == "stop" || action == "restart") {
		_ = a.Tasks.CancelActiveForSlave(slaveID)
	}
	if cancelFirst && a.Workflows != nil && (action == "stop" || action == "restart") {
		_ = a.Workflows.InterruptRunningForSlave(slaveID)
	}
	reqID := newRequestID()
	ch := a.Hub.Pending().Register(reqID)
	okSend := a.Hub.SendJSON(masterID, map[string]any{
		"type":      "master.control",
		"requestId": reqID,
		"slaveId":   slaveID,
		"action":    action,
	})
	if !okSend {
		a.Hub.Pending().Cancel(reqID)
		return http.StatusServiceUnavailable, 0, "master_unreachable", "master connection unavailable"
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultControlTimeout
	}
	ok, msg, code, timedOut := a.Hub.Pending().Wait(ch, timeout)
	if timedOut {
		a.Hub.Pending().Cancel(reqID)
		return http.StatusGatewayTimeout, 0, "master_control_timeout", "master control timed out"
	}
	if !ok {
		if code == "" {
			code = "control_error"
		}
		if msg == "" {
			msg = "master control failed"
		}
		return http.StatusBadRequest, 0, code, msg
	}
	return http.StatusOK, http.StatusOK, "", ""
}

func (a *API) forwardConfig(w http.ResponseWriter, r *http.Request, masterID, _ string, base map[string]any, auditAction string) {
	if !a.Reg.IsOnline(masterID) {
		a.audit(r, auditAction, http.StatusConflict, map[string]string{"masterId": masterID})
		writeErr(w, http.StatusConflict, "master_offline", "master offline")
		return
	}
	if !a.Hub.IsConnected(masterID) {
		a.audit(r, auditAction, http.StatusServiceUnavailable, map[string]string{"masterId": masterID})
		writeErr(w, http.StatusServiceUnavailable, "master_unreachable", "master connection unavailable")
		return
	}
	reqID := newRequestID()
	base["requestId"] = reqID
	ch := a.Hub.Pending().Register(reqID)
	if !a.Hub.SendJSON(masterID, base) {
		a.Hub.Pending().Cancel(reqID)
		writeErr(w, http.StatusServiceUnavailable, "master_unreachable", "master connection unavailable")
		return
	}
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = defaultControlTimeout
	}
	ok, msg, code, timedOut := a.Hub.Pending().Wait(ch, timeout)
	if timedOut {
		a.Hub.Pending().Cancel(reqID)
		a.audit(r, auditAction, http.StatusGatewayTimeout, map[string]string{"masterId": masterID})
		writeErr(w, http.StatusGatewayTimeout, "master_control_timeout", "master control timed out")
		return
	}
	if !ok {
		if code == "" {
			code = "config_error"
		}
		if msg == "" {
			msg = "master config failed"
		}
		a.audit(r, auditAction, http.StatusBadRequest, map[string]string{"masterId": masterID, "code": code})
		writeErr(w, http.StatusBadRequest, code, msg)
		return
	}
	a.audit(r, auditAction, http.StatusOK, map[string]string{"masterId": masterID})
	m, _ := a.Reg.GetWithOnline(masterID, a.onlineFn())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "master": m})
}

func (a *API) audit(r *http.Request, action string, status int, meta map[string]string) {
	if a.Audit == nil {
		return
	}
	a.Audit.Record(action, r.Method, r.URL.Path, audit.ClientIP(r), status, meta)
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "mreq_" + hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "code": code})
}
