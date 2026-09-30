// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { OptimizeFor } from "../agent/modelSelection.js";

export type SlaveProcessState =
  | "stopped"
  | "starting"
  | "running"
  | "stopping"
  | "error";

export type MasterSlaveProject = {
  id: string;
  name: string;
  cwd: string;
  index?: string;
};

export type MasterSlaveEntry = {
  id: string;
  name?: string;
  enabled: boolean;
  project: MasterSlaveProject;
};

export type MasterDefaults = {
  apiKeyEnv: string;
  defaultModel: string;
  autoModelId: string;
  optimizeFor: OptimizeFor;
  /** Child Slave data-plane WS URL. */
  slaveGatewayUrl: string;
};

export type MasterConfig = {
  masterId: string;
  name?: string;
  gatewayUrl: string;
  tokenEnv: string;
  slaveCommand: string[];
  slaveCwd: string;
  allowedRoots: string[];
  maxRunningSlaves: number;
  maxConcurrentStarts: number;
  gracePeriodMs: number;
  defaults: MasterDefaults;
  slaves: MasterSlaveEntry[];
};

export type SlaveRuntimeSnapshot = {
  id: string;
  name?: string;
  enabled: boolean;
  project: MasterSlaveProject;
  state: SlaveProcessState;
  pid: number | null;
  lastError: string | null;
  startedAt: string | null;
};
