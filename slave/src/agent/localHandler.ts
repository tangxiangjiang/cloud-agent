// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Agent, CursorAgentError, type AgentOptions, type Run } from "@cursor/sdk";
import type { SlaveConfig } from "../config.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";
import { log } from "../log.js";
import { attemptRunCancel } from "../safety/cancel.js";
import { resolveAssignedRepo } from "../safety/repo.js";
import {
  aiRetryDelayMs,
  isRetryableAiError,
  sleep,
} from "./aiRetry.js";
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

/**
 * Main chat/workflow Local Agent loads project rules (`.cursor/rules`, AGENTS.md).
 * Never `"all"` — that would pull user/team/plugins from the host environment.
 */
export const MAIN_TASK_SETTING_SOURCES = ["project"] as const;

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

    // Local-only: whitelist cwd; load project rules; never settingSources "all".
    const createOptions: AgentOptions = {
      apiKey: this.opts.apiKey,
      model,
      local: {
        cwd: repo.cwd,
        settingSources: [...MAIN_TASK_SETTING_SOURCES],
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
    const maxAttempts = this.opts.cfg.aiRetryAttempts;
    const baseDelayMs = this.opts.cfg.aiRetryBaseDelayMs;
    const isWorkflow = Boolean((task.workflowId ?? "").trim());
    const preferResumeId = (task.resumeAgentId ?? "").trim();
    let lastAgentId: string | undefined;

    /** Cleared when a resumed agent still has a zombie active run. */
    let forceFreshAgent = false;
    try {
      for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        if (this.cancelled.has(task.id)) {
          this.emitCancelledOrPaused(task, emit, lastAgentId, hangReason, isWorkflow);
          return;
        }

        try {
          const resumeId = forceFreshAgent
            ? ""
            : preferResumeId || lastAgentId || "";
          agent = await this.openAgent({
            task,
            createOptions,
            model,
            modelId,
            optimizeFor: typeof optimizeFor === "string" ? optimizeFor : undefined,
            preferResumeId: resumeId,
            cwd: repo.cwd,
            hang,
          });
          hang.touch();
          lastAgentId = agent.agentId;

          if (this.cancelled.has(task.id)) {
            this.emitCancelledOrPaused(task, emit, lastAgentId, hangReason, isWorkflow);
            return;
          }

          const sendPrompt =
            attempt > 1 && lastAgentId
              ? [
                  prompt,
                  "",
                  "## Reconnect after network interruption",
                  "A previous attempt was interrupted by a transient network/AI error.",
                  "Continue the **same** task from the current working tree; do not redo finished work.",
                ].join("\n")
              : prompt;

          const phaseState: InteractionPhaseState = {};
          let run: Run;
          try {
            run = await agent.send(sendPrompt, {
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
          } catch (sendErr) {
            const sendMsg = sendErr instanceof Error ? sendErr.message : String(sendErr);
            // Resume can succeed while Cursor still holds a zombie run on that agent.
            if (!forceFreshAgent && /already has active run/i.test(sendMsg)) {
              log.warn("resumed agent has active run; retrying with Agent.create", {
                taskId: task.id,
                agentId: lastAgentId ?? null,
                error: sendMsg,
              });
              forceFreshAgent = true;
              await this.disposeAgent(agent);
              agent = undefined;
              lastAgentId = undefined;
              this.activeRuns.delete(task.id);
              continue;
            }
            throw sendErr;
          }
          forceFreshAgent = false;
          this.activeRuns.set(task.id, run);
          hang.touch();
          lastAgentId = agent.agentId ?? run.agentId ?? lastAgentId;
          log.info("local run started", {
            taskId: task.id,
            runId: run.id,
            agentId: run.agentId,
            attempt,
            maxAttempts,
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
          let streamRetryableErr: unknown = null;
          try {
            for await (const event of run.stream()) {
              hang.touch();
              if (this.cancelled.has(task.id)) {
                if (!cancelAttempted) {
                  cancelAttempted = true;
                  try {
                    const attemptCancel = await attemptRunCancel(run);
                    if (!attemptCancel.supported) {
                      log.warn("cancel: unsupported during stream; dropping further events", {
                        taskId: task.id,
                        reason: attemptCancel.reason,
                      });
                    }
                  } catch (err) {
                    const message = err instanceof Error ? err.message : String(err);
                    log.warn("cancel during stream failed", { taskId: task.id, error: message });
                  }
                }
                continue;
              }
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
              attempt,
            });
            if (isRetryableAiError(streamErr)) {
              streamRetryableErr = streamErr;
            }
          }

          const result = await run.wait();
          log.info("local run finished", {
            taskId: task.id,
            runId: result.id,
            status: result.status,
            attempt,
          });

          const agentId = agent.agentId ?? run.agentId ?? lastAgentId;
          lastAgentId = agentId;

          if (result.status === "cancelled" || this.cancelled.has(task.id)) {
            this.emitCancelledOrPaused(task, emit, agentId, hangReason, isWorkflow);
            return;
          }

          if (result.status === "error") {
            const runErr = {
              message: result.error?.message ?? "run error",
              code: result.error?.code ?? "run_error",
              isRetryable: isRetryableAiError(result.error ?? result) || Boolean(streamRetryableErr),
            };
            if (runErr.isRetryable && attempt < maxAttempts && !this.cancelled.has(task.id)) {
              await this.backoffAndRetry(task, emit, attempt, maxAttempts, baseDelayMs, runErr.message);
              await this.disposeAgent(agent);
              agent = undefined;
              this.activeRuns.delete(task.id);
              continue;
            }
            if (runErr.isRetryable && isWorkflow) {
              this.emitNetworkPause(task, emit, agentId, runErr.message, runErr.code);
              return;
            }
            emit(task.id, "error", {
              message: runErr.message,
              code: runErr.code,
              phase: "run",
            });
            emit(task.id, "done", { status: "error", agentId });
            return;
          }

          emit(task.id, "done", { status: "finished", agentId });
          return;
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          const retryable =
            (err instanceof CursorAgentError && err.isRetryable) || isRetryableAiError(err);

          if (retryable && attempt < maxAttempts && !this.cancelled.has(task.id)) {
            log.warn("local agent attempt failed; will retry", {
              taskId: task.id,
              attempt,
              maxAttempts,
              error: message,
              code: err instanceof CursorAgentError ? (err.code ?? null) : null,
            });
            await this.backoffAndRetry(task, emit, attempt, maxAttempts, baseDelayMs, message);
            await this.disposeAgent(agent);
            agent = undefined;
            this.activeRuns.delete(task.id);
            continue;
          }

          if (retryable && isWorkflow) {
            log.warn("local agent network exhausted; pausing for human", {
              taskId: task.id,
              attempt,
              maxAttempts,
              error: message,
            });
            this.emitNetworkPause(
              task,
              emit,
              lastAgentId,
              message,
              err instanceof CursorAgentError ? (err.code ?? "ai_network") : "ai_network",
            );
            return;
          }

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
            emit(task.id, "done", { status: "error", ...(lastAgentId ? { agentId: lastAgentId } : {}) });
            return;
          }
          log.error("local agent unexpected failure", { taskId: task.id, error: message });
          emit(task.id, "error", { message, phase: "unknown" });
          emit(task.id, "done", { status: "error", ...(lastAgentId ? { agentId: lastAgentId } : {}) });
          return;
        }
      }
    } finally {
      hang.stop();
      this.activeRuns.delete(task.id);
      await this.disposeAgent(agent);
    }
  }

  private async openAgent(args: {
    task: AssignedTask;
    createOptions: AgentOptions;
    model: ReturnType<LocalAgentTaskHandler["resolveModel"]>;
    modelId: string;
    optimizeFor: string | undefined;
    preferResumeId: string;
    cwd: string;
    hang: HangTimeoutWatch;
  }): Promise<Awaited<ReturnType<typeof Agent.create>>> {
    const { task, createOptions, model, modelId, optimizeFor, preferResumeId, cwd, hang } = args;
    if (preferResumeId) {
      try {
        const resumed = await Agent.resume(preferResumeId, {
          apiKey: this.opts.apiKey,
          model,
          local: {
            cwd,
            settingSources: [...MAIN_TASK_SETTING_SOURCES],
          },
        });
        hang.touch();
        log.info("local agent resumed", {
          taskId: task.id,
          agentId: resumed.agentId,
          model: modelId,
          optimizeFor: model.params ? optimizeFor : undefined,
          cwd,
        });
        return resumed;
      } catch (resumeErr) {
        const message = resumeErr instanceof Error ? resumeErr.message : String(resumeErr);
        // Auth / non-retryable resume failure: fall through to create.
        // Retryable network on resume: rethrow so outer loop can backoff.
        if (isRetryableAiError(resumeErr)) {
          throw resumeErr;
        }
        log.warn("Agent.resume failed; falling back to Agent.create", {
          taskId: task.id,
          resumeAgentId: preferResumeId,
          error: message,
        });
      }
    }
    const created = await Agent.create(createOptions);
    hang.touch();
    log.info("local agent created", {
      taskId: task.id,
      agentId: created.agentId,
      model: modelId,
      optimizeFor: model.params ? optimizeFor : undefined,
      cwd,
    });
    return created;
  }

  private async backoffAndRetry(
    task: AssignedTask,
    emit: EmitEvent,
    attempt: number,
    maxAttempts: number,
    baseDelayMs: number,
    message: string,
  ): Promise<void> {
    const delayMs = aiRetryDelayMs(attempt + 1, baseDelayMs);
    emit(task.id, "status", {
      status: "retrying",
      attempt,
      nextAttempt: attempt + 1,
      maxAttempts,
      delayMs,
      message,
    });
    log.info("local agent reconnect backoff", {
      taskId: task.id,
      attempt,
      nextAttempt: attempt + 1,
      maxAttempts,
      delayMs,
      error: message,
    });
    await sleep(delayMs);
  }

  private emitNetworkPause(
    task: AssignedTask,
    emit: EmitEvent,
    agentId: string | undefined,
    message: string,
    code: string,
  ): void {
    emit(task.id, "error", {
      message: `AI network issue after retries; paused for human: ${message}`,
      code,
      phase: "run",
      retryable: true,
      paused: true,
    });
    emit(task.id, "done", {
      status: "paused",
      reason: "ai_network",
      ...(agentId ? { agentId } : {}),
    });
  }

  private emitCancelledOrPaused(
    task: AssignedTask,
    emit: EmitEvent,
    agentId: string | undefined,
    hangReason: HangTimeoutReason | null,
    isWorkflow: boolean,
  ): void {
    // Idle silence often means network stall — pause workflow for human instead of fail/cancel.
    if (isWorkflow && hangReason === "idle_timeout") {
      emit(task.id, "done", {
        status: "paused",
        reason: "idle_timeout",
        ...(agentId ? { agentId } : {}),
      });
      return;
    }
    emit(task.id, "done", {
      status: "cancelled",
      ...(agentId ? { agentId } : {}),
      ...(hangReason ? { reason: hangReason } : {}),
    });
  }

  private async disposeAgent(
    agent: Awaited<ReturnType<typeof Agent.create>> | undefined,
  ): Promise<void> {
    if (!agent) return;
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
