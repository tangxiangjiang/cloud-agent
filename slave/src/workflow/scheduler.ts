// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { SlaveConfig } from "../config.js";
import { findRepo } from "../config.js";
import type { OptimizeFor } from "../agent/modelSelection.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";
import { log } from "../log.js";
import { generateCommitMessageWithAi } from "./commitMessageAi.js";
import { captureBaseline, collectNodeDiff } from "./diff.js";
import { commitOnApprove } from "./gitCommit.js";
import { GatewayHttpApi } from "./http.js";
import { buildNodePrompt, buildRevisePrompt } from "./prompt.js";
import { writeProgressOnApprove } from "./progress.js";
import {
  defaultSlaveStatePath,
  loadSlaveRuntimeState,
  saveSlaveRuntimeState,
} from "./runtimeState.js";
import {
  schedulableNodes,
  type WorkflowReviewMessage,
  type WorkflowReviseMessage,
  type WorkflowRun,
} from "./types.js";

export type NodeRunOutcome = "finished" | "error" | "cancelled";

/**
 * Serial DAG scheduler: one ready node at a time.
 * Agent success → awaiting_review (never approved). Downstream stays blocked until approve (M05-P05).
 * Revise (M05-P04): follow-up / re-run → refresh diff → awaiting_review again (never approve / progress).
 */
export class SerialDagScheduler {
  private chain: Promise<void> = Promise.resolve();
  private readonly active = new Set<string>();
  /** workflowId\\0nodeId → git:<sha> captured at node start */
  private readonly baselines = new Map<string, string | null>();
  /** workflowId\\0nodeId → last Local Agent id (for Agent.resume on revise) */
  private readonly agentIds = new Map<string, string>();
  private readonly stateFile: string;

  constructor(
    private readonly opts: {
      cfg: SlaveConfig;
      http: GatewayHttpApi;
      /** Runs a Gateway task and emits task.event via WS. */
      handlers: TaskHandlers;
      emit: EmitEvent;
      /** When set, approve uses Local Agent to draft the commit message. */
      commitAi?: {
        apiKey: string;
        model: string;
        optimizeFor?: OptimizeFor;
      } | null;
      /** Persist baselines/agentIds across slave restarts. */
      stateFile?: string;
    },
  ) {
    this.stateFile = opts.stateFile?.trim() || defaultSlaveStatePath();
    const loaded = loadSlaveRuntimeState(this.stateFile);
    for (const [k, v] of Object.entries(loaded.baselines)) {
      this.baselines.set(k, v);
    }
    for (const [k, v] of Object.entries(loaded.agentIds)) {
      if (v) this.agentIds.set(k, v);
    }
    if (Object.keys(loaded.baselines).length || Object.keys(loaded.agentIds).length) {
      log.info("slave runtime state restored", {
        path: this.stateFile,
        baselines: Object.keys(loaded.baselines).length,
        agentIds: Object.keys(loaded.agentIds).length,
      });
    }
  }

  private persistRuntime(): void {
    try {
      const baselines: Record<string, string | null> = {};
      for (const [k, v] of this.baselines) baselines[k] = v;
      const agentIds: Record<string, string> = {};
      for (const [k, v] of this.agentIds) agentIds[k] = v;
      saveSlaveRuntimeState(this.stateFile, { baselines, agentIds });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.warn("slave runtime state save failed", { error: message });
    }
  }

  /** Enqueue a workflow.assign from Gateway. */
  enqueue(run: WorkflowRun): Promise<void> {
    if (this.active.has(run.id)) {
      log.info("workflow already scheduling", { workflowId: run.id });
      return this.chain;
    }
    this.active.add(run.id);
    this.chain = this.chain
      .then(() => this.runSerial(run.id))
      .finally(() => {
        this.active.delete(run.id);
      });
    return this.chain;
  }

