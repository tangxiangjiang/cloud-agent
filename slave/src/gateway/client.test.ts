// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { createServer } from "node:http";
import path from "node:path";
import { after, describe, it } from "node:test";
import { WebSocketServer, type WebSocket } from "ws";
import { redact } from "../log.js";
import { GatewayClient } from "./client.js";
import type { AssignedTask } from "./types.js";

const absCwd = path.resolve("/tmp/cloud-agent-fixture");

describe("GatewayClient", () => {
  const httpServer = createServer();
  const wss = new WebSocketServer({ server: httpServer, path: "/v1/slave/ws" });
  let port = 0;

  const sockets: WebSocket[] = [];
  let registerCount = 0;
  const events: Array<{ taskId: string; kind: string }> = [];

  wss.on("connection", (ws) => {
    sockets.push(ws);
    ws.on("message", (raw) => {
      const msg = JSON.parse(raw.toString()) as Record<string, unknown>;
      switch (msg.type) {
        case "auth":
          assert.equal(msg.token, "secret-test-token-value-abcdefgh");
          ws.send(JSON.stringify({ type: "auth.ok" }));
          break;
        case "register":
          registerCount++;
          assert.equal(msg.slaveId, "slave_test");
          ws.send(JSON.stringify({ type: "registered", slaveId: "slave_test" }));
          break;
        case "heartbeat":
          ws.send(JSON.stringify({ type: "heartbeat.ok" }));
          break;
        case "task.event": {
          const ev = msg.event as { kind?: string };
          events.push({ taskId: String(msg.taskId), kind: String(ev?.kind) });
          break;
        }
        default:
          break;
      }
    });
  });

  after(async () => {
    await new Promise<void>((r) => wss.close(() => r()));
    await new Promise<void>((r) => httpServer.close(() => r()));
  });

  it("registers, emits status on assign, re-registers after disconnect", async () => {
    await new Promise<void>((resolve) => {
      httpServer.listen(0, "127.0.0.1", () => resolve());
    });
    const addr = httpServer.address();
    assert.ok(addr && typeof addr === "object");
    port = addr.port;

    const assigned: AssignedTask[] = [];
    const client = new GatewayClient({
      url: `ws://127.0.0.1:${port}/v1/slave/ws`,
      token: "secret-test-token-value-abcdefgh",
      slaveId: "slave_test",
      name: "test",
      repos: [{ id: "r1", name: "fixture", cwd: absCwd }],
      heartbeatMs: 60_000,
      reconnectMs: 200,
      maxReconnectMs: 500,
      handlers: {
        onAssign(task, emit) {
          assigned.push(task);
          emit(task.id, "status", { status: "running" });
        },
        onCancel(taskId, emit) {
          emit(taskId, "done", { status: "cancelled" });
        },
      },
    });

    client.start();

    // Wait until registered
    await waitFor(() => client.isRegistered, 5000);
    assert.equal(registerCount, 1);

    // Assign a task on the live socket
    const live = sockets[sockets.length - 1];
    assert.ok(live);
    live.send(
      JSON.stringify({
        type: "task.assign",
        task: {
          id: "task_1",
          repoId: "r1",
          prompt: "hi",
          status: "queued",
        },
      }),
    );

    await waitFor(() => events.some((e) => e.kind === "status"), 5000);
    assert.equal(assigned[0]?.id, "task_1");
    assert.ok(events.some((e) => e.taskId === "task_1" && e.kind === "status"));

    // Force disconnect → reconnect + re-register
    live.close();
    await waitFor(() => registerCount >= 2, 5000);
    assert.ok(client.isRegistered);

    await client.stop();
  });

  it("redacts long tokens in log helper", () => {
    const secret = "secret-test-token-value-abcdefghij";
    assert.equal(redact(secret), "***");
    assert.ok(!redact(`Bearer ${secret}`).includes(secret));
  });
});

async function waitFor(pred: () => boolean, ms: number): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < ms) {
    if (pred()) return;
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error("waitFor timeout");
}
