// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import fs from "node:fs";
import path from "node:path";
import YAML from "yaml";
import type { MasterConfig, MasterSlaveEntry } from "./types.js";

/** `<masterRoot>/.local/slaves/<slaveId>/` — never under business git roots. */
export function slaveStateDir(
  masterConfigPath: string,
  slaveId: string,
): string {
  const masterRoot = path.dirname(path.resolve(masterConfigPath));
  return path.join(masterRoot, ".local", "slaves", slaveId);
}

export function childConfigPath(
  masterConfigPath: string,
  slaveId: string,
): string {
  return path.join(slaveStateDir(masterConfigPath, slaveId), "config.yaml");
}

export function childPidPath(
  masterConfigPath: string,
  slaveId: string,
): string {
  return path.join(slaveStateDir(masterConfigPath, slaveId), "slave.pid");
}

export function childSpawnTokenPath(
  masterConfigPath: string,
  slaveId: string,
): string {
  return path.join(slaveStateDir(masterConfigPath, slaveId), "spawnToken");
}

export function childLogPath(
  masterConfigPath: string,
  slaveId: string,
): string {
  return path.join(
    slaveStateDir(masterConfigPath, slaveId),
    "logs",
    "slave.log",
  );
}

/** Build a one-project slave config object from master defaults + entry. */
export function buildChildSlaveConfig(
  master: MasterConfig,
  entry: MasterSlaveEntry,
): Record<string, unknown> {
  const project: Record<string, unknown> = {
    id: entry.project.id,
    name: entry.project.name,
    cwd: entry.project.cwd,
  };
  if (entry.project.index) {
    project.index = entry.project.index;
  }
  return {
    gatewayUrl: master.defaults.slaveGatewayUrl,
    slaveId: entry.id,
    ...(entry.name ? { name: entry.name } : {}),
    apiKeyEnv: master.defaults.apiKeyEnv,
    tokenEnv: master.tokenEnv,
    defaultModel: master.defaults.defaultModel,
    autoModelId: master.defaults.autoModelId,
    optimizeFor: master.defaults.optimizeFor,
    projects: [project],
  };
}

export function writeChildSlaveConfig(
  masterConfigPath: string,
  master: MasterConfig,
  entry: MasterSlaveEntry,
): string {
  const out = childConfigPath(masterConfigPath, entry.id);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  const text = YAML.stringify(buildChildSlaveConfig(master, entry), {
    lineWidth: 0,
  });
  const tmp = `${out}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, text, "utf8");
  fs.renameSync(tmp, out);
  return out;
}

export function clearSlaveStateDir(
  masterConfigPath: string,
  slaveId: string,
): void {
  const dir = slaveStateDir(masterConfigPath, slaveId);
  try {
    fs.rmSync(dir, { recursive: true, force: true });
  } catch {
    /* ignore */
  }
}