  /** Enqueue a workflow.revise from Gateway (node already set to running). */
  enqueueRevise(msg: WorkflowReviseMessage): Promise<void> {
    const instruction = msg.instruction.trim();
    if (!msg.workflowId || !msg.nodeId || !instruction) {
      log.warn("workflow.revise missing fields");
      return this.chain;
    }
    this.chain = this.chain.then(() =>
      this.runRevise(msg.workflowId, msg.nodeId, instruction),
    );
    return this.chain;
  }

  /**
   * Enqueue a workflow.review from Gateway.
   * approve → write progressDoc + local git commit (AI message when configured);
   * reject → never write progress / never commit.
   * Downstream scheduling is driven by workflow.assign after approve.
   */
  enqueueReview(msg: WorkflowReviewMessage): Promise<void> {
    if (!msg.workflowId || !msg.nodeId || !msg.decision) {
      log.warn("workflow.review missing fields");
      return this.chain;
    }
    this.chain = this.chain.then(() => this.runReview(msg));
    return this.chain;
  }

  private async runSerial(workflowId: string): Promise<void> {
    log.info("DAG scheduler start", { workflowId });
    for (;;) {
      const run = await this.opts.http.getWorkflow(workflowId);
      if (run.status === "failed" || run.status === "cancelled" || run.status === "completed") {
        log.info("DAG scheduler stop; workflow terminal", {
          workflowId,
          status: run.status,
        });
        return;
      }

      const ready = schedulableNodes(run.nodes);
      if (ready.length === 0) {
        const awaiting = run.nodes.some((n) => n.status === "awaiting_review");
        log.info("DAG scheduler idle", {
          workflowId,
          awaitingReview: awaiting,
          reason: awaiting
            ? "blocked on review; next node not ready until approve"
            : "no ready nodes",
        });
        return;
      }

      // Serial: only first ready node.
      const node = ready[0]!;
      log.info("DAG schedule node", { workflowId, nodeId: node.id });

      const repo = findRepo(this.opts.cfg, run.repoId);
      if (!repo) {
        await this.opts.http.patchNode(workflowId, node.id, {
          status: "failed",
        });
        log.error("repo not in whitelist", { repoId: run.repoId });
        return;
      }

      let prompt: string;
      try {
        prompt = buildNodePrompt(node, repo.cwd);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        await this.opts.http.patchNode(workflowId, node.id, { status: "failed" });
        log.error("prompt build failed", { nodeId: node.id, error: message });
        return;
      }

      const baselineKey = `${workflowId}\0${node.id}`;
      const baseline = await captureBaseline(repo.cwd);
      this.baselines.set(baselineKey, baseline);
      this.persistRuntime();
      log.info("node baseline captured", {
        workflowId,
        nodeId: node.id,
        baseline: baseline ?? null,
      });

      await this.opts.http.patchNode(workflowId, node.id, { status: "running" });

      const task = await this.opts.http.createTask({
        slaveId: this.opts.cfg.slaveId,
        repoId: run.repoId,
        prompt,
        model: node.model ?? this.opts.cfg.defaultModel,
        workflowId,
        nodeId: node.id,
      });

      await this.opts.http.patchNode(workflowId, node.id, {
        status: "running",
        taskId: task.id,
      });

      const { outcome, agentId } = await this.executeTask(task);
      if (agentId) {
        this.agentIds.set(baselineKey, agentId);
        this.persistRuntime();
      }

      if (outcome === "finished") {
        await this.uploadDiffAndAwaitReview(
          workflowId,
          node.id,
          repo.cwd,
          baselineKey,
          baseline,
          task.id,
        );
        // Stop until human approve unlocks dependents (M05-P05) or revise (M05-P04).
        return;
      }

      await this.opts.http.patchNode(workflowId, node.id, {
        status: "failed",
        taskId: task.id,
      });
      const onFailure = node.onFailure ?? "stop";
      log.warn("node failed", { workflowId, nodeId: node.id, onFailure, outcome });
      if (onFailure !== "skip") {
        // Default stop (and retry→stop for P02).
        return;
      }
      await this.opts.http.patchNode(workflowId, node.id, { status: "skipped" });
    }
  }

