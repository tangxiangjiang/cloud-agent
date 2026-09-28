// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/slaves"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/workflow"
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/ws"
)

func main() {
	addr := flag.String("addr", envOr("GATEWAY_ADDR", ":8080"), "HTTP listen address (env GATEWAY_ADDR)")
	pairCodeFlag := flag.String("pair-code", envOr("GATEWAY_PAIR_CODE", ""), "pairing code (env GATEWAY_PAIR_CODE); generated if empty")
	configPath := flag.String("config", envOr("GATEWAY_CONFIG", ""), "optional YAML config path (env GATEWAY_CONFIG); see config.example.yaml")
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
	mux.HandleFunc("POST /v1/auth/pair", authStore.HandlePair)
	mux.Handle("GET /v1/auth/me", authStore.Middleware(http.HandlerFunc(handleMe)))
	mux.Handle("GET /v1/slaves", authStore.Middleware(http.HandlerFunc(slaveReg.HandleList)))
	taskHandler := authStore.Middleware(taskStore.Handler())
	mux.Handle("/v1/tasks", taskHandler)
	mux.Handle("/v1/tasks/", taskHandler)
	wfHandler := authStore.Middleware(wfStore.Handler())
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

	log.Printf("gateway listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
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
