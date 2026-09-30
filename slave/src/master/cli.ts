// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import path from "node:path";
import {
  deleteSlaveEntry,
  loadMasterConfig,
  MasterConfigError,
  saveMasterConfig,
  upsertSlaveEntry,
} from "./config.js";
import { loadConfigFile } from "../config.js";
import { MasterControlError } from "./errors.js";
import { MasterGatewayClient } from "./gateway-client.js";
import { migrateSlaveConfigToMaster, migrateSummary } from "./migrate.js";
import { ProcessManager } from "./process-manager.js";
import type { MasterConfig, MasterSlaveEntry } from "./types.js";

function usage(): never {
  console.error(`Usage:
  npm run master -- serve [--config master.config.yaml]
  npm run master -- status [--config master.config.yaml]
  npm run master -- start <slaveId> [--config ...]
  npm run master -- stop <slaveId> [--config ...]
  npm run master -- restart <slaveId> [--config ...]
  npm run master -- list [--config ...]
  npm run master -- migrate --from config.yaml --to master.config.yaml

serve  — connect control WS; cold start does not spawn children
status/list/start/stop/restart — local CLI (no Gateway required)

State dir: <masterRoot>/.local/slaves/<slaveId>/
`);
  process.exit(2);
}

function parseArgs(argv: string[]): {
  cmd: string;
  slaveId?: string;
  configPath: string;
  from?: string;
  to?: string;
} {
  const args = [...argv];
  const cmd = args.shift();
  if (!cmd) usage();

  let configPath = path.resolve(
    process.env.MASTER_CONFIG ?? "master.config.yaml",
  );
  let slaveId: string | undefined;
  let from: string | undefined;
  let to: string | undefined;

  while (args.length > 0) {
    const a = args.shift()!;
    if (a === "--config" || a === "-c") {
      const v = args.shift();
      if (!v) usage();
      configPath = path.resolve(v);
    } else if (a === "--from") {
      from = args.shift();
      if (!from) usage();
    } else if (a === "--to") {
      to = args.shift();
      if (!to) usage();
    } else if (a.startsWith("-")) {
      usage();
    } else if (!slaveId) {
      slaveId = a;
    } else {
      usage();
    }
  }

  return {
    cmd,
    configPath,
    ...(slaveId !== undefined ? { slaveId } : {}),
    ...(from !== undefined ? { from } : {}),
    ...(to !== undefined ? { to } : {}),
  };
}

async function main(): Promise<void> {
  const { cmd, slaveId, configPath, from, to } = parseArgs(
    process.argv.slice(2),
  );

  if (cmd === "migrate") {
    if (!from || !to) {
      console.error("migrate requires --from and --to");
      usage();
    }
    const { master, mapping } = migrateSlaveConfigToMaster({
      fromSlaveConfig: from,
      toMasterConfig: to,
    });
    const slaveCfg = loadConfigFile(from, { allowMultiProject: true });
    console.log(migrateSummary(slaveCfg, master, mapping));
    console.log(`Wrote ${path.resolve(to)}`);
    return;
  }

  let cfg;
  try {
    cfg = loadMasterConfig(configPath);
  } catch (err) {
    if (err instanceof MasterConfigError) {
      console.error(err.message);
      process.exit(1);
    }
    throw err;
  }

  let liveCfg: MasterConfig = cfg;
  const pm = new ProcessManager(cfg, { masterConfigPath: configPath });
  pm.adoptOrphans();

  if (cmd === "serve") {
    const token = process.env[liveCfg.tokenEnv]?.trim();
    if (!token) {
      console.error(`env ${liveCfg.tokenEnv} is required for serve`);
      process.exit(1);
    }
    const client = new MasterGatewayClient({
      gatewayUrl: liveCfg.gatewayUrl,
      token,
      masterConfigPath: configPath,
      getConfig: () => liveCfg,
      setConfig: (c) => {
        liveCfg = c;
      },
      pm,
      log: (msg, meta) => {
        console.log(JSON.stringify({ msg, ...meta }));
      },
    });
    client.start();
    console.log(
      JSON.stringify({
        msg: "master serve",
        masterId: liveCfg.masterId,
        gatewayUrl: liveCfg.gatewayUrl,
        slaves: pm.listSnapshots().map((s) => ({
          id: s.id,
          state: s.state,
          enabled: s.enabled,
        })),
      }),
    );
    const shutdown = () => {
      client.stop();
      process.exit(0);
    };
    process.on("SIGINT", shutdown);
    process.on("SIGTERM", shutdown);
    return;
  }

  if (cmd === "status" || cmd === "list") {
    const rows = pm.listSnapshots();
    console.log(
      JSON.stringify(
        {
          masterId: cfg.masterId,
          name: cfg.name ?? null,
          slaves: rows,
        },
        null,
        2,
      ),
    );
    return;
  }

  if (cmd === "start" || cmd === "stop" || cmd === "restart") {
    if (!slaveId) {
      console.error(`${cmd} requires <slaveId>`);
      usage();
    }
    try {
      const snap =
        cmd === "start"
          ? await pm.start(slaveId)
          : cmd === "stop"
            ? await pm.stop(slaveId)
            : await pm.restart(slaveId);
      console.log(JSON.stringify(snap, null, 2));
      // Spawned children are unref'd; force exit so npm/tsx does not hang.
      process.exit(0);
    } catch (err) {
      if (err instanceof MasterControlError) {
        console.error(`${err.code}: ${err.message}`);
      } else {
        console.error(err instanceof Error ? err.message : String(err));
      }
      process.exit(1);
    }
    return;
  }

  if (cmd === "upsert-demo") {
    // Hidden helper for tests — not in usage.
    if (!slaveId) usage();
    const entry: MasterSlaveEntry = {
      id: slaveId,
      enabled: true,
      project: {
        id: `p_${slaveId}`,
        name: slaveId,
        cwd: path.resolve("."),
      },
    };
    const next = upsertSlaveEntry(cfg, entry);
    saveMasterConfig(configPath, next);
    console.log("ok");
    return;
  }

  if (cmd === "delete") {
    if (!slaveId) usage();
    const next = deleteSlaveEntry(cfg, slaveId);
    saveMasterConfig(configPath, next);
    console.log("ok");
    return;
  }

  usage();
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  process.exit(1);
});
