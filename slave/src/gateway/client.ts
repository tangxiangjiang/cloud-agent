// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import WebSocket from "ws";
import type { RepoConfig } from "../config.js";
import { log } from "../log.js";
import type { ProjectCatalog } from "../project/catalog.js";
import type {
  WorkflowReviewMessage,
  WorkflowReviseMessage,
  WorkflowRun,
} from "../workflow/types.js";
import type { CursorModelEntry } from "../agent/listModels.js";
import type { AssignedTask, EmitEvent, InboundMessage, TaskHandlers } from "./types.js";

export interface GatewayClientOptions {
  url: string;
  /** Gateway Bearer — never logged in full. */
  token: string;
  slaveId: string;
  name?: string;
  /** Legacy flat whitelist (always sent; derived from projects when catalog present). */
  repos: RepoConfig[];
  /** Project catalog with milestone index (preferred register payload). */
  projects?: ProjectCatalog[];
  /** Cursor.models.list() snapshot for App model manage UI. */
  models?: CursorModelEntry[];
  /** Re-fetch Cursor models when Gateway sends models.refresh. */
  onModelsRefresh?: () => Promise<CursorModelEntry[]> | CursorModelEntry[];
  /** Name a chat from the first user message (AI). */
  onChatAutotitle?: (msg: {
    chatId: string;
    text: string;
  }) => void | Promise<void>;
  handlers: TaskHandlers;
  /** Optional DAG scheduler hook for workflow.assign */
  onWorkflowAssign?: (run: WorkflowRun) => void | Promise<void>;
  /** Optional revise follow-up hook for workflow.revise */
  onWorkflowRevise?: (msg: WorkflowReviseMessage) => void | Promise<void>;
  /** Optional review hook: approve → write progress; reject → no progress */
  onWorkflowReview?: (msg: WorkflowReviewMessage) => void | Promise<void>;
  /** Optional project.sync collect hook (M08). */
  onProjectSync?: (msg: {
    requestId: string;
    repoId: string;
  }) => void | Promise<void>;
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
  private models: CursorModelEntry[];

