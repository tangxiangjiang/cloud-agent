package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/tangxiangjiang/cloud-agent/gateway/internal/auth"
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

	store := auth.NewStore(pairCode)
	log.Printf("pair code: %s (use POST /v1/auth/pair)", store.PairCode())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", handleHealth)
	mux.HandleFunc("POST /v1/auth/pair", store.HandlePair)
	mux.Handle("GET /v1/auth/me", store.Middleware(http.HandlerFunc(handleMe)))

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
