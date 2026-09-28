// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/audit"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ratelimit"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ws"
)

func main() {
	addr := flag.String("addr", envOr("GATEWAY_ADDR", ":8080"), "HTTP listen address (env GATEWAY_ADDR)")
	pairCodeFlag := flag.String("pair-code", envOr("GATEWAY_PAIR_CODE", ""), "pairing code (env GATEWAY_PAIR_CODE); generated if empty")
	configPath := flag.String("config", envOr("GATEWAY_CONFIG", ""), "optional YAML config path (env GATEWAY_CONFIG); see config.example.yaml")
	auditPath := flag.String("audit-log", envOr("GATEWAY_AUDIT_LOG", ""), "audit JSONL path (env GATEWAY_AUDIT_LOG); default stderr")
	debug := flag.Bool("debug", envOr("GATEWAY_DEBUG", "") == "1", "enable debug event inject endpoint")
	flag.Parse()

	pairCode := *pairCodeFlag
	if pairCode == "" {
		var err error
		pairCode, err = auth.GeneratePairCode()
		if err != nil {
			log.Fatal(err)
		}
	}

	sink, err := audit.OpenFileSink(*auditPath)
	if err != nil {
		log.Fatalf("audit log: %v", err)
	}
	auditLog := audit.NewLogger(1000, sink)
	pairLimit := ratelimit.New(0, 10)   // 10 / min / IP
	reviseLimit := ratelimit.New(0, 30) // 30 / min / IP

	authStore := auth.NewStore(pairCode)
	taskStore := task.NewStore()
	wfStore := workflow.NewStore()
	hub := ws.NewHub(authStore)
	taskStore.SetEventSource(hub)

	var slaveList []slaves.Slave
	if *configPath != "" {
		cfg, err := slaves.LoadConfigFile(*configPath)
		if err != nil {
			log.Fatalf("load config %s: %v", *configPath, err)
		}
		slaveList = cfg.Slaves
		log.Printf("loaded %d slave(s) from %s", len(slaveList), *configPath)
	} else {
		log.Printf("no -config set; GET /v1/slaves returns empty list (see config.example.yaml)")
	}
	slaveReg := slaves.NewRegistry(slaveList)
	slaveHub := slaves.NewOutboundHub(authStore, slaveReg, taskStore, hub)
	taskStore.SetDispatcher(slaveHub)
	wfStore.SetStarter(slaveHub)

	log.Printf("pair code: %s (use POST /v1/auth/pair)", authStore.PairCode())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", handleHealth)

	pairHandler := pairLimit.MiddlewareFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		authStore.HandlePair(rw, r)
		auditLog.Record("pair", r.Method, r.URL.Path, audit.ClientIP(r), rw.status, nil)
	})
	mux.HandleFunc("POST /v1/auth/pair", pairHandler)

	mux.Handle("GET /v1/auth/me", authStore.Middleware(http.HandlerFunc(handleMe)))
	mux.Handle("GET /v1/slaves", authStore.Middleware(http.HandlerFunc(slaveReg.HandleList)))
	mux.Handle("GET /v1/audit", authStore.Middleware(http.HandlerFunc(auditLog.HandleList)))

	taskHandler := authStore.Middleware(auditTasks(auditLog, taskStore.Handler()))
	mux.Handle("/v1/tasks", taskHandler)
	mux.Handle("/v1/tasks/", taskHandler)

	wfInner := limitPathSuffix(reviseLimit, "/revise", auditWorkflows(auditLog, wfStore.Handler()))
	wfHandler := authStore.Middleware(wfInner)
	mux.Handle("/v1/workflows", wfHandler)
	mux.Handle("/v1/workflows/", wfHandler)

	mux.HandleFunc("GET /v1/ws", hub.HandleWS)
	mux.HandleFunc("GET /v1/slave/ws", slaveHub.HandleWS)

	if *debug {
		log.Printf("debug inject enabled: POST /v1/debug/tasks/{id}/events")
		mux.Handle("POST /v1/debug/tasks/{id}/events", authStore.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Kind    string         `json:"kind"`
				Payload map[string]any `json:"payload"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Kind == "" {
				http.Error(w, `{"error":"kind required"}`, http.StatusBadRequest)
				return
			}
			env := hub.Publish(r.PathValue("id"), body.Kind, body.Payload)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(env)
		})))
	}

	log.Printf("gateway listening on %s (audit → %s)", *addr, auditSinkLabel(*auditPath))
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func auditTasks(a *audit.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		action := "task"
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cancel") {
			action = "task.cancel"
		} else if r.Method == http.MethodPost && r.URL.Path == "/v1/tasks" {
			action = "task.create"
		}
		a.Record(action, r.Method, r.URL.Path, audit.ClientIP(r), rw.status, nil)
	})
}

func auditWorkflows(a *audit.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		action := "workflow"
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && p == "/v1/workflows":
			action = "workflow.create"
		case strings.HasSuffix(p, "/start"):
			action = "workflow.start"
		case strings.HasSuffix(p, "/revise"):
			action = "workflow.revise"
		case strings.HasSuffix(p, "/review"):
			action = "workflow.review"
		}
		a.Record(action, r.Method, p, audit.ClientIP(r), rw.status, nil)
	})
}

func limitPathSuffix(lim *ratelimit.Limiter, suffix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, suffix) {
			key := audit.ClientIP(r)
			if !lim.Allow(key) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func auditSinkLabel(path string) string {
	if strings.TrimSpace(path) == "" {
		return "stderr"
	}
	return path
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
