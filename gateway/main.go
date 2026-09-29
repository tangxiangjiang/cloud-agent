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
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/chat"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/models"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/persist"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/projectsync"
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
	stateFile := flag.String("state-file", envOr("GATEWAY_STATE_FILE", ""), "persist path: .db/.sqlite (SQLite, preferred) or .json (legacy); env GATEWAY_STATE_FILE; empty = memory only")
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
	chatLimit := ratelimit.New(0, 30)   // 30 / min / IP (chat messages)

	authStore := auth.NewStore(pairCode)
	taskStore := task.NewStore()
	wfStore := workflow.NewStore()
	chatStore := chat.NewStore(taskStore)

	var stateStore persist.Backend
	if strings.TrimSpace(*stateFile) != "" {
		var err error
		stateStore, err = persist.Open(*stateFile)
		if err != nil {
			log.Fatalf("open state %s: %v", *stateFile, err)
		}
		if snap, err := stateStore.Load(); err != nil {
			log.Fatalf("load state %s: %v", *stateFile, err)
		} else if snap != nil {
			authStore.ImportTokens(snap.Tokens)
			if err := wfStore.UnmarshalSnapshotJSON(snap.Workflows, snap.Diffs); err != nil {
				log.Fatalf("restore workflows: %v", err)
			}
			log.Printf("restored state from %s", *stateFile)
		} else {
			log.Printf("state empty/missing; starting fresh (%s)", *stateFile)
		}
		save := func() {
			wfJSON, diffJSON, err := wfStore.MarshalSnapshotJSON()
			if err != nil {
				log.Printf("state marshal: %v", err)
				return
			}
			stateStore.ScheduleSave(&persist.Snapshot{
				Tokens:    authStore.ExportTokens(),
				Workflows: wfJSON,
				Diffs:     diffJSON,
			})
		}
		authStore.SetOnChange(save)
		wfStore.SetOnChange(save)
	} else {
		log.Printf("no -state-file; workflow/auth state is memory-only (restart loses review progress)")
	}

	chatPersist := persist.AsChatSessionStore(stateStore)
	chatStore.SetPersist(chatPersist)
	if err := chatStore.LoadFromPersist(); err != nil {
		log.Fatalf("restore chats: %v", err)
	}

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

	syncStore := persist.AsProjectSyncStore(stateStore)
	syncSvc := projectsync.NewService(slaveReg, slaveHub, syncStore, auditLog)
	syncSvc.SetWorkflowSource(wfStore)
	slaveHub.SetProjectSyncResultHandler(func(slaveID, requestID, repoID string, payload json.RawMessage) error {
		return syncSvc.StoreReport(slaveID, requestID, repoID, payload)
	})

	log.Printf("pair code: %s (use POST /v1/auth/pair)", authStore.PairCode())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleRoot)
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
	mux.Handle("GET /v1/models", authStore.Middleware(models.Handler(models.FromEnv())))

	taskHandler := authStore.Middleware(auditTasks(auditLog, taskStore.Handler()))
	mux.Handle("/v1/tasks", taskHandler)
	mux.Handle("/v1/tasks/", taskHandler)

	wfInner := limitPathSuffix(reviseLimit, "/revise", auditWorkflows(auditLog, wfStore.Handler()))
	wfHandler := authStore.Middleware(wfInner)
	mux.Handle("/v1/workflows", wfHandler)
	mux.Handle("/v1/workflows/", wfHandler)

	syncSvc.Mount(mux, authStore.Middleware)

	chatInner := limitPathSuffix(chatLimit, "/messages", auditChats(auditLog, chatStore.Handler()))
	chatHandler := authStore.Middleware(chatInner)
	mux.Handle("/v1/chats", chatHandler)
	mux.Handle("/v1/chats/", chatHandler)

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
		case strings.HasSuffix(p, "/continue"):
			action = "workflow.continue"
		case strings.HasSuffix(p, "/revise"):
			action = "workflow.revise"
		case strings.HasSuffix(p, "/review"):
			action = "workflow.review"
		}
		a.Record(action, r.Method, p, audit.ClientIP(r), rw.status, nil)
	})
}

func auditChats(a *audit.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		action := "chat"
		p := r.URL.Path
		switch {
		case r.Method == http.MethodPost && p == "/v1/chats":
			action = "chat.create"
		case strings.HasSuffix(p, "/messages"):
			action = "chat.message"
		case strings.HasSuffix(p, "/assistant"):
			action = "chat.assistant"
		case strings.HasSuffix(p, "/stop"):
			action = "chat.stop"
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

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html><head><meta charset="utf-8"><title>cloud-agent Gateway</title>
<style>
body{font-family:system-ui,sans-serif;max-width:40rem;margin:2rem auto;padding:0 1rem;line-height:1.5}
code{background:#f2f2f2;padding:.1em .35em;border-radius:4px}
.ok{color:#0a7}
.warn{color:#a60}
</style></head><body>
<h1>cloud-agent Gateway</h1>
<p class="ok">Listening. Health: <a href="/v1/health"><code>/v1/health</code></a></p>
<h2>Which URL?</h2>
<ul>
<li><b>This PC browser</b>: <code>http://127.0.0.1:8080/</code> — <span class="warn">not</span> <code>10.0.2.2</code></li>
<li><b>Android emulator App</b>: Gateway URL = <code>http://10.0.2.2:8080</code> (emulator→host alias)</li>
<li><b>If emulator still fails</b>: run <code>adb reverse tcp:8080 tcp:8080</code>, then use <code>http://127.0.0.1:8080</code> in the App</li>
<li><b>Physical phone</b>: use your PC LAN IP, e.g. <code>http://192.168.x.x:8080</code></li>
</ul>
<p>Pair in the Flutter App with the pair code printed in the Gateway log (<code>pair code: …</code>). Do not put <code>CURSOR_API_KEY</code> in the App.</p>
</body></html>`))
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
