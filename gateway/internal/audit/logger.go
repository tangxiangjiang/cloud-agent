// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Event is one audit record. Never include full Authorization or API keys.
type Event struct {
	At     string            `json:"at"`
	Action string            `json:"action"`
	Method string            `json:"method,omitempty"`
	Path   string            `json:"path,omitempty"`
	IP     string            `json:"ip,omitempty"`
	Status int               `json:"status,omitempty"`
	Meta   map[string]string `json:"meta,omitempty"`
}

// Logger keeps a ring buffer and optional file sink.
type Logger struct {
	mu     sync.Mutex
	ring   []Event
	max    int
	sink   io.Writer
	now    func() time.Time
}

func NewLogger(max int, sink io.Writer) *Logger {
	if max <= 0 {
		max = 500
	}
	if sink == nil {
		sink = os.Stderr
	}
	return &Logger{
		ring: make([]Event, 0, max),
		max:  max,
		sink: sink,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// OpenFileSink opens path for append; empty path → stderr only.
func OpenFileSink(path string) (io.Writer, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return os.Stderr, nil
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}

func (l *Logger) Record(action, method, path, ip string, status int, meta map[string]string) {
	ev := Event{
		At:     l.now().Format(time.RFC3339Nano),
		Action: action,
		Method: method,
		Path:   path,
		IP:     ip,
		Status: status,
		Meta:   sanitizeMeta(meta),
	}
	l.mu.Lock()
	l.ring = append(l.ring, ev)
	if len(l.ring) > l.max {
		l.ring = l.ring[len(l.ring)-l.max:]
	}
	sink := l.sink
	l.mu.Unlock()

	if sink != nil {
		b, _ := json.Marshal(ev)
		_, _ = sink.Write(append(b, '\n'))
	}
}

func (l *Logger) Recent(limit int) []Event {
	if limit <= 0 {
		limit = 50
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.ring)
	if n == 0 {
		return nil
	}
	if limit > n {
		limit = n
	}
	out := make([]Event, limit)
	copy(out, l.ring[n-limit:])
	return out
}

func (l *Logger) HandleList(w http.ResponseWriter, r *http.Request) {
	limit := 50
	evs := l.Recent(limit)
	if evs == nil {
		evs = []Event{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"events": evs})
}

// ClientIP extracts a coarse client key (no auth secrets).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		return host[:i]
	}
	return host
}

func sanitizeMeta(meta map[string]string) map[string]string {
	if meta == nil {
		return nil
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "authorization") ||
			strings.Contains(lk, "token") ||
			strings.Contains(lk, "apikey") ||
			strings.Contains(lk, "api_key") ||
			strings.Contains(lk, "cursor") {
			out[k] = redact(v)
			continue
		}
		out[k] = v
	}
	return out
}

func redact(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	if len(s) <= 6 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

// RedactHeaderValue never returns a full bearer token.
func RedactHeaderValue(h string) string {
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return "Bearer " + redact(strings.TrimSpace(strings.TrimPrefix(h, p)))
	}
	return redact(h)
}
