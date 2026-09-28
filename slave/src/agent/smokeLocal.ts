// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * One-shot Local Agent smoke (no Gateway).
 *
 * Env:
 *   CURSOR_API_KEY  — required
 *   SMOKE_CWD       — optional absolute cwd (default: fixtures/smoke-repo)
 *   SMOKE_MODEL     — optional model id (default: composer-2.5)
 *
 * Requires Node.js >= 22.13 (@cursor/sdk engines).
 *
 *   npm run smoke-local
 */
import path from "node:path";
import { fileURLToPath } from "node:url";
import { Agent, CursorAgentError } from "@cursor/sdk";
import { assertNoCloud } from "./localHandler.js";
import { mapSdkMessage } from "./mapStream.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const defaultCwd = path.resolve(here, "../../fixtures/smoke-repo");

async function main(): Promise<void> {
  const apiKey = process.env.CURSOR_API_KEY?.trim();
  if (!apiKey) {
    console.error("CURSOR_API_KEY is required for smoke-local");
    process.exit(1);
  }
  const cwd = process.env.SMOKE_CWD?.trim() || defaultCwd;
  const model = process.env.SMOKE_MODEL?.trim() || "composer-2.5";
  const prompt =
    process.env.SMOKE_PROMPT?.trim() ||
    "Reply with exactly: smoke-ok. Do not modify any files.";

  const options: import("@cursor/sdk").AgentOptions = {
    apiKey,
    model: { id: model },
    local: { cwd, settingSources: [] },
  };
  assertNoCloud(options);

  console.log(JSON.stringify({ msg: "smoke start", cwd, model }));

  const agent = await Agent.create(options);
  try {
    const run = await agent.send(prompt);
    console.log(JSON.stringify({ msg: "run started", runId: run.id, agentId: run.agentId }));
    for await (const event of run.stream()) {
      mapSdkMessage(event, (kind, payload) => {
        console.log(JSON.stringify({ kind, payload }));
      });
    }
    const result = await run.wait();
    console.log(JSON.stringify({ msg: "smoke done", status: result.status }));
    if (result.status === "error") process.exit(2);
    if (result.status === "cancelled") process.exit(3);
  } catch (err) {
    if (err instanceof CursorAgentError) {
      console.error(
        JSON.stringify({
          phase: "startup",
          error: err.message,
          retryable: err.isRetryable,
        }),
      );
      process.exit(1);
    }
    throw err;
  } finally {
    await agent[Symbol.asyncDispose]();
  }
}

main().catch((err: unknown) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});
