// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import fs from "node:fs";
import path from "node:path";
import YAML from "yaml";
import {
  parseOptimizeFor,
  type OptimizeFor,
} from "../agent/modelSelection.js";
import {
  DEFAULT_IDLE_TIMEOUT_MS,
  DEFAULT_TASK_TIMEOUT_MS,
} from "../config.js";
import type {
  MasterConfig,
  MasterDefaults,
  MasterSlaveEntry,
  MasterSlaveProject,
} from "./types.js";

export class MasterConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "MasterConfigError";
  }
}

type RawProject = {
  id?: unknown;
  name?: unknown;
  cwd?: unknown;
  index?: unknown;
};

type RawSlave = {
  id?: unknown;
  name?: unknown;
  enabled?: unknown;
  project?: unknown;
};

type RawDefaults = {
  apiKeyEnv?: unknown;
  defaultModel?: unknown;
  autoModelId?: unknown;
  optimizeFor?: unknown;
  slaveGatewayUrl?: unknown;
  taskTimeoutMs?: unknown;
  idleTimeoutMs?: unknown;
};

type RawMaster = {
  masterId?: unknown;
  name?: unknown;
  gatewayUrl?: unknown;
  tokenEnv?: unknown;
  slaveCommand?: unknown;
  slaveCwd?: unknown;
  allowedRoots?: unknown;
  maxRunningSlaves?: unknown;
  maxConcurrentStarts?: unknown;
  gracePeriodMs?: unknown;
  defaults?: unknown;
  slaves?: unknown;
};

function requireString(v: unknown, field: string): string {
  if (typeof v !== "string" || v.trim() === "") {
    throw new MasterConfigError(`${field} must be a non-empty string`);
  }
  return v.trim();
}

function optionalString(v: unknown, field: string): string | undefined {
  if (v === undefined || v === null) return undefined;
  return requireString(v, field);
}

function requireId(v: unknown, field: string): string {
  const s = requireString(v, field);
  if (!/^[a-zA-Z0-9_][a-zA-Z0-9_-]{0,63}$/.test(s)) {
    throw new MasterConfigError(
      `${field} must match [a-zA-Z0-9_][a-zA-Z0-9_-]{0,63}`,
    );
  }
  return s;
}

function requirePositiveInt(
  v: unknown,
  field: string,
  fallback: number,
): number {
  if (v === undefined || v === null) return fallback;
  if (typeof v !== "number" || !Number.isInteger(v) || v < 1) {
    throw new MasterConfigError(`${field} must be an integer >= 1`);
  }
  return v;
}

function requireNonNegInt(
  v: unknown,
  field: string,
  fallback: number,
): number {
  if (v === undefined || v === null) return fallback;
  if (typeof v !== "number" || !Number.isInteger(v) || v < 0) {
    throw new MasterConfigError(`${field} must be an integer >= 0`);
  }
  return v;
}

function deriveSlaveGatewayUrl(masterGatewayUrl: string): string {
  try {
    const u = new URL(masterGatewayUrl);
    u.pathname = "/v1/slave/ws";
    u.search = "";
    u.hash = "";
    return u.toString();
  } catch {
    return "ws://127.0.0.1:8080/v1/slave/ws";
  }
}

function parseProject(raw: unknown, prefix: string): MasterSlaveProject {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
    throw new MasterConfigError(`${prefix} must be a mapping`);
  }
  const r = raw as RawProject;
  const cwdRaw = requireString(r.cwd, `${prefix}.cwd`);
  if (!path.isAbsolute(cwdRaw)) {
    throw new MasterConfigError(
      `${prefix}.cwd must be an absolute path (got ${JSON.stringify(cwdRaw)})`,
    );
  }
  const project: MasterSlaveProject = {
    id: requireString(r.id, `${prefix}.id`),
    name: requireString(r.name, `${prefix}.name`),
    cwd: path.normalize(cwdRaw),
  };
  if (r.index !== undefined && r.index !== null) {
    const indexRel = requireString(r.index, `${prefix}.index`);
    if (path.isAbsolute(indexRel) || indexRel.split(/[/\\]/).includes("..")) {
      throw new MasterConfigError(
        `${prefix}.index must be a relative path without ..`,
      );
    }
    project.index = indexRel.replace(/\\/g, "/");
  }
  return project;
}

function parseSlave(raw: unknown, i: number): MasterSlaveEntry {
  const prefix = `slaves[${i}]`;
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
    throw new MasterConfigError(`${prefix} must be a mapping`);
  }
  const r = raw as RawSlave;
  let enabled = true;
  if (r.enabled !== undefined && r.enabled !== null) {
    if (typeof r.enabled !== "boolean") {
      throw new MasterConfigError(`${prefix}.enabled must be a boolean`);
    }
    enabled = r.enabled;
  }
  const name = optionalString(r.name, `${prefix}.name`);
  return {
    id: requireId(r.id, `${prefix}.id`),
    ...(name !== undefined ? { name } : {}),
    enabled,
    project: parseProject(r.project, `${prefix}.project`),
  };
}

