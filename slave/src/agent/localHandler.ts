// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Agent, CursorAgentError, type AgentOptions, type Run } from "@cursor/sdk";
import type { SlaveConfig } from "../config.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";
import { log } from "../log.js";
import { attemptRunCancel } from "../safety/cancel.js";
import { resolveAssignedRepo } from "../safety/repo.js";
import { applyChatModePrefix, normalizeChatMode } from "./chatMode.js";
import {
  HangTimeoutWatch,
  hangTimeoutMessage,
  type HangTimeoutReason,
} from "./hangTimeout.js";
import { mapInteractionDelta, mapSdkMessage, type InteractionPhaseState } from "./mapStream.js";
import {
  isAutoModelId,
  resolveModelSelection,
  type OptimizeFor,
} from "./modelSelection.js";

export interface LocalAgentHandlerOptions {
  cfg: SlaveConfig;
  /** Cursor API key value (from env named by apiKeyEnv). Never log this. */
  apiKey: string;
  /** From Cursor.models.list(); empty until setAvailableModels. */
  availableModelIds?: ReadonlySet<string>;
}

/**
 * Execute assigned tasks with @cursor/sdk **Local** runtime only.
 * Never sets `cloud` on AgentOptions; cwd only from whitelist.
 */
export class LocalAgentTaskHandler implements TaskHandlers {
  private readonly cancelled = new Set<string>();
  private readonly activeRuns = new Map<string, Run>();
  private chain: Promise<void> = Promise.resolve();
  private availableModelIds: ReadonlySet<string>;

  constructor(private readonly opts: LocalAgentHandlerOptions) {
    this.availableModelIds = opts.availableModelIds ?? new Set();
  }

  /** Update catalog after startup Cursor.models.list(). */
  setAvailableModels(ids: ReadonlySet<string>): void {
    this.availableModelIds = ids;
  }

  private resolveModel(taskModel: string | null | undefined) {
    return resolveModelSelection(taskModel, this.opts.cfg.defaultModel, {
      optimizeFor: this.opts.cfg.optimizeFor,
      autoModelId: this.opts.cfg.autoModelId,
      availableModelIds: this.availableModelIds,
    });
  }

  onAssign(task: AssignedTask, emit: EmitEvent): Promise<void> {
    this.cancelled.delete(task.id);
    this.chain = this.chain.then(() => this.runTask(task, emit));
    return this.chain;
  }

