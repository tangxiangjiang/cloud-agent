// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package masters

// ProcessState mirrors Master process table (doc/slave-master.md §3).
type ProcessState string

const (
	ProcessStopped  ProcessState = "stopped"
	ProcessStarting ProcessState = "starting"
	ProcessRunning  ProcessState = "running"
	ProcessStopping ProcessState = "stopping"
	ProcessError    ProcessState = "error"
)

// Project is the single project bound to a child Slave.
type Project struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cwd   string `json:"cwd"`
	Index string `json:"index,omitempty"`
}

// SlaveMirror is Gateway's mirrored view of one Master-managed Slave.
type SlaveMirror struct {
	ID            string       `json:"id"`
	Name          string       `json:"name,omitempty"`
	Enabled       bool         `json:"enabled"`
	Project       Project      `json:"project"`
	Process       ProcessState `json:"process"`
	Pid           *int         `json:"pid"`
	LastError     *string      `json:"lastError"`
	StartedAt     *string      `json:"startedAt,omitempty"`
	GatewayOnline bool         `json:"gatewayOnline"`
}

// MasterMirror is the App-facing Master record.
type MasterMirror struct {
	MasterID string        `json:"masterId"`
	Name     string        `json:"name,omitempty"`
	Online   bool          `json:"online"`
	Slaves   []SlaveMirror `json:"slaves"`
	Updated  string        `json:"updatedAt,omitempty"`
}

// SlaveReport is one entry in master.slaves.report / register.
type SlaveReport struct {
	ID        string       `json:"id"`
	Name      string       `json:"name,omitempty"`
	Enabled   *bool        `json:"enabled,omitempty"`
	Desired   *bool        `json:"desired,omitempty"` // alias for enabled
	Project   *Project     `json:"project,omitempty"`
	Process   ProcessState `json:"process,omitempty"`
	Pid       *int         `json:"pid"`
	LastError *string      `json:"lastError"`
	StartedAt *string      `json:"startedAt,omitempty"`
}