function parseDefaults(
  raw: unknown,
  masterGatewayUrl: string,
): MasterDefaults {
  const r =
    raw === undefined || raw === null
      ? ({} as RawDefaults)
      : (raw as RawDefaults);
  if (raw !== undefined && raw !== null) {
    if (typeof raw !== "object" || Array.isArray(raw)) {
      throw new MasterConfigError("defaults must be a mapping");
    }
  }
  let optimizeFor: OptimizeFor = "cost";
  if (r.optimizeFor !== undefined && r.optimizeFor !== null) {
    try {
      optimizeFor = parseOptimizeFor(r.optimizeFor);
    } catch (err) {
      throw new MasterConfigError(
        err instanceof Error ? err.message : String(err),
      );
    }
  }
  return {
    apiKeyEnv:
      optionalString(r.apiKeyEnv, "defaults.apiKeyEnv") ?? "CURSOR_API_KEY",
    defaultModel:
      optionalString(r.defaultModel, "defaults.defaultModel") ?? "default",
    autoModelId:
      optionalString(r.autoModelId, "defaults.autoModelId") ?? "default",
    optimizeFor,
    slaveGatewayUrl:
      optionalString(r.slaveGatewayUrl, "defaults.slaveGatewayUrl") ??
      deriveSlaveGatewayUrl(masterGatewayUrl),
    taskTimeoutMs: requireNonNegInt(
      r.taskTimeoutMs,
      "defaults.taskTimeoutMs",
      DEFAULT_TASK_TIMEOUT_MS,
    ),
    idleTimeoutMs: requireNonNegInt(
      r.idleTimeoutMs,
      "defaults.idleTimeoutMs",
      DEFAULT_IDLE_TIMEOUT_MS,
    ),
  };
}

function isUnderRoot(cwd: string, root: string): boolean {
  const nCwd = path.normalize(cwd);
  const nRoot = path.normalize(root);
  if (nCwd === nRoot) return true;
  const prefix = nRoot.endsWith(path.sep) ? nRoot : nRoot + path.sep;
  return nCwd.startsWith(prefix);
}

/** Validate and normalize master config object. */
export function validateMasterConfig(raw: unknown): MasterConfig {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
    throw new MasterConfigError("config root must be a mapping");
  }
  const obj = raw as RawMaster;
  const masterId = requireId(obj.masterId, "masterId");
  const gatewayUrl = requireString(obj.gatewayUrl, "gatewayUrl");
  const tokenEnv =
    optionalString(obj.tokenEnv, "tokenEnv") ?? "GATEWAY_TOKEN";
  const name = optionalString(obj.name, "name");

  let slaveCommand: string[];
  if (obj.slaveCommand === undefined || obj.slaveCommand === null) {
    slaveCommand = ["node", "dist/index.js"];
  } else if (
    !Array.isArray(obj.slaveCommand) ||
    obj.slaveCommand.length === 0 ||
    !obj.slaveCommand.every((x) => typeof x === "string" && x.trim() !== "")
  ) {
    throw new MasterConfigError(
      "slaveCommand must be a non-empty array of strings",
    );
  } else {
    slaveCommand = obj.slaveCommand.map((x) => String(x));
  }

  const slaveCwdRaw =
    optionalString(obj.slaveCwd, "slaveCwd") ?? process.cwd();
  const slaveCwd = path.isAbsolute(slaveCwdRaw)
    ? path.normalize(slaveCwdRaw)
    : path.resolve(process.cwd(), slaveCwdRaw);

  let allowedRoots: string[] = [];
  if (obj.allowedRoots !== undefined && obj.allowedRoots !== null) {
    if (!Array.isArray(obj.allowedRoots)) {
      throw new MasterConfigError("allowedRoots must be an array");
    }
    allowedRoots = obj.allowedRoots.map((r, i) => {
      const s = requireString(r, `allowedRoots[${i}]`);
      if (!path.isAbsolute(s)) {
        throw new MasterConfigError(
          `allowedRoots[${i}] must be an absolute path`,
        );
      }
      return path.normalize(s);
    });
  }

  if (!Array.isArray(obj.slaves) || obj.slaves.length === 0) {
    throw new MasterConfigError("slaves must be a non-empty array");
  }
  const seenSlave = new Set<string>();
  const seenProject = new Set<string>();
  const seenCwd = new Set<string>();
  const slaves = obj.slaves.map((s, i) => {
    const entry = parseSlave(s, i);
    if (seenSlave.has(entry.id)) {
      throw new MasterConfigError(`duplicate slave id: ${entry.id}`);
    }
    if (seenProject.has(entry.project.id)) {
      throw new MasterConfigError(
        `repo_conflict: duplicate project id across slaves: ${entry.project.id}`,
      );
    }
    if (seenCwd.has(entry.project.cwd)) {
      throw new MasterConfigError(
        `repo_conflict: duplicate project.cwd across slaves: ${entry.project.cwd}`,
      );
    }
    seenSlave.add(entry.id);
    seenProject.add(entry.project.id);
    seenCwd.add(entry.project.cwd);
    if (allowedRoots.length > 0) {
      const ok = allowedRoots.some((root) =>
        isUnderRoot(entry.project.cwd, root),
      );
      if (!ok) {
        throw new MasterConfigError(
          `cwd_denied: slaves[${i}].project.cwd is outside allowedRoots`,
        );
      }
    }
    return entry;
  });

  return {
    masterId,
    ...(name !== undefined ? { name } : {}),
    gatewayUrl,
    tokenEnv,
    slaveCommand,
    slaveCwd,
    allowedRoots,
    maxRunningSlaves: requirePositiveInt(
      obj.maxRunningSlaves,
      "maxRunningSlaves",
      4,
    ),
    maxConcurrentStarts: requirePositiveInt(
      obj.maxConcurrentStarts,
      "maxConcurrentStarts",
      2,
    ),
    gracePeriodMs: requireNonNegInt(obj.gracePeriodMs, "gracePeriodMs", 15000),
    defaults: parseDefaults(obj.defaults, gatewayUrl),
    slaves,
  };
}

