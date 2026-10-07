// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/** Default tick while awake. */
export const DEFAULT_SLEEP_WAKE_TICK_MS = 2_000;
/** Clock gap ≥ this means the host slept (timers freeze during sleep). */
export const DEFAULT_SLEEP_WAKE_JUMP_MS = 20_000;

/** If wall-clock jumped by ≥ jumpMs since last sample, the host likely slept. */
export function sleptGapMs(
  prev: number,
  now: number,
  jumpMs: number = DEFAULT_SLEEP_WAKE_JUMP_MS,
): number | null {
  const gap = now - prev;
  return gap >= jumpMs ? gap : null;
}

export type SleepWakeWatchOptions = {
  tickMs?: number;
  jumpMs?: number;
  now?: () => number;
  setIntervalFn?: (fn: () => void, ms: number) => ReturnType<typeof setInterval>;
  clearIntervalFn?: (id: ReturnType<typeof setInterval>) => void;
};

/**
 * Detect macOS/Windows sleep: setInterval does not fire while asleep, so the
 * first tick after resume sees a large wall-clock gap.
 */
export function startSleepWakeWatch(
  onWake: (sleptMs: number) => void,
  opts?: SleepWakeWatchOptions,
): () => void {
  const tickMs = opts?.tickMs ?? DEFAULT_SLEEP_WAKE_TICK_MS;
  const jumpMs = opts?.jumpMs ?? DEFAULT_SLEEP_WAKE_JUMP_MS;
  const now = opts?.now ?? Date.now;
  const setInt = opts?.setIntervalFn ?? setInterval;
  const clearInt = opts?.clearIntervalFn ?? clearInterval;
  let last = now();
  const id = setInt(() => {
    const t = now();
    const gap = sleptGapMs(last, t, jumpMs);
    last = t;
    if (gap != null) onWake(gap);
  }, tickMs);
  return () => clearInt(id);
}
