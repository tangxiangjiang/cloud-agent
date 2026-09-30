// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import WebSocket from "ws";
import {
  deleteSlaveEntry,
  saveMasterConfig,
  upsertSlaveEntry,
} from "./config.js";
import { MasterControlError } from "./errors.js";
import type { ProcessManager } from "./process-manager.js";
import type { MasterConfig, MasterSlaveEntry, SlaveRuntimeSnapshot } from "./types.js";

export type GatewayClientOptions = {
  gatewayUrl: string;
  token: string;
  masterConfigPath: string;
  getConfig: () => MasterConfig;
  setConfig: (cfg: MasterConfig) => void;
  pm: ProcessManager;
  log?: (msg: string, meta?: Record<string, unknown>) => void;
};

function snapToReport(s: SlaveRuntimeSnapshot): Record<string, unknown> {
  return {
    id: s.id,
    name: s.name,
    enabled: s.enabled,
    desired: s.enabled,
    project: s.project,
    process: s.state,
    pid: s.pid,
    lastError: s.lastError,
    startedAt: s.startedAt,
  };
}

/**
 * Outbound Master control-plane client (Gateway /v1/master/ws).
 */
export class MasterGatewayClient {
  private ws: WebSocket | null = null;
  private stopped = false;
  private heartbeat: ReturnType<typeof setInterval> | null = null;
  private readonly opts: GatewayClientOptions;

  constructor(opts: GatewayClientOptions) {
    this.opts = opts;
  }

  start(): void {
    this.stopped = false;
    this.connect();
  }

  stop(): void {
    this.stopped = true;
    if (this.heartbeat) {
      clearInterval(this.heartbeat);
      this.heartbeat = null;
    }
    this.ws?.close();
    this.ws = null;
  }

  /** Push full slaves.report (after local start/stop/config change). */
  reportAll(): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return;
    const slaves = this.opts.pm.listSnapshots().map(snapToReport);
    this.send({ type: "master.slaves.report", slaves });
  }

  private log(msg: string, meta?: Record<string, unknown>): void {
    this.opts.log?.(msg, meta);
  }

  private send(obj: Record<string, unknown>): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(obj));
    }
  }

  private connect(): void {
    if (this.stopped) return;
    const url = this.opts.gatewayUrl;
    this.log("master connecting", { url });
    const ws = new WebSocket(url);
    this.ws = ws;

    ws.on("open", () => {
      this.send({ type: "auth", token: this.opts.token });
    });

    ws.on("message", (data) => {
      let msg: Record<string, unknown>;
      try {
        msg = JSON.parse(String(data)) as Record<string, unknown>;
      } catch {
        return;
      }
      void this.onMessage(msg);
    });

    ws.on("close", () => {
      if (this.heartbeat) {
        clearInterval(this.heartbeat);
        this.heartbeat = null;
      }
      if (!this.stopped) {
        setTimeout(() => this.connect(), 2000);
      }
    });

    ws.on("error", (err) => {
      this.log("master ws error", {
        error: err instanceof Error ? err.message : String(err),
      });
    });
  }

  private async onMessage(msg: Record<string, unknown>): Promise<void> {
    const type = msg.type;
    if (type === "auth.ok") {
      const cfg = this.opts.getConfig();
      this.send({
        type: "master.register",
        masterId: cfg.masterId,
        name: cfg.name,
        slaves: this.opts.pm.listSnapshots().map(snapToReport),
      });
      return;
    }
    if (type === "master.registered") {
      this.log("master registered", { masterId: msg.masterId });
      if (this.heartbeat) clearInterval(this.heartbeat);
      this.heartbeat = setInterval(() => {
        this.send({ type: "master.heartbeat" });
      }, 25_000);
      return;
    }
    if (type === "master.config.list") {
      this.reportAll();
      return;
    }
    if (type === "master.config.upsert") {
      await this.handleUpsert(msg);
      return;
    }
    if (type === "master.config.delete") {
      await this.handleDelete(msg);
      return;
    }
    if (type === "master.control") {
      await this.handleControl(msg);
    }
  }

  private replyOk(kind: "config" | "control", requestId: string): void {
    this.send({
      type: kind === "config" ? "master.config.ok" : "master.control.ok",
      requestId,
    });
  }

  private replyErr(
    kind: "config" | "control",
    requestId: string,
    err: unknown,
  ): void {
    const code =
      err instanceof MasterControlError ? err.code : "error";
    const error = err instanceof Error ? err.message : String(err);
    this.send({
      type: kind === "config" ? "master.config.error" : "master.control.error",
      requestId,
      code,
      error,
    });
  }

  private async handleUpsert(msg: Record<string, unknown>): Promise<void> {
    const requestId = String(msg.requestId ?? "");
    try {
      const raw = msg.slave as Record<string, unknown>;
      if (!raw || typeof raw !== "object") {
        throw new Error("slave required");
      }
      const project = raw.project as Record<string, unknown>;
      if (!project) throw new Error("slave.project required");
      const entry: MasterSlaveEntry = {
        id: String(raw.id),
        ...(raw.name != null && String(raw.name) !== ""
          ? { name: String(raw.name) }
          : {}),
        enabled: raw.enabled !== false,
        project: {
          id: String(project.id),
          name: String(project.name),
          cwd: String(project.cwd),
          ...(project.index ? { index: String(project.index) } : {}),
        },
      };
      const next = upsertSlaveEntry(this.opts.getConfig(), entry);
      saveMasterConfig(this.opts.masterConfigPath, next);
      this.opts.setConfig(next);
      this.opts.pm.applyConfig(next);
      this.reportAll();
      this.replyOk("config", requestId);
    } catch (err) {
      this.replyErr("config", requestId, err);
    }
  }

  private async handleDelete(msg: Record<string, unknown>): Promise<void> {
    const requestId = String(msg.requestId ?? "");
    const slaveId = String(msg.slaveId ?? "");
    try {
      const snap = this.opts.pm.snapshot(slaveId);
      if (snap.state !== "stopped") {
        await this.opts.pm.stop(slaveId);
      }
      const next = deleteSlaveEntry(this.opts.getConfig(), slaveId);
      saveMasterConfig(this.opts.masterConfigPath, next);
      this.opts.setConfig(next);
      this.opts.pm.applyConfig(next);
      this.reportAll();
      this.replyOk("config", requestId);
    } catch (err) {
      this.replyErr("config", requestId, err);
    }
  }

  private async handleControl(msg: Record<string, unknown>): Promise<void> {
    const requestId = String(msg.requestId ?? "");
    const slaveId = String(msg.slaveId ?? "");
    const action = String(msg.action ?? "");
    try {
      if (action === "start") await this.opts.pm.start(slaveId);
      else if (action === "stop") await this.opts.pm.stop(slaveId);
      else if (action === "restart") await this.opts.pm.restart(slaveId);
      else throw new Error(`unknown action: ${action}`);
      this.reportAll();
      this.replyOk("control", requestId);
    } catch (err) {
      this.replyErr("control", requestId, err);
    }
  }
}