export function loadMasterConfig(filePath: string): MasterConfig {
  const abs = path.resolve(filePath);
  let text: string;
  try {
    text = fs.readFileSync(abs, "utf8");
  } catch (err) {
    throw new MasterConfigError(
      `cannot read master config ${abs}: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
  let parsed: unknown;
  try {
    parsed = YAML.parse(text);
  } catch (err) {
    throw new MasterConfigError(
      `invalid YAML in ${abs}: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
  return validateMasterConfig(parsed);
}

/** Serialize config back to a plain object suitable for YAML dump. */
export function masterConfigToPlain(cfg: MasterConfig): Record<string, unknown> {
  return {
    masterId: cfg.masterId,
    ...(cfg.name ? { name: cfg.name } : {}),
    gatewayUrl: cfg.gatewayUrl,
    tokenEnv: cfg.tokenEnv,
    slaveCommand: cfg.slaveCommand,
    slaveCwd: cfg.slaveCwd,
    ...(cfg.allowedRoots.length > 0
      ? { allowedRoots: cfg.allowedRoots }
      : {}),
    maxRunningSlaves: cfg.maxRunningSlaves,
    maxConcurrentStarts: cfg.maxConcurrentStarts,
    gracePeriodMs: cfg.gracePeriodMs,
    defaults: {
      apiKeyEnv: cfg.defaults.apiKeyEnv,
      defaultModel: cfg.defaults.defaultModel,
      autoModelId: cfg.defaults.autoModelId,
      optimizeFor: cfg.defaults.optimizeFor,
      slaveGatewayUrl: cfg.defaults.slaveGatewayUrl,
      taskTimeoutMs: cfg.defaults.taskTimeoutMs,
      idleTimeoutMs: cfg.defaults.idleTimeoutMs,
    },
    slaves: cfg.slaves.map((s) => ({
      id: s.id,
      ...(s.name ? { name: s.name } : {}),
      enabled: s.enabled,
      project: {
        id: s.project.id,
        name: s.project.name,
        cwd: s.project.cwd,
        ...(s.project.index ? { index: s.project.index } : {}),
      },
    })),
  };
}

export function saveMasterConfig(filePath: string, cfg: MasterConfig): void {
  const abs = path.resolve(filePath);
  const dir = path.dirname(abs);
  fs.mkdirSync(dir, { recursive: true });
  const text = YAML.stringify(masterConfigToPlain(cfg), {
    lineWidth: 0,
  });
  const tmp = `${abs}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, text, "utf8");
  fs.renameSync(tmp, abs);
}

/** Upsert a slave entry (by id) and return new config. */
export function upsertSlaveEntry(
  cfg: MasterConfig,
  entry: MasterSlaveEntry,
): MasterConfig {
  if (cfg.allowedRoots.length > 0) {
    const ok = cfg.allowedRoots.some((root) =>
      isUnderRoot(entry.project.cwd, root),
    );
    if (!ok) {
      throw new MasterConfigError(
        "cwd_denied: project.cwd is outside allowedRoots",
      );
    }
  }
  const others = cfg.slaves.filter((s) => s.id !== entry.id);
  if (others.some((s) => s.project.id === entry.project.id)) {
    throw new MasterConfigError(
      `repo_conflict: duplicate project id across slaves: ${entry.project.id}`,
    );
  }
  if (others.some((s) => s.project.cwd === entry.project.cwd)) {
    throw new MasterConfigError(
      `repo_conflict: duplicate project.cwd across slaves: ${entry.project.cwd}`,
    );
  }
  return { ...cfg, slaves: [...others, entry] };
}

export function deleteSlaveEntry(
  cfg: MasterConfig,
  slaveId: string,
): MasterConfig {
  const next = cfg.slaves.filter((s) => s.id !== slaveId);
  if (next.length === cfg.slaves.length) {
    throw new MasterConfigError(`unknown slave id: ${slaveId}`);
  }
  if (next.length === 0) {
    throw new MasterConfigError("cannot delete the last slave entry");
  }
  return { ...cfg, slaves: next };
}
