// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package masters

import (
	"sync"
	"time"
)

type pendingResult struct {
	OK    bool
	Error string
	Code  string
}

type pendingWaiter struct {
	ch chan pendingResult
}

// Pending tracks requestId waiters for sync HTTP ↔ Master WS.
type Pending struct {
	mu sync.Mutex
	m  map[string]*pendingWaiter
}

func NewPending() *Pending {
	return &Pending{m: make(map[string]*pendingWaiter)}
}

// Register creates a waiter. Caller must Complete or let Wait timeout cleanup.
func (p *Pending) Register(requestID string) <-chan pendingResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan pendingResult, 1)
	p.m[requestID] = &pendingWaiter{ch: ch}
	return ch
}

func (p *Pending) Complete(requestID string, ok bool, errMsg, code string) {
	p.mu.Lock()
	w := p.m[requestID]
	delete(p.m, requestID)
	p.mu.Unlock()
	if w == nil {
		return
	}
	select {
	case w.ch <- pendingResult{OK: ok, Error: errMsg, Code: code}:
	default:
	}
}

func (p *Pending) Cancel(requestID string) {
	p.mu.Lock()
	w := p.m[requestID]
	delete(p.m, requestID)
	p.mu.Unlock()
	if w != nil {
		close(w.ch)
	}
}

// Wait blocks until Complete or timeout. Returns ok, error message, code, timedOut.
func (p *Pending) Wait(ch <-chan pendingResult, timeout time.Duration) (ok bool, errMsg, code string, timedOut bool) {
	select {
	case res, open := <-ch:
		if !open {
			return false, "cancelled", "", false
		}
		return res.OK, res.Error, res.Code, false
	case <-time.After(timeout):
		return false, "master_control_timeout", "master_control_timeout", true
	}
}
