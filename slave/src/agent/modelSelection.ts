// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { ModelSelection } from "@cursor/sdk";

/** Cursor Router optimize_for modes (SDK `auto-smart`). */
export type OptimizeFor = "cost" | "balanced" | "intelligence";

const AUTO_IDS = new Set(["auto", "auto-smart"]);

export function isAutoModelId(id: string): boolean {
  return AUTO_IDS.has(id.trim().toLowerCase());
}

export function parseOptimizeFor(raw: unknown, field = "optimizeFor"): OptimizeFor {
  if (raw === undefined || raw === null || raw === "") {
    return "balanced";
  }
  if (typeof raw !== "string") {
    throw new Error(`${field} must be a string`);
  }
  const v = raw.trim().toLowerCase();
  if (v === "cost" || v === "balanced" || v === "intelligence") {
    return v;
  }
  throw new Error(`${field} must be cost|balanced|intelligence`);
}

/**
 * Resolve task.model for Local Agent.
 * - empty → `defaultModel`
 * - `auto` / `auto-smart` → Cursor Router (`auto-smart` + `optimize_for`)
 * - other ids → pass through
 */
export function resolveModelSelection(
  taskModel: string | null | undefined,
  defaultModel: string,
  optimizeFor: OptimizeFor = "balanced",
): ModelSelection {
  const raw = (taskModel ?? "").trim();
  const id = raw || defaultModel.trim() || "auto-smart";
  if (isAutoModelId(id)) {
    return {
      id: "auto-smart",
      params: [{ id: "optimize_for", value: optimizeFor }],
    };
  }
  return { id };
}
