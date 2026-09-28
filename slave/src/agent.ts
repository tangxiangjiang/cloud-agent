// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * Placeholder for Local Agent execution (wired in M04-P03).
 * Intentionally has no @cursor/sdk imports and no cloud options.
 */

export interface LocalRunRequest {
  taskId: string;
  repoId: string;
  /** Absolute cwd from slave whitelist. */
  cwd: string;
  prompt: string;
  model?: string;
}

export interface TaskRunner {
  run(req: LocalRunRequest): Promise<void>;
  cancel?(taskId: string): Promise<void>;
}

/** Stub until M04-P03 implements Agent.create / send / stream / wait. */
export class StubTaskRunner implements TaskRunner {
  async run(_req: LocalRunRequest): Promise<void> {
    throw new Error("TaskRunner not implemented (see M04-P03)");
  }
}
