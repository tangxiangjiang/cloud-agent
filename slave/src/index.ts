// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { parseArgs } from "node:util";
import {
  ConfigError,
  configSummary,
  loadConfigFile,
  resolveConfigPath,
} from "./config.js";
import { GatewayClient } from "./gateway/client.js";
import { log, setLogLevel } from "./log.js";
import { StubTaskHandler } from "./tasks/stubHandler.js";

function usage(): void {
  console.log(`Usage: slave [--config <path>] [--log-level debug|info|warn|error]

Outbound Local Slave: load config, connect to Gateway WS, register, heartbeat,
handle task.assign with stub events (real SDK in M04-P03).

Requires env named by config tokenEnv (default GATEWAY_TOKEN) = Bearer from
POST /v1/auth/pair — not CURSOR_API_KEY.
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

  const token = process.env[cfg.tokenEnv]?.trim();
  if (!token) {
    throw new ConfigError(
      `env ${cfg.tokenEnv} is required (Gateway Bearer from POST /v1/auth/pair)`,
    );
  }
  if (!process.env[cfg.apiKeyEnv]) {
    log.warn(`env ${cfg.apiKeyEnv} is unset (required before Local Agent runs)`);
  }

  const clientOpts: ConstructorParameters<typeof GatewayClient>[0] = {
    url: cfg.gatewayUrl,
    token,
    slaveId: cfg.slaveId,
    repos: cfg.repos,
    handlers: new StubTaskHandler(cfg),
  };
  if (cfg.name !== undefined) {
    clientOpts.name = cfg.name;
  }
  const client = new GatewayClient(clientOpts);

  const shutdown = async (signal: string) => {
    log.info("shutting down", { signal });
    await client.stop();
    process.exit(0);
  };
  process.once("SIGINT", () => void shutdown("SIGINT"));
  process.once("SIGTERM", () => void shutdown("SIGTERM"));

  client.start();
  log.info("slave running (outbound WS); Ctrl+C to stop");
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