  async onCancel(taskId: string, emit: EmitEvent): Promise<void> {
    this.cancelled.add(taskId);
    emit(taskId, "status", { status: "cancelling" });

    const run = this.activeRuns.get(taskId);
    if (!run) {
      log.info("cancel: no active run yet", { taskId });
      return;
    }

    try {
      const attempt = await attemptRunCancel(run);
      if (attempt.supported) {
        log.info("cancel: run.cancel requested", { taskId, runId: run.id });
      } else {
        log.warn("cancel: unsupported; stopping stream relay (best effort)", {
          taskId,
          runId: run.id,
          reason: attempt.reason,
        });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.warn("cancel failed", { taskId, error: message });
    }
  }

  private async runTask(task: AssignedTask, emit: EmitEvent): Promise<void> {
    const resolved = resolveAssignedRepo(this.opts.cfg, task);
    if (!resolved.ok) {
      log.warn("assign rejected", {
        taskId: task.id,
        code: resolved.code,
        message: resolved.message,
      });
      emit(task.id, "error", {
        message: resolved.message,
        code: resolved.code,
        phase: "policy",
      });
      emit(task.id, "done", { status: "error" });
      return;
    }
    const { repo } = resolved;

    const rawPrompt = (task.prompt ?? "").trim();
    if (!rawPrompt) {
      emit(task.id, "error", {
        message: "empty prompt",
        code: "empty_prompt",
        phase: "policy",
      });
      emit(task.id, "done", { status: "error" });
      return;
    }
    const prompt = applyChatModePrefix(rawPrompt, task.mode);

    if (this.cancelled.has(task.id)) {
      emit(task.id, "done", { status: "cancelled" });
      return;
    }

    const requested = String(task.model ?? "").trim();
    const model = this.resolveModel(task.model);
    const modelId = model.id;

    // Local-only: whitelist cwd only; never cloud; never settingSources "all".
    const createOptions: AgentOptions = {
      apiKey: this.opts.apiKey,
      model,
      local: {
        cwd: repo.cwd,
        settingSources: [],
      },
    };
    assertNoCloud(createOptions);

    const optimizeFor = model.params?.find((p) => p.id === "optimize_for")?.value;
    emit(task.id, "status", {
      status: "running",
      mode: normalizeChatMode(task.mode),
      ...(isAutoModelId(requested) || (!requested && isAutoModelId(this.opts.cfg.defaultModel))
        ? {
            model: requested || "auto",
            resolvedModel: modelId,
            ...(optimizeFor ? { optimizeFor } : {}),
          }
        : { model: modelId }),
    });

    let agent: Awaited<ReturnType<typeof Agent.create>> | undefined;
    let hangReason: HangTimeoutReason | null = null;
    const taskTimeoutMs = this.opts.cfg.taskTimeoutMs;
    const idleTimeoutMs = this.opts.cfg.idleTimeoutMs;
    const hang = new HangTimeoutWatch({
      taskTimeoutMs,
      idleTimeoutMs,
      onTimeout: (reason) => {
        if (this.cancelled.has(task.id)) return;
        this.cancelled.add(task.id);
        hangReason = reason;
        const message = hangTimeoutMessage(reason, taskTimeoutMs, idleTimeoutMs);
        log.warn("local agent hang timeout", {
          taskId: task.id,
          reason,
          taskTimeoutMs,
          idleTimeoutMs,
        });
        emit(task.id, "status", { status: "cancelling" });
        emit(task.id, "error", {
          message,
          code: reason,
          phase: "run",
        });
        const run = this.activeRuns.get(task.id);
        if (!run) {
          log.info("hang timeout: no active run yet", { taskId: task.id, reason });
          return;
        }
        void attemptRunCancel(run)
          .then((attempt) => {
            if (attempt.supported) {
              log.info("hang timeout: run.cancel requested", {
                taskId: task.id,
                runId: run.id,
                reason,
              });
            } else {
              log.warn("hang timeout: cancel unsupported; stopping stream relay", {
                taskId: task.id,
                runId: run.id,
                reason: attempt.reason,
              });
            }
          })
          .catch((err) => {
            const msg = err instanceof Error ? err.message : String(err);
            log.warn("hang timeout: cancel failed", { taskId: task.id, error: msg });
          });
      },
    });
    hang.start();
    try {
      const resumeId = (task.resumeAgentId ?? "").trim();
      if (resumeId) {
        try {
          agent = await Agent.resume(resumeId, {
            apiKey: this.opts.apiKey,
            model,
            local: {
              cwd: repo.cwd,
              settingSources: [],
            },
          });
          hang.touch();
          log.info("local agent resumed (revise follow-up)", {
            taskId: task.id,
            agentId: agent.agentId,
            model: modelId,
            optimizeFor: model.params ? optimizeFor : undefined,
            cwd: repo.cwd,
          });
        } catch (resumeErr) {
          const message =
            resumeErr instanceof Error ? resumeErr.message : String(resumeErr);
          log.warn("Agent.resume failed; falling back to Agent.create", {
            taskId: task.id,
            resumeAgentId: resumeId,
            error: message,
          });
        }
      }
      if (!agent) {
        agent = await Agent.create(createOptions);
        hang.touch();
        log.info("local agent created", {
          taskId: task.id,
          agentId: agent.agentId,
          model: modelId,
          optimizeFor: model.params ? optimizeFor : undefined,
          cwd: repo.cwd,
        });
      }

      if (this.cancelled.has(task.id)) {
        emit(task.id, "done", {
          status: "cancelled",
          ...(hangReason ? { reason: hangReason } : {}),
        });
        return;
      }

      const phaseState: InteractionPhaseState = {};
      const run = await agent.send(prompt, {
        onDelta: ({ update }) => {
          hang.touch();
          if (this.cancelled.has(task.id)) return;
          mapInteractionDelta(
            update as {
              type?: string;
              text?: string;
              callId?: string;
              toolCall?: { type?: string; args?: unknown; result?: { status?: string } };
            },
            (kind, payload) => emit(task.id, kind, payload),
            phaseState,
          );
        },
      });
      this.activeRuns.set(task.id, run);
      hang.touch();
      log.info("local run started", {
        taskId: task.id,
        runId: run.id,
        agentId: run.agentId,
      });

      if (this.cancelled.has(task.id)) {
        try {
          await attemptRunCancel(run);
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          log.warn("cancel after hang before stream failed", {
            taskId: task.id,
            error: message,
          });
        }
      }

      let cancelAttempted = false;
      try {
        for await (const event of run.stream()) {
          hang.touch();
          if (this.cancelled.has(task.id)) {
            if (!cancelAttempted) {
              cancelAttempted = true;
              try {
                const attempt = await attemptRunCancel(run);
                if (!attempt.supported) {
                  log.warn("cancel: unsupported during stream; dropping further events", {
                    taskId: task.id,
                    reason: attempt.reason,
                  });
                }
              } catch (err) {
                const message = err instanceof Error ? err.message : String(err);
                log.warn("cancel during stream failed", { taskId: task.id, error: message });
              }
            }
            // DoD: stop relaying new tool/assistant events ASAP after cancel.
            continue;
          }
          // Text already streamed via onDelta; stream() still carries tools/status/thinking.
          mapSdkMessage(event, (kind, payload) => emit(task.id, kind, payload), {
            skipAssistantText: true,
            phaseState,
          });
        }
      } catch (streamErr) {
        const message = streamErr instanceof Error ? streamErr.message : String(streamErr);
        log.warn("stream ended with error; still calling wait()", {
          taskId: task.id,
          error: message,
        });
      }

      const result = await run.wait();
      log.info("local run finished", {
        taskId: task.id,
        runId: result.id,
        status: result.status,
      });

      const agentId = agent.agentId ?? run.agentId;
      if (result.status === "cancelled" || this.cancelled.has(task.id)) {
        emit(task.id, "done", {
          status: "cancelled",
          agentId,
          ...(hangReason ? { reason: hangReason } : {}),
        });
        return;
      }
      if (result.status === "error") {
        emit(task.id, "error", {
          message: result.error?.message ?? "run error",
          code: result.error?.code ?? "run_error",
          phase: "run",
        });
        emit(task.id, "done", { status: "error", agentId });
        return;
      }
      emit(task.id, "done", { status: "finished", agentId });
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
      hang.stop();
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

/** @deprecated use resolveModelSelection — kept for call-site string checks. */
export function resolveTaskModel(
  taskModel: string | null | undefined,
  defaultModel: string,
  optimizeFor: OptimizeFor = "cost",
): string {
  return resolveModelSelection(taskModel, defaultModel, {
    optimizeFor,
    autoModelId: "default",
  }).id;
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
