// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * Wall-clock + idle hang detection for Local Agent main tasks.
 * Injectable clock for unit tests; `0` timeouts are disabled.
 */

export type HangTimeoutReason = "task_timeout" | "idle_timeout";

export type HangTimerHandle = { __brand: "HangTimerHandle" };

export type HangTimeoutClock = {
  now: () => number;
  setTimeout: (fn: () => void, ms: number) => HangTimerHandle;
  clearTimeout: (id: HangTimerHandle) => void;
};

export type HangTimeoutWatchOptions = {
  taskTimeoutMs: number;
  idleTimeoutMs: number;
  onTimeout: (reason: HangTimeoutReason) => void;
  clock?: Partial<HangTimeoutClock>;
};

function defaultClock(): HangTimeoutClock {
  return {
    now: () => Date.now(),
    setTimeout: (fn, ms) =>
      setTimeout(fn, ms) as unknown as HangTimerHandle,
    clearTimeout: (id) =>
      clearTimeout(id as unknown as ReturnType<typeof setTimeout>),
  };
}

/**
 * Starts optional TTL + idle timers. Call {@link HangTimeoutWatch.touch} on
 * stream/onDelta activity; {@link HangTimeoutWatch.stop} when the run ends.
 */
export class HangTimeoutWatch {
  private taskTimer: HangTimerHandle | null = null;
  private idleTimer: HangTimerHandle | null = null;
  private stopped = false;
  private readonly clock: HangTimeoutClock;
  private readonly taskTimeoutMs: number;
  private readonly idleTimeoutMs: number;
  private readonly onTimeout: (reason: HangTimeoutReason) => void;

  constructor(opts: HangTimeoutWatchOptions) {
    this.taskTimeoutMs = Math.max(0, opts.taskTimeoutMs);
    this.idleTimeoutMs = Math.max(0, opts.idleTimeoutMs);
    this.onTimeout = opts.onTimeout;
    const base = defaultClock();
    this.clock = {
      now: opts.clock?.now ?? base.now,
      setTimeout: opts.clock?.setTimeout ?? base.setTimeout,
      clearTimeout: opts.clock?.clearTimeout ?? base.clearTimeout,
    };
  }

  /** Begin TTL (if enabled) and arm idle. No-op when both timeouts are 0. */
  start(): void {
    if (this.stopped) return;
    if (this.taskTimeoutMs > 0) {
      this.taskTimer = this.clock.setTimeout(
        () => this.fire("task_timeout"),
        this.taskTimeoutMs,
      );
    }
    this.armIdle();
  }

  /** Reset idle timer after stream/onDelta activity. */
  touch(): void {
    if (this.stopped) return;
    this.armIdle();
  }

  /** Clear timers; further touch/start are no-ops. */
  stop(): void {
    this.stopped = true;
    this.clearTimers();
  }

  private armIdle(): void {
    if (this.idleTimer) {
      this.clock.clearTimeout(this.idleTimer);
      this.idleTimer = null;
    }
    if (this.idleTimeoutMs > 0 && !this.stopped) {
      this.idleTimer = this.clock.setTimeout(
        () => this.fire("idle_timeout"),
        this.idleTimeoutMs,
      );
    }
  }

  private fire(reason: HangTimeoutReason): void {
    if (this.stopped) return;
    this.stop();
    this.onTimeout(reason);
  }

  private clearTimers(): void {
    if (this.taskTimer) {
      this.clock.clearTimeout(this.taskTimer);
      this.taskTimer = null;
    }
    if (this.idleTimer) {
      this.clock.clearTimeout(this.idleTimer);
      this.idleTimer = null;
    }
  }
}

/** Human-readable cancel reason for logs / error events. */
export function hangTimeoutMessage(
  reason: HangTimeoutReason,
  taskTimeoutMs: number,
  idleTimeoutMs: number,
): string {
  if (reason === "task_timeout") {
    return `task wall-clock timeout (${taskTimeoutMs}ms)`;
  }
  return `task idle timeout (${idleTimeoutMs}ms; no stream events)`;
}
