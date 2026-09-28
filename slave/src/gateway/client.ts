// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import WebSocket from "ws";
import type { RepoConfig } from "../config.js";
import { log } from "../log.js";
import type { AssignedTask, InboundMessage, TaskHandlers } from "./types.js";

export interface GatewayClientOptions {
  url: string;
  /** Gateway Bearer — never logged in full. */
  token: string;
  slaveId: string;
  name?: string;
  repos: RepoConfig[];
  handlers: TaskHandlers;
  heartbeatMs?: number;
  /** Initial reconnect delay; doubles up to maxReconnectMs. */
  reconnectMs?: number;
  maxReconnectMs?: number;
}

/**
 * Outbound Slave → Gateway WebSocket client.
 * Auth → register → heartbeat; receives assign/cancel; emits task.event.
 */
export class GatewayClient {
  private readonly opts: Required<
    Pick<GatewayClientOptions, "heartbeatMs" | "reconnectMs" | "maxReconnectMs">
  > &
    GatewayClientOptions;

  private ws: WebSocket | null = null;
  private stopped = false;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private backoff: number;
  private registered = false;
  private connectGeneration = 0;

  constructor(opts: GatewayClientOptions) {
    this.opts = {
      ...opts,
      heartbeatMs: opts.heartbeatMs ?? 15_000,
      reconnectMs: opts.reconnectMs ?? 1_000,
      maxReconnectMs: opts.maxReconnectMs ?? 30_000,
    };
    this.backoff = this.opts.reconnectMs;
  }

  /** Begin connect loop (returns immediately). */
  start(): void {
    this.stopped = false;
    void this.connect();
  }

  async stop(): Promise<void> {
    this.stopped = true;
    this.clearHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    const ws = this.ws;
    this.ws = null;
    if (ws) {
      await new Promise<void>((resolve) => {
        ws.once("close", () => resolve());
        try {
          ws.close();
        } catch {
          resolve();
        }
        setTimeout(resolve, 500);
      });
    }
  }

  get isRegistered(): boolean {
    return this.registered;
  }

  private connect(): void {
    if (this.stopped) return;
    const gen = ++this.connectGeneration;
    this.registered = false;

    log.info("connecting to gateway", {
      url: this.opts.url,
      slaveId: this.opts.slaveId,
    });

    let ws: WebSocket;
    try {
      ws = new WebSocket(this.opts.url);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      log.error("websocket create failed", { error: msg });
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;

    ws.on("open", () => {
      if (gen !== this.connectGeneration || this.stopped) return;
      log.info("gateway socket open");
      // Auth via first frame (token not logged).
      this.send({ type: "auth", token: this.opts.token });
    });

    ws.on("message", (data) => {
      if (gen !== this.connectGeneration || this.stopped) return;
      void this.onMessage(data);
    });

    ws.on("close", (code, reason) => {
      if (gen !== this.connectGeneration) return;
      this.clearHeartbeat();
      this.registered = false;
      this.ws = null;
      log.warn("gateway disconnected", {
        code,
        reason: reason.toString() || undefined,
      });
      this.scheduleReconnect();
    });

    ws.on("error", (err) => {
      log.warn("gateway socket error", { error: err.message });
    });
  }

  private async onMessage(data: WebSocket.RawData): Promise<void> {
    let msg: InboundMessage;
    try {
      msg = JSON.parse(data.toString()) as InboundMessage;
    } catch {
      log.warn("invalid json from gateway");
      return;
    }

    switch (msg.type) {
      case "auth.ok":
        log.info("gateway auth ok");
        this.sendRegister();
        break;
      case "registered":
        this.registered = true;
        this.backoff = this.opts.reconnectMs;
        log.info("registered with gateway", {
          slaveId: (msg as { slaveId?: string }).slaveId ?? this.opts.slaveId,
        });
        this.startHeartbeat();
        break;
      case "heartbeat.ok":
      case "pong":
        log.debug(`gateway ${msg.type}`);
        break;
      case "task.assign": {
        const task = (msg as { task?: AssignedTask }).task;
        if (!task?.id) {
          log.warn("task.assign missing task.id");
          break;
        }
        log.info("task assigned", { taskId: task.id, repoId: task.repoId ?? null });
        try {
          await this.opts.handlers.onAssign(task, (taskId, kind, payload) =>
            this.emitEvent(taskId, kind, payload),
          );
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          log.error("task handler failed", { taskId: task.id, error: message });
          this.emitEvent(task.id, "error", { message });
          this.emitEvent(task.id, "done", { status: "error" });
        }
        break;
      }
      case "task.cancel": {
        const taskId = (msg as { taskId?: string }).taskId;
        if (!taskId) {
          log.warn("task.cancel missing taskId");
          break;
        }
        log.info("task cancel requested", { taskId });
        try {
          await this.opts.handlers.onCancel(taskId, (id, kind, payload) =>
            this.emitEvent(id, kind, payload),
          );
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          log.error("cancel handler failed", { taskId, error: message });
        }
        break;
      }
      case "error":
        log.error("gateway error", {
          error: (msg as { error?: string }).error ?? "unknown",
        });
        break;
      default:
        log.debug("gateway message", { type: msg.type });
    }
  }

  private sendRegister(): void {
    const body: Record<string, unknown> = {
      type: "register",
      slaveId: this.opts.slaveId,
      repos: this.opts.repos.map((r) => ({
        id: r.id,
        name: r.name,
        cwd: r.cwd,
      })),
    };
    if (this.opts.name) {
      body.name = this.opts.name;
    }
    this.send(body);
  }

  private emitEvent(
    taskId: string,
    kind: string,
    payload: Record<string, unknown>,
  ): void {
    this.send({
      type: "task.event",
      taskId,
      event: { kind, payload },
    });
  }

  private send(obj: Record<string, unknown>): void {
    const ws = this.ws;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      log.warn("send skipped; socket not open", { type: String(obj.type ?? "") });
      return;
    }
    // Never log auth frames (contain token).
    if (obj.type !== "auth") {
      log.debug("→ gateway", { type: String(obj.type ?? "") });
    }
    ws.send(JSON.stringify(obj));
  }

  private startHeartbeat(): void {
    this.clearHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      this.send({ type: "heartbeat" });
    }, this.opts.heartbeatMs);
  }

  private clearHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private scheduleReconnect(): void {
    if (this.stopped) return;
    if (this.reconnectTimer) return;
    const delay = this.backoff;
    this.backoff = Math.min(this.backoff * 2, this.opts.maxReconnectMs);
    log.info("reconnecting", { delayMs: delay });
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }
}
