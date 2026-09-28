// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

// Command mockslave registers with Gateway and mock-executes assigned tasks (no Cursor SDK).
//
//	go run ./cmd/mockslave -token <bearer> -id slave_devpc
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
	token := flag.String("token", envOr("SLAVE_TOKEN", ""), "Gateway bearer token")
	id := flag.String("id", "slave_devpc", "slave id")
	name := flag.String("name", "mockslave", "display name")
	cwd := flag.String("cwd", "/path/to/your/cloud-agent", "placeholder cwd")
	flag.Parse()
	if *token == "" {
		log.Fatal("-token required")
	}

	conn, _, err := websocket.DefaultDialer.Dial(*gateway, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	write := func(v any) {
		if err := conn.WriteJSON(v); err != nil {
			log.Fatal(err)
		}
	}

	write(map[string]string{"type": "auth", "token": *token})
	write(map[string]any{
		"type": "register", "slaveId": *id, "name": *name,
		"repos": []map[string]string{{"id": "r_cloud_agent", "name": "cloud-agent", "cwd": *cwd}},
	})

	cancelled := map[string]bool{}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			_ = conn.WriteJSON(map[string]string{"type": "heartbeat"})
		}
	}()

	go func() {
		<-interrupt
		os.Exit(0)
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg["type"] {
		case "auth.ok", "registered", "heartbeat.ok", "pong":
			log.Printf("<- %v", msg["type"])
		case "task.assign":
			taskObj, _ := msg["task"].(map[string]any)
			taskID, _ := taskObj["id"].(string)
			log.Printf("assign %s", taskID)
			emit := func(kind string, payload map[string]any) {
				write(map[string]any{
					"type": "task.event", "taskId": taskID,
					"event": map[string]any{"kind": kind, "payload": payload},
				})
			}
			emit("status", map[string]any{"status": "running"})
			time.Sleep(50 * time.Millisecond)
			if cancelled[taskID] {
				emit("done", map[string]any{"status": "cancelled"})
				continue
			}
			emit("assistant.delta", map[string]any{"text": "mock slave working…"})
			time.Sleep(50 * time.Millisecond)
			if cancelled[taskID] {
				emit("done", map[string]any{"status": "cancelled"})
				continue
			}
			emit("done", map[string]any{"status": "finished"})
		case "task.cancel":
			taskID, _ := msg["taskId"].(string)
			log.Printf("cancel %s", taskID)
			cancelled[taskID] = true
			write(map[string]any{
				"type": "task.event", "taskId": taskID,
				"event": map[string]any{"kind": "done", "payload": map[string]any{"status": "cancelled"}},
			})
		case "error":
			log.Printf("error: %v", msg)
		}
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
