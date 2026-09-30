// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { ModelSelection } from "@cursor/sdk";

/** Cursor Router optimize_for modes (SDK `auto-smart`, Teams/Enterprise). */
export type OptimizeFor = "cost" | "balanced" | "intelligence";

/** App/Gateway "auto" aliases that need account-aware resolution. */
const AUTO_ALIASES = new Set(["auto", "auto-smart", "default"]);

export function isAutoModelId(id: string): boolean {
  return AUTO_ALIASES.has(id.trim().toLowerCase());
}

export function parseOptimizeFor(raw: unknown, field = "optimizeFor"): OptimizeFor {
  // Default cost: bundled Auto pricing when Router is available.
  if (raw === undefined || raw === null || raw === "") {
    return "cost";
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

export interface ResolveModelOptions {
  optimizeFor?: OptimizeFor;
  /**
   * Preferred Auto target when App sends `auto`.
   * Pro often only has `default`; Teams/Enterprise may have `auto-smart`.
   */
  autoModelId?: string;
  /** Ids from `Cursor.models.list()` for this API key (optional). */
  availableModelIds?: ReadonlySet<string> | readonly string[];
}

function toSet(
  ids: ReadonlySet<string> | readonly string[] | undefined,
): ReadonlySet<string> | undefined {
  if (!ids) return undefined;
  if (ids instanceof Set) return ids;
  const out = new Set<string>();
  for (const x of ids) {
    const id = x.trim();
    if (id) out.add(id);
  }
  return out;
}

/**
 * Pick SDK model id for App Auto / config auto aliases.
 * Prefer catalog: auto-smart → auto → default. Respect `preferred` when present in catalog.
 */
export function pickAutoTarget(
  preferred: string | undefined,
  availableModelIds?: ReadonlySet<string> | readonly string[],
): { id: string; useOptimizeFor: boolean } {
  const available = toSet(availableModelIds);
  const has = (id: string): boolean => {
    if (!available || available.size === 0) return true;
    return available.has(id);
  };
  const pref = (preferred ?? "").trim().toLowerCase();

  if (pref && has(pref)) {
    return { id: pref, useOptimizeFor: pref === "auto-smart" };
  }

  for (const id of ["auto-smart", "auto", "default"] as const) {
    if (available?.has(id)) {
      return { id, useOptimizeFor: id === "auto-smart" };
    }
  }

  // No catalog / preferred missing: Pro-safe fallback (not auto-smart).
  if (pref === "auto-smart" || pref === "auto" || pref === "default" || !pref) {
    return { id: pref && pref !== "auto" ? pref : "default", useOptimizeFor: pref === "auto-smart" };
  }
  return { id: pref, useOptimizeFor: false };
}

/**
 * Resolve task.model for Local Agent.
 * - empty → `defaultModel`
 * - `auto` / `auto-smart` / `default` → account-aware Auto (`default` on Pro; Router on Teams)
 * - other ids → pass through
 */
export function resolveModelSelection(
  taskModel: string | null | undefined,
  defaultModel: string,
  options: ResolveModelOptions | OptimizeFor = "cost",
): ModelSelection {
  const opts: ResolveModelOptions =
    typeof options === "string" ? { optimizeFor: options } : options;
  const optimizeFor = opts.optimizeFor ?? "cost";
  const raw = (taskModel ?? "").trim();
  const id = raw || defaultModel.trim() || "default";

  if (isAutoModelId(id)) {
    // App "auto" → preferred autoModelId; explicit auto-smart/default keep their id as preference.
    const preferred =
      id.toLowerCase() === "auto"
        ? opts.autoModelId?.trim() || "default"
        : id;
    const target = pickAutoTarget(preferred, opts.availableModelIds);
    if (target.useOptimizeFor) {
      return {
        id: "auto-smart",
        params: [{ id: "optimize_for", value: optimizeFor }],
      };
    }
    return { id: target.id };
  }
  return { id };
}
