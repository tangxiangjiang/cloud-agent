// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Agent, type AgentOptions } from "@cursor/sdk";
import { log } from "../log.js";
import { assertNoCloud } from "./localHandler.js";
import {
  resolveModelSelection,
  type OptimizeFor,
} from "./modelSelection.js";

const DEFAULT_TIMEOUT_MS = 20_000;

/**
 * Short Local Agent call: suggest a chat session title from the first user message.
 */
export async function generateChatTitleWithAi(opts: {
  apiKey: string;
  model: string;
  optimizeFor?: OptimizeFor;
  autoModelId?: string;
  availableModelIds?: ReadonlySet<string> | readonly string[];
  cwd: string;
  text: string;
  timeoutMs?: number;
}): Promise<string | null> {
  const text = opts.text.trim();
  if (!text) return null;
  const timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const prompt = `Reply with ONLY a short conversation title for this first user message.
Rules:
- Max 16 Chinese characters OR max 8 English words
- No quotes, no markdown, no "Title:" prefix, no trailing punctuation
- Capture the intent, not a verbatim copy when too long

User message:
"""
${text.slice(0, 800)}
"""
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

  const work = async (): Promise<string | null> => {
    let agent: Awaited<ReturnType<typeof Agent.create>> | undefined;
    try {
      agent = await Agent.create(createOptions);
      const run = await agent.send(prompt);
      let out = "";
      try {
        for await (const event of run.stream()) {
          const ev = event as { type?: string; message?: { content?: unknown } };
          if (ev.type !== "assistant") continue;
          const content = ev.message?.content;
          if (!Array.isArray(content)) continue;
          for (const block of content) {
            const b = block as { type?: string; text?: string };
            if (b.type === "text" && typeof b.text === "string") {
              out += b.text;
            }
          }
        }
      } catch (streamErr) {
        const message = streamErr instanceof Error ? streamErr.message : String(streamErr);
        log.warn("chat-title AI stream error", { error: message });
      }
      await run.wait();
      return sanitizeTitle(out);
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
  };

  try {
    return await Promise.race([
      work(),
      new Promise<null>((resolve) => setTimeout(() => resolve(null), timeoutMs)),
    ]);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    log.warn("chat-title AI failed", { error: message });
    return null;
  }
}

export function sanitizeTitle(raw: string): string | null {
  let s = raw.trim();
  s = s.replace(/^title\s*[:：]\s*/i, "");
  s = s.replace(/^["'`「」『』]+|["'`「」『』]+$/g, "");
  s = s.split(/\r?\n/)[0]?.trim() ?? "";
  s = s.replace(/\s+/g, " ");
  if (!s) return null;
  const chars = [...s];
  if (chars.length > 24) {
    s = chars.slice(0, 24).join("") + "…";
  }
  return s;
}
