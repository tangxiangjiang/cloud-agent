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
	"github.com/tangxiangjiang/cloud-agent/gateway/internal/task"
)

func main() {
	addr := flag.String("addr", envOr("GATEWAY_ADDR", ":8080"), "HTTP listen address (env GATEWAY_ADDR)")
	pairCodeFlag := flag.String("pair-code", envOr("GATEWAY_PAIR_CODE", ""), "pairing code (env GATEWAY_PAIR_CODE); generated if empty")
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
	log.Printf("pair code: %s (use POST /v1/auth/pair)", authStore.PairCode())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", handleHealth)
	mux.HandleFunc("POST /v1/auth/pair", authStore.HandlePair)
	mux.Handle("GET /v1/auth/me", authStore.Middleware(http.HandlerFunc(handleMe)))
	taskHandler := authStore.Middleware(taskStore.Handler())
	mux.Handle("/v1/tasks", taskHandler)
	mux.Handle("/v1/tasks/", taskHandler)

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
