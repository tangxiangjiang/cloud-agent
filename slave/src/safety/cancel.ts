// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * Cancel / stream relay helpers (unit-testable without real SDK).
 */

export interface CancelableRun {
  id: string;
  supports(op: "cancel"): boolean;
  unsupportedReason?(op: "cancel"): string | undefined;
  cancel(): Promise<void>;
}

export type CancelAttempt =
  | { supported: true }
  | { supported: false; reason: string };

/**
 * Try SDK cancel once. When unsupported, caller must stop relaying new tool events.
 */
export async function attemptRunCancel(run: CancelableRun): Promise<CancelAttempt> {
  if (!run.supports("cancel")) {
    return {
      supported: false,
      reason: run.unsupportedReason?.("cancel") ?? "cancel unsupported",
    };
  }
  await run.cancel();
  return { supported: true };
}

/** After cancel is requested, drop further stream events (esp. tool.*) before wait(). */
export function shouldRelayStreamEvent(cancelRequested: boolean): boolean {
  return !cancelRequested;
}
