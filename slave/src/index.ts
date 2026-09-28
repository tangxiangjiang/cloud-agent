// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { parseArgs } from "node:util";
import {
  ConfigError,
  configSummary,
  loadConfigFile,
  resolveConfigPath,
} from "./config.js";
import { log, setLogLevel } from "./log.js";

function usage(): void {
  console.log(`Usage: slave [--config <path>] [--log-level debug|info|warn|error]

Loads and validates Local Slave config (gatewayUrl, slaveId, repos whitelist, apiKeyEnv).
Gateway WS client: M04-P02. Local Agent runs: M04-P03.
`);
}

async function main(): Promise<void> {
  const { values } = parseArgs({
    options: {
      config: { type: "string", short: "c" },
      "log-level": { type: "string", default: "info" },
      help: { type: "boolean", short: "h", default: false },
    },
    allowPositionals: false,
  });

  if (values.help) {
    usage();
    process.exit(0);
  }

  const level = values["log-level"] ?? "info";
  if (level !== "debug" && level !== "info" && level !== "warn" && level !== "error") {
    throw new ConfigError(`invalid --log-level: ${level}`);
  }
  setLogLevel(level);

  const configPath = resolveConfigPath(values.config);
  const cfg = loadConfigFile(configPath);

  log.info("slave config ok", { configPath, ...configSummary(cfg) });
  if (!process.env[cfg.apiKeyEnv]) {
    log.warn(`env ${cfg.apiKeyEnv} is unset (required before Local Agent runs)`);
  }

  log.info("M04-P01 complete: config load + cwd whitelist validation");
  log.info("next: Gateway outbound client (M04-P02)");
}

main().catch((err: unknown) => {
  const msg = err instanceof Error ? err.message : String(err);
  if (err instanceof ConfigError) {
    log.error(msg);
    process.exit(1);
  }
  log.error("fatal", { error: msg });
  process.exit(1);
});
