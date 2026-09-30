// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import path from "node:path";
import { loadConfigFile, type SlaveConfig } from "../config.js";
import {
  MasterConfigError,
  saveMasterConfig,
  validateMasterConfig,
} from "./config.js";
import type { MasterConfig, MasterSlaveEntry } from "./types.js";

export type MigrateOptions = {
  fromSlaveConfig: string;
  toMasterConfig: string;
  /** Project id that keeps the old slaveId (default: first project). */
  primaryProjectId?: string;
  masterId?: string;
  masterName?: string;
  masterGatewayUrl?: string;
  /** If true, do not write file — return config only. */
  dryRun?: boolean;
};

export type MigrateMapping = {
  projectId: string;
  projectName: string;
  cwd: string;
  oldSlaveId: string;
  newSlaveId: string;
  isPrimary: boolean;
};

/**
 * Split a multi-project slave config into master.config + N slave entries.
 * Primary project inherits the old slaveId; others get slave_<repoId>.
 * Child processes are not started (cold start).
 */
export function migrateSlaveConfigToMaster(opts: MigrateOptions): {
  master: MasterConfig;
  mapping: MigrateMapping[];
} {
  // Source may be legacy multi-project; migration is the escape hatch.
  const slaveCfg = loadConfigFile(opts.fromSlaveConfig, {
    allowMultiProject: true,
  });
  if (slaveCfg.projects.length === 0) {
    throw new MasterConfigError("source slave config has no projects");
  }

  const primaryId =
    opts.primaryProjectId ?? slaveCfg.projects[0]!.id;
  if (!slaveCfg.projects.some((p) => p.id === primaryId)) {
    throw new MasterConfigError(
      `primaryProjectId not found: ${primaryId}`,
    );
  }

  const baseMasterId = slaveCfg.slaveId.replace(/^slave_/, "") || "local";
  const masterId = opts.masterId ?? `master_${baseMasterId}`;

  const slaveGatewayUrl = slaveCfg.gatewayUrl;
  const masterGatewayUrl =
    opts.masterGatewayUrl ??
    (() => {
      try {
        const u = new URL(slaveCfg.gatewayUrl);
        u.pathname = "/v1/master/ws";
        return u.toString();
      } catch {
        return "ws://127.0.0.1:8080/v1/master/ws";
      }
    })();

  const mapping: MigrateMapping[] = [];
  const usedIds = new Set<string>();

  const slaves: MasterSlaveEntry[] = slaveCfg.projects.map((p) => {
    const isPrimary = p.id === primaryId;
    let id = isPrimary ? slaveCfg.slaveId : `slave_${p.id}`;
    if (usedIds.has(id)) {
      id = `${id}_x`;
    }
    usedIds.add(id);
    mapping.push({
      projectId: p.id,
      projectName: p.name,
      cwd: p.cwd,
      oldSlaveId: slaveCfg.slaveId,
      newSlaveId: id,
      isPrimary,
    });
    return {
      id,
      name: p.name,
      enabled: true,
      project: {
        id: p.id,
        name: p.name,
        cwd: p.cwd,
        ...(p.index ? { index: p.index } : {}),
      },
    };
  });

  const raw = {
    masterId,
    ...(opts.masterName || slaveCfg.name
      ? { name: opts.masterName ?? slaveCfg.name }
      : {}),
    gatewayUrl: masterGatewayUrl,
    tokenEnv: slaveCfg.tokenEnv,
    slaveCommand: ["node", "dist/index.js"],
    slaveCwd: path.resolve(path.dirname(path.resolve(opts.fromSlaveConfig))),
    defaults: {
      apiKeyEnv: slaveCfg.apiKeyEnv,
      defaultModel: slaveCfg.defaultModel,
      autoModelId: slaveCfg.autoModelId,
      optimizeFor: slaveCfg.optimizeFor,
      slaveGatewayUrl,
    },
    slaves,
  };

  const master = validateMasterConfig(raw);
  if (!opts.dryRun) {
    saveMasterConfig(opts.toMasterConfig, master);
  }
  return { master, mapping };
}

/** Hint text for operators after migration. */
export function migrateSummary(
  slaveCfg: SlaveConfig,
  master: MasterConfig,
  mapping: MigrateMapping[],
): string {
  const mapLines = mapping.map(
    (m) =>
      `  ${m.projectId} → ${m.newSlaveId}${m.isPrimary ? " (primary, keeps workflows)" : ""}`,
  );
  return [
    `Migrated ${slaveCfg.projects.length} project(s) → master ${master.masterId}`,
    "Mapping:",
    ...mapLines,
    "Cold start: no child processes started.",
    "Start primary: npm run master -- start " +
      (mapping.find((m) => m.isPrimary)?.newSlaveId ?? "<id>"),
    "Legacy multi-project single slave: npm run start -- --legacy-multi-project",
  ].join("\n");
}
