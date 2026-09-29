// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { parseArgs } from "node:util";
import { LocalAgentTaskHandler } from "./agent/localHandler.js";
import {
  ConfigError,
  configSummary,
  isSyncAiSummaryEnabled,
  loadConfigFile,
  resolveConfigPath,
} from "./config.js";
import { GatewayClient } from "./gateway/client.js";
import type { TaskHandlers } from "./gateway/types.js";
import { log, setLogLevel } from "./log.js";
import { buildProjectCatalog } from "./project/catalog.js";
import { ensureAllProjectGitRepos } from "./project/ensureGit.js";
import { collectProjectSync } from "./project/syncCollect.js";
import { generateSyncSummaryWithAi } from "./project/syncSummaryAi.js";
import { StubTaskHandler } from "./tasks/stubHandler.js";
import { GatewayHttpApi, gatewayHttpBase } from "./workflow/http.js";
import { SerialDagScheduler } from "./workflow/scheduler.js";

function usage(): void {
  console.log(`Usage: slave [--config <path>] [--stub] [--log-level debug|info|warn|error]

Outbound Local Slave: Gateway WS + Cursor SDK Local Agent + serial DAG scheduler.

Env:
  GATEWAY_TOKEN (or tokenEnv)  — Bearer from POST /v1/auth/pair
  CURSOR_API_KEY (or apiKeyEnv) — required unless --stub
  SYNC_AI_SUMMARY=0            — disable optional AI summary on project.sync

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
  const http = new GatewayHttpApi(gatewayHttpBase(cfg.gatewayUrl), token);
  const apiKey = process.env[cfg.apiKeyEnv]?.trim();
  const syncAiEnabled =
    !values.stub && Boolean(apiKey) && isSyncAiSummaryEnabled(cfg);

  // Diff needs a local git HEAD; auto-init whitelist projects that lack a repo.
  await ensureAllProjectGitRepos(cfg.projects);

  const projects = buildProjectCatalog(cfg);
  log.info("project catalog loaded", {
    projects: projects.map((p) => ({
      id: p.id,
      index: p.index,
      milestones: p.milestones.length,
    })),
    syncAiSummary: syncAiEnabled,
  });

  // Client is created first so scheduler can emit via it; scheduler wired after.
  let scheduler!: SerialDagScheduler;
  let client!: GatewayClient;

  const clientOpts: ConstructorParameters<typeof GatewayClient>[0] = {
    url: cfg.gatewayUrl,
    token,
    slaveId: cfg.slaveId,
    repos: cfg.repos,
    projects,
    handlers,
    onWorkflowAssign: (run) => scheduler.enqueue(run),
    onWorkflowRevise: (msg) => scheduler.enqueueRevise(msg),
    onWorkflowReview: (msg) => scheduler.enqueueReview(msg),
    onProjectSync: async ({ requestId, repoId }) => {
      const payload = await collectProjectSync({
        cfg,
        slaveId: cfg.slaveId,
        requestId,
        repoId,
        ...(syncAiEnabled && apiKey
          ? {
              generateSummary: (ctx) =>
                generateSyncSummaryWithAi({
                  apiKey,
                  model: cfg.defaultModel,
                  ctx,
                }),
            }
          : {}),
      });
      try {
        await http.postProjectSync({
          requestId,
          slaveId: cfg.slaveId,
          repoId,
          payload: payload as unknown as Record<string, unknown>,
        });
        log.info("project.sync reported via HTTP", {
          repoId,
          requestId,
          branch: payload.branch,
          dirty: payload.dirty,
          summarySource: payload.summarySource,
        });
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        log.warn("project.sync HTTP failed; falling back to WS", {
          repoId,
          error: message,
        });
        client.sendProjectSyncResult({
          requestId,
          repoId,
          payload: payload as unknown as Record<string, unknown>,
        });
      }
    },
  };
  if (cfg.name !== undefined) {
    clientOpts.name = cfg.name;
  }
  client = new GatewayClient(clientOpts);

  const commitAi =
    !values.stub && apiKey
      ? { apiKey, model: cfg.defaultModel }
      : null;

  const stateFile =
    process.env.SLAVE_STATE_FILE?.trim() ||
    // When run from slave/, persist next to cloud-agent .local
    undefined;

  scheduler = new SerialDagScheduler({
    cfg,
    http,
    handlers,
    emit: (taskId, kind, payload) => client.emitTaskEvent(taskId, kind, payload),
    commitAi,
    ...(stateFile ? { stateFile } : {}),
  });

  const shutdown = async (signal: string) => {
    log.info("shutting down", { signal });
    await client.stop();
    process.exit(0);
  };
  process.once("SIGINT", () => void shutdown("SIGINT"));
  process.once("SIGTERM", () => void shutdown("SIGTERM"));

  client.start();
  log.info("slave running (Local Agent + DAG scheduler); Ctrl+C to stop");
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
