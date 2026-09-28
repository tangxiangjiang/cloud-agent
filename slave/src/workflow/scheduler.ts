// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { SlaveConfig } from "../config.js";
import { findRepo } from "../config.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";
import { log } from "../log.js";
import { captureBaseline, collectNodeDiff } from "./diff.js";
import { GatewayHttpApi } from "./http.js";
import { buildNodePrompt } from "./prompt.js";
import { schedulableNodes, type WorkflowRun } from "./types.js";

export type NodeRunOutcome = "finished" | "error" | "cancelled";

/**
 * Serial DAG scheduler: one ready node at a time.
 * Agent success → awaiting_review (never approved). Downstream stays blocked until approve (M05-P05).
 */
export class SerialDagScheduler {
  private chain: Promise<void> = Promise.resolve();
  private readonly active = new Set<string>();
  /** workflowId\\0nodeId → git:<sha> captured at node start */
  private readonly baselines = new Map<string, string | null>();

  constructor(
    private readonly opts: {
      cfg: SlaveConfig;
      http: GatewayHttpApi;
      /** Runs a Gateway task and emits task.event via WS. */
      handlers: TaskHandlers;
      emit: EmitEvent;
    },
  ) {}

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

      const outcome = await this.executeTask(task);
      if (outcome === "finished") {
        try {
          const diff = await collectNodeDiff(
            repo.cwd,
            workflowId,
            node.id,
            this.baselines.get(baselineKey) ?? baseline,
          );
          await this.opts.http.putNodeDiff(diff);
          log.info("node diff uploaded", {
            workflowId,
            nodeId: node.id,
            files: diff.files.length,
          });
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          log.warn("diff collect/upload failed; continuing to awaiting_review", {
            workflowId,
            nodeId: node.id,
            error: message,
          });
        }
        // Hard rule: never approve here.
        await this.opts.http.patchNode(workflowId, node.id, {
          status: "awaiting_review",
          taskId: task.id,
        });
        log.info("node awaiting_review", { workflowId, nodeId: node.id, taskId: task.id });
        // Stop until human approve unlocks dependents (M05-P05).
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

  private async executeTask(task: AssignedTask): Promise<NodeRunOutcome> {
    let outcome: NodeRunOutcome = "error";
    await this.opts.handlers.onAssign(task, (taskId, kind, payload) => {
      if (kind === "done") {
        const st = String(payload.status ?? "");
        if (st === "finished") outcome = "finished";
        else if (st === "cancelled") outcome = "cancelled";
        else outcome = "error";
      }
      this.opts.emit(taskId, kind, payload);
    });
    return outcome;
  }
}
