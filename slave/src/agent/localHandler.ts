// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Agent, CursorAgentError, type AgentOptions, type Run } from "@cursor/sdk";
import type { SlaveConfig } from "../config.js";
import { findRepo } from "../config.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";
import { log } from "../log.js";
import { mapSdkMessage } from "./mapStream.js";

export interface LocalAgentHandlerOptions {
  cfg: SlaveConfig;
  /** Cursor API key value (from env named by apiKeyEnv). Never log this. */
  apiKey: string;
  /** Fallback model id when task.model is absent. */
  defaultModel: string;
}

/**
 * Execute assigned tasks with @cursor/sdk **Local** runtime only.
 * Never sets `cloud` on AgentOptions.
 */
export class LocalAgentTaskHandler implements TaskHandlers {
  private readonly cancelled = new Set<string>();
  private readonly activeRuns = new Map<string, Run>();
  private chain: Promise<void> = Promise.resolve();

  constructor(private readonly opts: LocalAgentHandlerOptions) {}

  onAssign(task: AssignedTask, emit: EmitEvent): Promise<void> {
    this.cancelled.delete(task.id);
    this.chain = this.chain.then(() => this.runTask(task, emit));
    return this.chain;
  }

  async onCancel(taskId: string, _emit: EmitEvent): Promise<void> {
    this.cancelled.add(taskId);
    const run = this.activeRuns.get(taskId);
    if (!run) {
      log.info("cancel: no active run yet", { taskId });
      return;
    }
    if (run.supports("cancel")) {
      log.info("cancel: requesting run.cancel", { taskId, runId: run.id });
      try {
        await run.cancel();
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        log.warn("cancel failed", { taskId, error: message });
      }
    } else {
      log.warn("cancel: run does not support cancel", {
        taskId,
        reason: run.unsupportedReason("cancel") ?? "unknown",
      });
    }
  }

  private async runTask(task: AssignedTask, emit: EmitEvent): Promise<void> {
    const repoId = task.repoId ?? "";
    const repo = repoId ? findRepo(this.opts.cfg, repoId) : undefined;
    if (!repo) {
      log.warn("assign rejected: repo not in whitelist", {
        taskId: task.id,
        repoId: repoId || null,
      });
      emit(task.id, "error", { message: `repo not in whitelist: ${repoId}` });
      emit(task.id, "done", { status: "error" });
      return;
    }

    const prompt = (task.prompt ?? "").trim();
    if (!prompt) {
      emit(task.id, "error", { message: "empty prompt" });
      emit(task.id, "done", { status: "error" });
      return;
    }

    if (this.cancelled.has(task.id)) {
      emit(task.id, "done", { status: "cancelled" });
      return;
    }

    const modelId = (task.model ?? this.opts.defaultModel).trim() || this.opts.defaultModel;

    // Local-only options: explicit `local.cwd`, never `cloud`, never settingSources "all".
    const createOptions: AgentOptions = {
      apiKey: this.opts.apiKey,
      model: { id: modelId },
      local: {
        cwd: repo.cwd,
        settingSources: [],
      },
    };
    assertNoCloud(createOptions);

    emit(task.id, "status", { status: "running" });

    let agent: Awaited<ReturnType<typeof Agent.create>> | undefined;
    try {
      agent = await Agent.create(createOptions);
      log.info("local agent created", {
        taskId: task.id,
        agentId: agent.agentId,
        model: modelId,
        cwd: repo.cwd,
      });

      if (this.cancelled.has(task.id)) {
        emit(task.id, "done", { status: "cancelled" });
        return;
      }

      const run = await agent.send(prompt);
      this.activeRuns.set(task.id, run);
      log.info("local run started", {
        taskId: task.id,
        runId: run.id,
        agentId: run.agentId,
      });

      try {
        for await (const event of run.stream()) {
          if (this.cancelled.has(task.id)) {
            if (run.supports("cancel")) {
              await run.cancel().catch(() => undefined);
            }
            break;
          }
          mapSdkMessage(event, (kind, payload) => emit(task.id, kind, payload));
        }
      } catch (streamErr) {
        const message = streamErr instanceof Error ? streamErr.message : String(streamErr);
        log.warn("stream ended with error; still calling wait()", {
          taskId: task.id,
          error: message,
        });
      }

      // Required: wait for terminal result even if stream aborted.
      const result = await run.wait();
      log.info("local run finished", {
        taskId: task.id,
        runId: result.id,
        status: result.status,
      });

      if (result.status === "cancelled" || this.cancelled.has(task.id)) {
        emit(task.id, "done", { status: "cancelled" });
        return;
      }
      if (result.status === "error") {
        emit(task.id, "error", {
          message: result.error?.message ?? "run error",
          code: result.error?.code ?? "run_error",
          phase: "run",
        });
        emit(task.id, "done", { status: "error" });
        return;
      }
      emit(task.id, "done", { status: "finished" });
    } catch (err) {
      if (err instanceof CursorAgentError) {
        log.error("local agent startup failed", {
          taskId: task.id,
          error: err.message,
          code: err.code ?? null,
          retryable: err.isRetryable,
        });
        emit(task.id, "error", {
          message: err.message,
          code: err.code ?? "startup_failed",
          phase: "startup",
          retryable: err.isRetryable,
        });
        emit(task.id, "done", { status: "error" });
        return;
      }
      const message = err instanceof Error ? err.message : String(err);
      log.error("local agent unexpected failure", { taskId: task.id, error: message });
      emit(task.id, "error", { message, phase: "unknown" });
      emit(task.id, "done", { status: "error" });
    } finally {
      this.activeRuns.delete(task.id);
      if (agent) {
        try {
          await agent[Symbol.asyncDispose]();
        } catch {
          try {
            agent.close();
          } catch {
            /* ignore */
          }
        }
      }
    }
  }
}

/** Compile-time / runtime guard: AgentOptions must not enable cloud. */
export function assertNoCloud(options: AgentOptions): void {
  if ("cloud" in options && options.cloud !== undefined) {
    throw new Error("cloud runtime is forbidden for Local Slave");
  }
  if (!options.local || !options.local.cwd) {
    throw new Error("local.cwd is required");
  }
  const sources = options.local.settingSources;
  if (sources?.includes("all")) {
    throw new Error('settingSources "all" is not allowed as default');
  }
}
