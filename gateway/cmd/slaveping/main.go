// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Command slaveping is a minimal outbound Slave that registers with Gateway and heartbeats.
//
//	go run ./cmd/slaveping -gateway ws://127.0.0.1:8080/v1/slave/ws -token <bearer> -id slave_devpc
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	gateway := flag.String("gateway", envOr("SLAVE_GATEWAY_WS", "ws://127.0.0.1:8080/v1/slave/ws"), "Gateway slave WebSocket URL")
	token := flag.String("token", envOr("SLAVE_TOKEN", ""), "Gateway bearer token from /v1/auth/pair")
	id := flag.String("id", envOr("SLAVE_ID", "slave_devpc"), "slave id")
	name := flag.String("name", "slaveping", "display name")
	cwd := flag.String("cwd", "/path/to/your/cloud-agent", "example repo cwd (placeholder ok)")
	flag.Parse()
	if *token == "" {
		log.Fatal("-token required (pair with Gateway first)")
	}

	conn, _, err := websocket.DefaultDialer.Dial(*gateway, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	must := func(v any) {
		if err := conn.WriteJSON(v); err != nil {
			log.Fatal(err)
		}
	}
	read := func() map[string]any {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		log.Printf("<- %v", m)
		return m
	}

	must(map[string]string{"type": "auth", "token": *token})
	if read()["type"] != "auth.ok" {
		log.Fatal("auth failed")
	}
	must(map[string]any{
		"type":    "register",
		"slaveId": *id,
		"name":    *name,
		"repos": []map[string]string{
			{"id": "r_cloud_agent", "name": "cloud-agent", "cwd": *cwd},
		},
	})
	if read()["type"] != "registered" {
		log.Fatal("register failed")
	}
	log.Printf("registered as %s; heartbeating (Ctrl+C to exit)", *id)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ticker.C:
			must(map[string]string{"type": "heartbeat"})
		case <-interrupt:
			return
		}
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
