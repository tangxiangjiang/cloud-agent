// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { SlaveConfig } from "../config.js";
import { log } from "../log.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";
import { resolveAssignedRepo } from "../safety/repo.js";

/**
 * Fake events for Gateway-only联调 (`--stub`). Production path: LocalAgentTaskHandler.
 */
export class StubTaskHandler implements TaskHandlers {
  private readonly cancelled = new Set<string>();
  private chain: Promise<void> = Promise.resolve();

  constructor(private readonly cfg: SlaveConfig) {}

  onAssign(task: AssignedTask, emit: EmitEvent): Promise<void> {
    this.cancelled.delete(task.id);
    this.chain = this.chain.then(() => this.run(task, emit));
    return this.chain;
  }

  onCancel(taskId: string, emit: EmitEvent): void {
    this.cancelled.add(taskId);
    emit(taskId, "status", { status: "cancelling" });
    emit(taskId, "done", { status: "cancelled" });
  }

  private async run(task: AssignedTask, emit: EmitEvent): Promise<void> {
    const resolved = resolveAssignedRepo(this.cfg, task);
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

    emit(task.id, "status", { status: "running" });
    await sleep(30);
    if (this.cancelled.has(task.id)) {
      emit(task.id, "done", { status: "cancelled" });
      return;
    }

    emit(task.id, "assistant.delta", {
      text: "stub slave (--stub): no SDK",
    });
    await sleep(30);
    if (this.cancelled.has(task.id)) {
      emit(task.id, "done", { status: "cancelled" });
      return;
    }

    emit(task.id, "done", { status: "finished" });
    log.info("stub task finished", { taskId: task.id, cwd: resolved.repo.cwd });
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
