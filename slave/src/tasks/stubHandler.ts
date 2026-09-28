// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { SlaveConfig } from "../config.js";
import { findRepo } from "../config.js";
import { log } from "../log.js";
import type { AssignedTask, EmitEvent, TaskHandlers } from "../gateway/types.js";

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
    emit(taskId, "done", { status: "cancelled" });
  }

  private async run(task: AssignedTask, emit: EmitEvent): Promise<void> {
    const repoId = task.repoId ?? "";
    const repo = repoId ? findRepo(this.cfg, repoId) : undefined;
    if (!repo) {
      log.warn("assign rejected: repo not in whitelist", {
        taskId: task.id,
        repoId: repoId || null,
      });
      emit(task.id, "error", { message: `repo not in whitelist: ${repoId}` });
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
      text: "stub slave (M04-P02): no SDK yet",
    });
    await sleep(30);
    if (this.cancelled.has(task.id)) {
      emit(task.id, "done", { status: "cancelled" });
      return;
    }

    emit(task.id, "done", { status: "finished" });
    log.info("stub task finished", { taskId: task.id, cwd: repo.cwd });
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}
