// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Cursor } from "@cursor/sdk";
import { log } from "../log.js";

export interface CursorModelEntry {
  id: string;
  label: string;
}

/**
 * Discover models available to this API key (Pro often only `default`;
 * Teams may include `auto-smart` and more). Failures return [].
 */
export async function listCursorModels(apiKey: string): Promise<CursorModelEntry[]> {
  try {
    const models = await Cursor.models.list({ apiKey });
    const out: CursorModelEntry[] = [];
    const seen = new Set<string>();
    for (const m of models) {
      const id = typeof m.id === "string" ? m.id.trim() : "";
      if (!id || seen.has(id)) continue;
      seen.add(id);
      const label =
        typeof m.displayName === "string" && m.displayName.trim()
          ? m.displayName.trim()
          : id;
      out.push({ id, label });
    }
    log.info("cursor models.list ok", {
      count: out.length,
      ids: out.map((m) => m.id),
    });
    return out;
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    log.warn("cursor models.list failed; using config autoModelId only", {
      error: message,
    });
    return [];
  }
}

/** Ids-only helper for Local Agent Auto resolution. */
export async function listAvailableModelIds(apiKey: string): Promise<Set<string>> {
  const models = await listCursorModels(apiKey);
  return new Set(models.map((m) => m.id));
}