  private async runReview(msg: WorkflowReviewMessage): Promise<void> {
    const decision = String(msg.decision).trim().toLowerCase();
    log.info("review received", {
      workflowId: msg.workflowId,
      nodeId: msg.nodeId,
      decision,
      autoApprove: msg.autoApprove === true,
    });

    if (decision === "reject") {
      log.info("review reject: progress unchanged", {
        workflowId: msg.workflowId,
        nodeId: msg.nodeId,
      });
      return;
    }
    if (decision !== "approve") {
      log.warn("review ignored: unknown decision", { decision });
      return;
    }

    const run = await this.opts.http.getWorkflow(msg.workflowId);
    const node = run.nodes.find((n) => n.id === msg.nodeId);
    if (!node) {
      log.error("review: node not found", {
        workflowId: msg.workflowId,
        nodeId: msg.nodeId,
      });
      return;
    }

    const repo = findRepo(this.opts.cfg, run.repoId);
    if (!repo) {
      log.error("review: repo not in whitelist", { repoId: run.repoId });
      return;
    }

    try {
      const result = writeProgressOnApprove({
        repoCwd: repo.cwd,
        progressDoc: run.progressDoc,
        node,
      });
      if (result.written) {
        log.info("progress.md updated on approve", {
          workflowId: msg.workflowId,
          nodeId: msg.nodeId,
          phaseKey: result.phaseKey,
          path: result.path,
        });
      } else {
        log.warn("progress.md not written", {
          workflowId: msg.workflowId,
          nodeId: msg.nodeId,
          phaseKey: result.phaseKey,
          reason: result.reason,
        });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.error("progress.md write failed", {
        workflowId: msg.workflowId,
        nodeId: msg.nodeId,
        error: message,
      });
    }

    const commitAi = this.opts.commitAi;
    try {
      const commitResult = await commitOnApprove({
        repoCwd: repo.cwd,
        node,
        comment: msg.comment ?? null,
        generateMessage: commitAi
          ? async (ctx) =>
              generateCommitMessageWithAi({
                apiKey: commitAi.apiKey,
                model: commitAi.model,
                optimizeFor: commitAi.optimizeFor ?? this.opts.cfg.optimizeFor,
                cwd: repo.cwd,
                status: ctx.status,
                diffStat: ctx.diffStat,
                node: ctx.node,
                phaseKey: ctx.phaseKey,
                fallback: ctx.fallback,
              })
          : null,
      });
      if (!commitResult.ok) {
        log.warn("local git commit on approve failed", {
          workflowId: msg.workflowId,
          nodeId: msg.nodeId,
          reason: commitResult.reason,
        });
      } else if (commitResult.skipped) {
        log.info("local git commit skipped", {
          workflowId: msg.workflowId,
          nodeId: msg.nodeId,
          reason: commitResult.reason,
        });
      } else {
        log.info("local git commit on approve", {
          workflowId: msg.workflowId,
          nodeId: msg.nodeId,
          sha: commitResult.sha,
          ai: commitResult.ai,
          message: commitResult.message.split("\n")[0] ?? "",
        });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.error("local git commit on approve unexpected error", {
        workflowId: msg.workflowId,
        nodeId: msg.nodeId,
        error: message,
      });
    }
    // Gateway already set approved + RecomputeReady and sends workflow.assign
    // for newly ready nodes; no further action here.
  }

  private async runRevise(
    workflowId: string,
    nodeId: string,
    instruction: string,
  ): Promise<void> {
    log.info("revise start", { workflowId, nodeId });
    const run = await this.opts.http.getWorkflow(workflowId);
    const node = run.nodes.find((n) => n.id === nodeId);
    if (!node) {
      log.error("revise: node not found", { workflowId, nodeId });
      return;
    }

    const repo = findRepo(this.opts.cfg, run.repoId);
    if (!repo) {
      await this.opts.http.patchNode(workflowId, nodeId, { status: "failed" });
      log.error("revise: repo not in whitelist", { repoId: run.repoId });
      return;
    }

    const baselineKey = `${workflowId}\0${nodeId}`;
    let baseline: string | null;
    if (this.baselines.has(baselineKey)) {
      baseline = this.baselines.get(baselineKey) ?? null;
    } else {
      try {
        const prev = await this.opts.http.getNodeDiff(workflowId, nodeId);
        baseline = prev.baseline ?? null;
        this.baselines.set(baselineKey, baseline);
        this.persistRuntime();
        log.info("revise: restored baseline from prior diff", {
          workflowId,
          nodeId,
          baseline,
        });
      } catch {
        baseline = await captureBaseline(repo.cwd);
        this.baselines.set(baselineKey, baseline);
        this.persistRuntime();
        log.warn("revise: no prior baseline; captured current HEAD", {
          workflowId,
          nodeId,
          baseline,
        });
      }
    }

    const prompt = buildRevisePrompt(node, instruction);
    const resumeAgentId = this.agentIds.get(baselineKey);

    const task = await this.opts.http.createTask({
      slaveId: this.opts.cfg.slaveId,
      repoId: run.repoId,
      prompt,
      model: node.model ?? this.opts.cfg.defaultModel,
      workflowId,
      nodeId,
    });
    if (resumeAgentId) {
      task.resumeAgentId = resumeAgentId;
    }

    await this.opts.http.patchNode(workflowId, nodeId, {
      status: "running",
      taskId: task.id,
    });

    const { outcome, agentId } = await this.executeTask(task);
    if (agentId) {
      this.agentIds.set(baselineKey, agentId);
      this.persistRuntime();
    }

    if (outcome === "finished") {
      await this.uploadDiffAndAwaitReview(
        workflowId,
        nodeId,
        repo.cwd,
        baselineKey,
        baseline,
        task.id,
      );
      return;
    }

    await this.opts.http.patchNode(workflowId, nodeId, {
      status: "failed",
      taskId: task.id,
    });
    log.warn("revise failed", { workflowId, nodeId, outcome });
  }

  private async uploadDiffAndAwaitReview(
    workflowId: string,
    nodeId: string,
    repoCwd: string,
    baselineKey: string,
    baseline: string | null,
    taskId: string,
  ): Promise<void> {
    try {
      const diff = await collectNodeDiff(
        repoCwd,
        workflowId,
        nodeId,
        this.baselines.get(baselineKey) ?? baseline,
      );
      await this.opts.http.putNodeDiff(diff);
      log.info("node diff uploaded", {
        workflowId,
        nodeId,
        files: diff.files.length,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.warn("diff collect/upload failed; continuing to awaiting_review", {
        workflowId,
        nodeId,
        error: message,
      });
    }
    // Hard rule: never approve here (revise / first run alike).
    await this.opts.http.patchNode(workflowId, nodeId, {
      status: "awaiting_review",
      taskId,
    });
    log.info("node awaiting_review", { workflowId, nodeId, taskId });
  }

  private async executeTask(
    task: AssignedTask,
  ): Promise<{ outcome: NodeRunOutcome; agentId?: string }> {
    let outcome: NodeRunOutcome = "error";
    let agentId: string | undefined;
    await this.opts.handlers.onAssign(task, (taskId, kind, payload) => {
      if (kind === "done") {
        const st = String(payload.status ?? "");
        if (st === "finished") outcome = "finished";
        else if (st === "cancelled") outcome = "cancelled";
        else outcome = "error";
        const aid = payload.agentId;
        if (typeof aid === "string" && aid.trim()) agentId = aid.trim();
      }
      this.opts.emit(taskId, kind, payload);
    });
    if (agentId) return { outcome, agentId };
    return { outcome };
  }
}
