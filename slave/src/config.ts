// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { readFileSync } from "node:fs";
import path from "node:path";
import { parse as parseYaml } from "yaml";

export interface RepoConfig {
  id: string;
  name: string;
  /** Absolute local path; whitelist entry for Local Agent cwd. */
  cwd: string;
}

export interface SlaveConfig {
  gatewayUrl: string;
  slaveId: string;
  name?: string;
  repos: RepoConfig[];
  /** Environment variable name that holds the Cursor API key. */
  apiKeyEnv: string;
  /** Environment variable name that holds the Gateway Bearer token (M04-P02+). */
  tokenEnv: string;
  /** Default Local Agent model id when task.model is omitted. */
  defaultModel: string;
}

export class ConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ConfigError";
  }
}

type RawRepo = {
  id?: unknown;
  name?: unknown;
  cwd?: unknown;
};

type RawConfig = {
  gatewayUrl?: unknown;
  slaveId?: unknown;
  name?: unknown;
  repos?: unknown;
  apiKeyEnv?: unknown;
  tokenEnv?: unknown;
  defaultModel?: unknown;
  /** Forbidden: secrets must come from env named by apiKeyEnv. */
  apiKey?: unknown;
  cursorApiKey?: unknown;
  cloud?: unknown;
  local?: unknown;
};

function requireString(value: unknown, field: string): string {
  if (typeof value !== "string" || value.trim() === "") {
    throw new ConfigError(`${field} must be a non-empty string`);
  }
  return value.trim();
}

/** Resolve config path: CLI arg, SLAVE_CONFIG, or ./config.yaml. */
export function resolveConfigPath(cliPath?: string): string {
  const fromEnv = process.env.SLAVE_CONFIG?.trim();
  const chosen = cliPath?.trim() || fromEnv || "config.yaml";
  return path.resolve(chosen);
}

export function loadConfigFile(filePath: string): SlaveConfig {
  let text: string;
  try {
    text = readFileSync(filePath, "utf8");
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    throw new ConfigError(`cannot read config ${filePath}: ${msg}`);
  }
  let raw: unknown;
  try {
    raw = parseYaml(text);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    throw new ConfigError(`invalid YAML in ${filePath}: ${msg}`);
  }
  return validateConfig(raw);
}

/**
 * Validate and normalize slave config.
 * - repos[].cwd must be absolute paths (whitelist)
 * - rejects cloud / Cloud Agent defaults
 */
export function validateConfig(raw: unknown): SlaveConfig {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
    throw new ConfigError("config root must be a mapping");
  }
  const obj = raw as RawConfig;

  if (obj.apiKey !== undefined || obj.cursorApiKey !== undefined) {
    throw new ConfigError(
      "apiKey must not appear in config file; set apiKeyEnv and put the secret in that environment variable",
    );
  }
  if (obj.cloud !== undefined) {
    throw new ConfigError("cloud is not allowed; Local Slave uses local cwd only");
  }
  // Guard accidental SDK-shaped nesting
  if (
    obj.local !== undefined &&
    typeof obj.local === "object" &&
    obj.local !== null &&
    "cloud" in (obj.local as object)
  ) {
    throw new ConfigError("local.cloud is not allowed");
  }

  const gatewayUrl = requireString(obj.gatewayUrl, "gatewayUrl");
  const slaveId = requireString(obj.slaveId, "slaveId");
  const name =
    obj.name === undefined || obj.name === null
      ? undefined
      : requireString(obj.name, "name");
  const apiKeyEnv =
    obj.apiKeyEnv === undefined || obj.apiKeyEnv === null
      ? "CURSOR_API_KEY"
      : requireString(obj.apiKeyEnv, "apiKeyEnv");
  const tokenEnv =
    obj.tokenEnv === undefined || obj.tokenEnv === null
      ? "GATEWAY_TOKEN"
      : requireString(obj.tokenEnv, "tokenEnv");
  const defaultModel =
    obj.defaultModel === undefined || obj.defaultModel === null
      ? "composer-2.5"
      : requireString(obj.defaultModel, "defaultModel");

  if (!Array.isArray(obj.repos) || obj.repos.length === 0) {
    throw new ConfigError("repos must be a non-empty array");
  }

  const seenIds = new Set<string>();
  const repos: RepoConfig[] = [];
  for (let i = 0; i < obj.repos.length; i++) {
    const r = obj.repos[i] as RawRepo;
    const prefix = `repos[${i}]`;
    if (r === null || typeof r !== "object") {
      throw new ConfigError(`${prefix} must be a mapping`);
    }
    const id = requireString(r.id, `${prefix}.id`);
    const repoName = requireString(r.name, `${prefix}.name`);
    const cwdRaw = requireString(r.cwd, `${prefix}.cwd`);
    if (!path.isAbsolute(cwdRaw)) {
      throw new ConfigError(
        `${prefix}.cwd must be an absolute path (got ${JSON.stringify(cwdRaw)})`,
      );
    }
    const cwd = path.normalize(cwdRaw);
    if (seenIds.has(id)) {
      throw new ConfigError(`duplicate repos.id: ${id}`);
    }
    seenIds.add(id);
    repos.push({ id, name: repoName, cwd });
  }

  const cfg: SlaveConfig = {
    gatewayUrl,
    slaveId,
    repos,
    apiKeyEnv,
    tokenEnv,
    defaultModel,
  };
  if (name !== undefined) {
    cfg.name = name;
  }
  return cfg;
}

/** Look up a whitelist repo by id. */
export function findRepo(cfg: SlaveConfig, repoId: string): RepoConfig | undefined {
  return cfg.repos.find((r) => r.id === repoId);
}

/** Whether cwd is exactly one of the whitelist entries (normalized). */
export function isAllowedCwd(cfg: SlaveConfig, cwd: string): boolean {
  const norm = path.normalize(cwd);
  return cfg.repos.some((r) => r.cwd === norm);
}

/** Safe summary for logs (never includes API key or token values). */
export function configSummary(cfg: SlaveConfig): Record<string, unknown> {
  return {
    gatewayUrl: cfg.gatewayUrl,
    slaveId: cfg.slaveId,
    name: cfg.name ?? null,
    apiKeyEnv: cfg.apiKeyEnv,
    tokenEnv: cfg.tokenEnv,
    defaultModel: cfg.defaultModel,
    apiKeyPresent: Boolean(process.env[cfg.apiKeyEnv]),
    tokenPresent: Boolean(process.env[cfg.tokenEnv]),
    repos: cfg.repos.map((r) => ({ id: r.id, name: r.name, cwd: r.cwd })),
  };
}
