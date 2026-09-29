// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import path from "node:path";
import { log } from "../log.js";

export interface SlaveRuntimeState {
  schemaVersion: 1;
  savedAt: string;
  /** workflowId\0nodeId → git:<sha> */
  baselines: Record<string, string | null>;
  /** workflowId\0nodeId → agent id */
  agentIds: Record<string, string>;
}

const SCHEMA = 1 as const;

export function defaultSlaveStatePath(): string {
  const fromEnv = process.env.SLAVE_STATE_FILE?.trim();
  if (fromEnv) return path.resolve(fromEnv);
  return path.resolve(process.cwd(), "../.local/slave-runtime.json");
}

export function loadSlaveRuntimeState(filePath: string): SlaveRuntimeState {
  try {
    const raw = readFileSync(filePath, "utf8");
    const data = JSON.parse(raw) as Partial<SlaveRuntimeState>;
    return {
      schemaVersion: SCHEMA,
      savedAt: data.savedAt ?? "",
      baselines: data.baselines ?? {},
      agentIds: data.agentIds ?? {},
    };
  } catch (err) {
    const code = (err as NodeJS.ErrnoException)?.code;
    if (code !== "ENOENT") {
      log.warn("slave runtime state load failed; starting empty", {
        path: filePath,
        error: err instanceof Error ? err.message : String(err),
      });
    }
    return {
      schemaVersion: SCHEMA,
      savedAt: "",
      baselines: {},
      agentIds: {},
    };
  }
}

export function saveSlaveRuntimeState(
  filePath: string,
  state: { baselines: Record<string, string | null>; agentIds: Record<string, string> },
): void {
  const dir = path.dirname(filePath);
  mkdirSync(dir, { recursive: true });
  const full: SlaveRuntimeState = {
    schemaVersion: SCHEMA,
    savedAt: new Date().toISOString(),
    baselines: state.baselines,
    agentIds: state.agentIds,
  };
  const tmp = `${filePath}.tmp`;
  writeFileSync(tmp, `${JSON.stringify(full, null, 2)}\n`, "utf8");
  renameSync(tmp, filePath);
}
