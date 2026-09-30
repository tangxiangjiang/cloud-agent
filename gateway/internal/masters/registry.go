// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package masters

import (
	"sync"
	"time"
)

type masterRec struct {
	masterID string
	name     string
	online   bool
	slaves   map[string]*SlaveMirror
	updated  time.Time
	timer    *time.Timer
}

// Registry mirrors Master config + process state. gatewayOnline is filled at read time.
type Registry struct {
	mu    sync.Mutex
	byID  map[string]*masterRec
	grace time.Duration
}

func NewRegistry() *Registry {
	return &Registry{
		byID:  make(map[string]*masterRec),
		grace: 3 * time.Second,
	}
}

func (r *Registry) SetGrace(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grace = d
}

func (r *Registry) UpsertRegister(masterID, name string, reports []SlaveReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[masterID]
	if rec == nil {
		rec = &masterRec{
			masterID: masterID,
			slaves:   make(map[string]*SlaveMirror),
		}
		r.byID[masterID] = rec
	}
	if rec.timer != nil {
		rec.timer.Stop()
		rec.timer = nil
	}
	rec.name = name
	rec.online = true
	rec.updated = time.Now().UTC()
	for _, rep := range reports {
		r.applyReportLocked(rec, rep)
	}
}

func (r *Registry) ApplyReports(masterID string, reports []SlaveReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[masterID]
	if rec == nil {
		rec = &masterRec{
			masterID: masterID,
			slaves:   make(map[string]*SlaveMirror),
			online:   true,
		}
		r.byID[masterID] = rec
	}
	rec.updated = time.Now().UTC()
	for _, rep := range reports {
		r.applyReportLocked(rec, rep)
	}
}

func (r *Registry) applyReportLocked(rec *masterRec, rep SlaveReport) {
	if rep.ID == "" {
		return
	}
	cur := rec.slaves[rep.ID]
	if cur == nil {
		cur = &SlaveMirror{ID: rep.ID, Enabled: true, Process: ProcessStopped}
		rec.slaves[rep.ID] = cur
	}
	if rep.Name != "" {
		cur.Name = rep.Name
	}
	if rep.Enabled != nil {
		cur.Enabled = *rep.Enabled
	} else if rep.Desired != nil {
		cur.Enabled = *rep.Desired
	}
	if rep.Project != nil {
		cur.Project = *rep.Project
	}
	if rep.Process != "" {
		cur.Process = rep.Process
	}
	cur.Pid = rep.Pid
	cur.LastError = rep.LastError
	if rep.StartedAt != nil {
		cur.StartedAt = rep.StartedAt
	}
}

func (r *Registry) Touch(masterID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec := r.byID[masterID]; rec != nil {
		rec.updated = time.Now().UTC()
	}
}

func (r *Registry) ScheduleOffline(masterID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[masterID]
	if rec == nil {
		return
	}
	if rec.timer != nil {
		rec.timer.Stop()
	}
	grace := r.grace
	rec.timer = time.AfterFunc(grace, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if cur := r.byID[masterID]; cur != nil {
			cur.online = false
			cur.timer = nil
		}
	})
}

func (r *Registry) IsOnline(masterID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[masterID]
	return rec != nil && rec.online
}

func (r *Registry) List() []MasterMirror {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]MasterMirror, 0, len(r.byID))
	for _, rec := range r.byID {
		out = append(out, r.cloneLocked(rec, nil))
	}
	return out
}

func (r *Registry) Get(masterID string) (MasterMirror, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[masterID]
	if rec == nil {
		return MasterMirror{}, false
	}
	return r.cloneLocked(rec, nil), true
}

// GetWithOnline merges child slave gatewayOnline from onlineFn(slaveId).
func (r *Registry) GetWithOnline(masterID string, onlineFn func(slaveID string) bool) (MasterMirror, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.byID[masterID]
	if rec == nil {
		return MasterMirror{}, false
	}
	return r.cloneLocked(rec, onlineFn), true
}

func (r *Registry) ListWithOnline(onlineFn func(slaveID string) bool) []MasterMirror {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]MasterMirror, 0, len(r.byID))
	for _, rec := range r.byID {
		out = append(out, r.cloneLocked(rec, onlineFn))
	}
	return out
}

func (r *Registry) cloneLocked(rec *masterRec, onlineFn func(string) bool) MasterMirror {
	slaves := make([]SlaveMirror, 0, len(rec.slaves))
	for _, s := range rec.slaves {
		cp := *s
		if onlineFn != nil {
			cp.GatewayOnline = onlineFn(s.ID)
		}
		slaves = append(slaves, cp)
	}
	m := MasterMirror{
		MasterID: rec.masterID,
		Name:     rec.name,
		Online:   rec.online,
		Slaves:   slaves,
	}
	if !rec.updated.IsZero() {
		m.Updated = rec.updated.Format(time.RFC3339Nano)
	}
	return m
}
