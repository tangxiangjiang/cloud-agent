// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { parseArgs } from "node:util";
import { LocalAgentTaskHandler } from "./agent/localHandler.js";
import {
  ConfigError,
  configSummary,
  loadConfigFile,
  resolveConfigPath,
} from "./config.js";
import { GatewayClient } from "./gateway/client.js";
import type { TaskHandlers } from "./gateway/types.js";
import { log, setLogLevel } from "./log.js";
import { StubTaskHandler } from "./tasks/stubHandler.js";

function usage(): void {
  console.log(`Usage: slave [--config <path>] [--stub] [--log-level debug|info|warn|error]

Outbound Local Slave: Gateway WS + Cursor SDK Local Agent (no cloud).

Env:
  GATEWAY_TOKEN (or tokenEnv)  — Bearer from POST /v1/auth/pair
  CURSOR_API_KEY (or apiKeyEnv) — required unless --stub

--stub  use fake status/delta/done (no SDK; for Gateway-only联调)
`);
}

function buildHandlers(
  cfg: ReturnType<typeof loadConfigFile>,
  stub: boolean,
): TaskHandlers {
  if (stub) {
    log.warn("using stub task handler (--stub); SDK disabled");
    return new StubTaskHandler(cfg);
  }
  const apiKey = process.env[cfg.apiKeyEnv]?.trim();
  if (!apiKey) {
    throw new ConfigError(
      `env ${cfg.apiKeyEnv} is required for Local Agent (or pass --stub)`,
    );
  }
  return new LocalAgentTaskHandler({
    cfg,
    apiKey,
    defaultModel: cfg.defaultModel,
  });
}

async function main(): Promise<void> {
  const { values } = parseArgs({
    options: {
      config: { type: "string", short: "c" },
      stub: { type: "boolean", default: false },
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

  const handlers = buildHandlers(cfg, Boolean(values.stub));

  const clientOpts: ConstructorParameters<typeof GatewayClient>[0] = {
    url: cfg.gatewayUrl,
    token,
    slaveId: cfg.slaveId,
    repos: cfg.repos,
    handlers,
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
  log.info("slave running (Local Agent + outbound WS); Ctrl+C to stop");
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
