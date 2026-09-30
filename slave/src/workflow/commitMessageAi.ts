// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Agent, type AgentOptions } from "@cursor/sdk";
import { log } from "../log.js";
import { assertNoCloud } from "../agent/localHandler.js";
import {
  resolveModelSelection,
  type OptimizeFor,
} from "../agent/modelSelection.js";
import type { WorkflowNode } from "./types.js";

/**
 * One-shot Local Agent: ask for a conventional commit message only.
 * Collects assistant text from the stream. Never enables cloud.
 */
export async function generateCommitMessageWithAi(opts: {
  apiKey: string;
  model: string;
  optimizeFor?: OptimizeFor;
  autoModelId?: string;
  availableModelIds?: ReadonlySet<string> | readonly string[];
  cwd: string;
  status: string;
  diffStat: string;
  node: WorkflowNode;
  phaseKey: string | null;
  fallback: string;
}): Promise<string | null> {
  const phase = opts.phaseKey ?? opts.node.id;
  const title = (opts.node.title ?? "").trim() || opts.node.id;
  const prompt = `You are writing a git commit message for an approved workflow node.

Node: ${opts.node.id}
Phase: ${phase}
Title: ${title}

git status --porcelain:
\`\`\`
${opts.status || "(empty)"}
\`\`\`

git diff --stat:
\`\`\`
${opts.diffStat || "(empty)"}
\`\`\`

Rules:
- Reply with ONLY the commit message text (subject + optional body).
- Prefer conventional commits, e.g. "feat(M01-P01): …" or "approve(${phase}): …"
- Subject ≤ 72 characters when possible.
- No markdown fences, no commentary, no tool calls narration.
- Do not modify any files; message only.
`;

  const createOptions: AgentOptions = {
    apiKey: opts.apiKey,
    model: resolveModelSelection(opts.model, opts.model, {
      optimizeFor: opts.optimizeFor ?? "cost",
      autoModelId: opts.autoModelId ?? "default",
      ...(opts.availableModelIds
        ? { availableModelIds: opts.availableModelIds }
        : {}),
    }),
    local: {
      cwd: opts.cwd,
      settingSources: [],
    },
  };
  assertNoCloud(createOptions);

  let agent: Awaited<ReturnType<typeof Agent.create>> | undefined;
  try {
    agent = await Agent.create(createOptions);
    const run = await agent.send(prompt);
    let text = "";
    try {
      for await (const event of run.stream()) {
        const ev = event as { type?: string; message?: { content?: unknown } };
        if (ev.type !== "assistant") continue;
        const content = ev.message?.content;
        if (!Array.isArray(content)) continue;
        for (const block of content) {
          const b = block as { type?: string; text?: string };
          if (b.type === "text" && typeof b.text === "string") {
            text += b.text;
          }
        }
      }
    } catch (streamErr) {
      const message = streamErr instanceof Error ? streamErr.message : String(streamErr);
      log.warn("commit-message AI stream error; waiting anyway", { error: message });
    }
    const result = await run.wait();
    if (result.status === "error" || result.status === "cancelled") {
      log.warn("commit-message AI run not finished", { status: result.status });
      return null;
    }
    const out = text.trim();
    return out || null;
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    log.warn("commit-message AI failed", { error: message });
    return null;
  } finally {
    if (agent) {
      try {
        await agent[Symbol.asyncDispose]();
      } catch {
        try {
          agent.close();
        } catch {
          /* ignore */
        }
      }
    }
  }
}
