// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * Chat mode system prefixes for Local Agent (M09-P03).
 * Gateway also injects prefixes; Slave re-applies without doubling.
 */

const ASK_MARKER = "[Mode: Ask";
const PLAN_MARKER = "[Mode: Plan";

const ASK_PREFIX =
  "[Mode: Ask — READ-ONLY. Do not write, edit, delete, or create files; do not run mutating git commands; answer from inspection only.]\n\n";

const PLAN_PREFIX =
  "[Mode: Plan — Produce a structured plan with clear steps. Prefer not to modify the workspace unless the user explicitly asks to execute the plan.]\n\n";

/** Normalize chat mode from task.mode. */
export function normalizeChatMode(mode: string | null | undefined): "agent" | "ask" | "plan" {
  switch (String(mode ?? "").trim().toLowerCase()) {
    case "ask":
      return "ask";
    case "plan":
      return "plan";
    default:
      return "agent";
  }
}

/**
 * Ensure Ask/Plan guidance is present on the prompt sent to Local Agent.
 * Does not double-prefix if Gateway already injected the same mode marker.
 */
export function applyChatModePrefix(
  prompt: string,
  mode: string | null | undefined,
): string {
  const text = prompt.trim();
  const m = normalizeChatMode(mode);
  if (m === "ask") {
    if (text.includes(ASK_MARKER)) return text;
    return ASK_PREFIX + text;
  }
  if (m === "plan") {
    if (text.includes(PLAN_MARKER)) return text;
    return PLAN_PREFIX + text;
  }
  return text;
}