  constructor(opts: GatewayClientOptions) {
    this.models = opts.models ? [...opts.models] : [];
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
      case "models.refresh": {
        log.info("models.refresh requested");
        try {
          const list = this.opts.onModelsRefresh
            ? await this.opts.onModelsRefresh()
            : this.models;
          this.models = list ?? [];
          this.sendModelsReport(this.models);
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          log.warn("models.refresh failed", { error: message });
          this.sendModelsReport(this.models);
        }
        break;
      }
      case "chat.autotitle": {
        const chatId = String((msg as { chatId?: string }).chatId ?? "").trim();
        const text = String((msg as { text?: string }).text ?? "").trim();
        if (!chatId || !text) {
          log.warn("chat.autotitle missing chatId/text");
          break;
        }
        if (!this.opts.onChatAutotitle) {
          log.warn("chat.autotitle handler not configured");
          break;
        }
        try {
          await this.opts.onChatAutotitle({ chatId, text });
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          log.warn("chat.autotitle failed", { chatId, error: message });
        }
        break;
      }
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
      case "workflow.assign": {
        const wf = (msg as { workflow?: WorkflowRun }).workflow;
        if (!wf?.id || !Array.isArray(wf.nodes)) {
          log.warn("workflow.assign missing workflow");
          break;
        }
        log.info("workflow assigned", {
          workflowId: wf.id,
          nodes: wf.nodes.length,
        });
        if (this.opts.onWorkflowAssign) {
          try {
            await this.opts.onWorkflowAssign(wf);
          } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            log.error("workflow handler failed", { workflowId: wf.id, error: message });
          }
        } else {
          log.warn("workflow.assign ignored (no scheduler)");
        }
        break;
      }
      case "workflow.revise": {
        const m = msg as {
          workflowId?: string;
          nodeId?: string;
          instruction?: string;
        };
        if (!m.workflowId || !m.nodeId || !m.instruction?.trim()) {
          log.warn("workflow.revise missing fields");
          break;
        }
        log.info("workflow revise", {
          workflowId: m.workflowId,
          nodeId: m.nodeId,
        });
        if (this.opts.onWorkflowRevise) {
          try {
            await this.opts.onWorkflowRevise({
              workflowId: m.workflowId,
              nodeId: m.nodeId,
              instruction: m.instruction,
            });
          } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            log.error("revise handler failed", {
              workflowId: m.workflowId,
              nodeId: m.nodeId,
              error: message,
            });
          }
        } else {
          log.warn("workflow.revise ignored (no scheduler)");
        }
        break;
      }
      case "workflow.review": {
        const m = msg as {
          workflowId?: string;
          nodeId?: string;
          decision?: string;
          comment?: string;
          autoApprove?: boolean;
        };
        if (!m.workflowId || !m.nodeId || !m.decision) {
          log.warn("workflow.review missing fields");
          break;
        }
        log.info("workflow review", {
          workflowId: m.workflowId,
          nodeId: m.nodeId,
          decision: m.decision,
          autoApprove: m.autoApprove === true,
        });
        if (this.opts.onWorkflowReview) {
          try {
            const payload: WorkflowReviewMessage = {
              workflowId: m.workflowId,
              nodeId: m.nodeId,
              decision: m.decision,
            };
            if (m.comment !== undefined) payload.comment = m.comment;
            if (m.autoApprove === true) payload.autoApprove = true;
            await this.opts.onWorkflowReview(payload);
          } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            log.error("review handler failed", {
              workflowId: m.workflowId,
              nodeId: m.nodeId,
              error: message,
            });
          }
        } else {
          log.warn("workflow.review ignored (no scheduler)");
        }
        break;
      }
      case "project.sync": {
        const m = msg as { requestId?: string; repoId?: string };
        const requestId = (m.requestId ?? "").trim();
        const repoId = (m.repoId ?? "").trim();
        if (!repoId) {
          log.warn("project.sync missing repoId");
          break;
        }
        log.info("project.sync requested", { requestId: requestId || null, repoId });
        if (this.opts.onProjectSync) {
          try {
            await this.opts.onProjectSync({
              requestId: requestId || `req_${Date.now()}`,
              repoId,
            });
          } catch (err) {
            const message = err instanceof Error ? err.message : String(err);
            log.error("project.sync handler failed", { repoId, error: message });
            this.sendProjectSyncResult({
              requestId: requestId || `req_${Date.now()}`,
              repoId,
              payload: {
                schemaVersion: 1,
                requestId: requestId || `req_${Date.now()}`,
                slaveId: this.opts.slaveId,
                repoId,
                error: message,
                summary: `collect failed: ${message}`,
                syncedAt: new Date().toISOString(),
                branch: null,
                head: null,
                dirty: null,
                phases: [],
                recentCommits: [],
                warnings: [],
                activeWorkflows: [],
              },
            });
          }
        } else {
          log.warn("project.sync ignored (no handler)");
        }
        break;
      }
      case "project.sync.ok":
        log.info("project.sync.ok", {
          requestId: (msg as { requestId?: string }).requestId ?? null,
        });
        break;
      case "error":
        log.error("gateway error", {
          error: (msg as { error?: string }).error ?? "unknown",
        });
        break;
      default:
        log.debug("gateway message", { type: msg.type });
    }
  }

  /** Public emit for DAG scheduler-owned tasks. */
  emitTaskEvent: EmitEvent = (taskId, kind, payload) => {
    this.emitEvent(taskId, kind, payload);
  };

  /** WS fallback when HTTP POST /v1/project-sync is unavailable. */
  sendProjectSyncResult(body: {
    requestId: string;
    repoId: string;
    payload: Record<string, unknown>;
  }): void {
    this.send({
      type: "project.sync.result",
      requestId: body.requestId,
      slaveId: this.opts.slaveId,
      repoId: body.repoId,
      payload: body.payload,
    });
  }

  private sendRegister(): void {
    const projects = this.opts.projects;
    const repos =
      projects && projects.length > 0
        ? projects.map((p) => ({ id: p.id, name: p.name, cwd: p.cwd }))
        : this.opts.repos.map((r) => ({
            id: r.id,
            name: r.name,
            cwd: r.cwd,
          }));
    const body: Record<string, unknown> = {
      type: "register",
      slaveId: this.opts.slaveId,
      repos,
    };
    if (projects && projects.length > 0) {
      body.projects = projects.map((p) => ({
        id: p.id,
        name: p.name,
        cwd: p.cwd,
        index: p.index,
        milestones: p.milestones,
      }));
    }
    if (this.opts.name) {
      body.name = this.opts.name;
    }
    if (this.models.length > 0) {
      body.models = this.models;
    }
    this.send(body);
  }

  private sendModelsReport(models: CursorModelEntry[]): void {
    this.send({
      type: "models.report",
      slaveId: this.opts.slaveId,
      models,
    });
  }

  /** Update cached catalog (e.g. after startup list). */
  setModels(models: CursorModelEntry[]): void {
    this.models = models ? [...models] : [];
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
