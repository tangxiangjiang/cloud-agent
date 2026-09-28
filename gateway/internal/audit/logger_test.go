// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package audit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/audit"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ratelimit"
)

func TestAuditRedactsTokens(t *testing.T) {
	var buf strings.Builder
	l := audit.NewLogger(10, &buf)
	l.Record("pair", "POST", "/v1/auth/pair", "1.2.3.4", 200, map[string]string{
		"token":         "supersecrettokenvalue",
		"authorization": "Bearer abcdefghijklmnop",
	})
	out := buf.String()
	if strings.Contains(out, "supersecrettokenvalue") {
		t.Fatalf("token leaked: %s", out)
	}
	if strings.Contains(out, "abcdefghijklmnop") {
		t.Fatalf("auth leaked: %s", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("expected redaction: %s", out)
	}
	evs := l.Recent(1)
	if len(evs) != 1 || evs[0].Action != "pair" {
		t.Fatalf("ring: %+v", evs)
	}
}

func TestRateLimit(t *testing.T) {
	lim := ratelimit.New(0, 2) // defaults window, max 2
	if !lim.Allow("a") || !lim.Allow("a") {
		t.Fatal("first two should pass")
	}
	if lim.Allow("a") {
		t.Fatal("third should fail")
	}
	if !lim.Allow("b") {
		t.Fatal("other key ok")
	}

	h := lim.MiddlewareFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	// exhaust
	lim.Allow("127.0.0.1")
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	// Use a fresh limiter for HTTP test
	lim2 := ratelimit.New(0, 1)
	h2 := lim2.MiddlewareFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	h2.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("first: %d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec2.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec2.Body).Decode(&body)
	if body["error"] == "" {
		t.Fatal("expected error json")
	}
	_ = h // silence
}
