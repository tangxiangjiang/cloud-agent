// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { Agent, type AgentOptions } from "@cursor/sdk";
import { log } from "../log.js";
import { assertNoCloud } from "../agent/localHandler.js";

export interface InferredPhaseStatus {
  id: string;
  /** Hint only — never applied to Gateway nodes. */
  status: string;
  note?: string;
}

export interface SyncAiSummaryResult {
  summary: string;
  inferredPhaseStatus?: InferredPhaseStatus[];
}

export interface SyncSummaryContext {
  cwd: string;
  branch: string | null;
  head: string | null;
  dirty: boolean | null;
  dirtyNames: string[];
  recentCommits: string[];
  phases: Array<{ id: string; progressStatus: string; title?: string }>;
  ruleSummary: string;
}

const DEFAULT_TIMEOUT_MS = 20_000;

/** Strip fences / chatter; keep a short human-readable summary. */
export function sanitizeSyncSummary(raw: string, fallback: string): string {
  let s = raw.replace(/\r\n/g, "\n").trim();
  if (!s) return fallback;
  s = s.replace(/^```(?:json|text)?\n?/i, "").replace(/\n?```$/i, "").trim();
  // Prefer JSON object if the model returned one.
  if (s.startsWith("{")) {
    try {
      const obj = JSON.parse(s) as { summary?: unknown };
      if (typeof obj.summary === "string" && obj.summary.trim()) {
        s = obj.summary.trim();
      }
    } catch {
      /* keep text */
    }
  }
  if (s.length > 1200) s = `${s.slice(0, 1197)}...`;
  return s || fallback;
}

export function parseSyncAiJson(raw: string): SyncAiSummaryResult | null {
  let s = raw.replace(/\r\n/g, "\n").trim();
  if (!s) return null;
  s = s.replace(/^```(?:json|text)?\n?/i, "").replace(/\n?```$/i, "").trim();
  const start = s.indexOf("{");
  const end = s.lastIndexOf("}");
  if (start < 0 || end <= start) {
    // Plain text summary
    if (s.length >= 8) return { summary: s };
    return null;
  }
  try {
    const obj = JSON.parse(s.slice(start, end + 1)) as {
      summary?: unknown;
      inferredPhaseStatus?: unknown;
    };
    if (typeof obj.summary !== "string" || !obj.summary.trim()) return null;
    const out: SyncAiSummaryResult = { summary: obj.summary.trim() };
    if (Array.isArray(obj.inferredPhaseStatus)) {
      const inferred: InferredPhaseStatus[] = [];
      for (const row of obj.inferredPhaseStatus) {
        if (!row || typeof row !== "object") continue;
        const r = row as Record<string, unknown>;
        const id = String(r.id ?? "").trim();
        const status = String(r.status ?? "").trim();
        if (!id || !status) continue;
        const item: InferredPhaseStatus = { id, status };
        if (typeof r.note === "string" && r.note.trim()) {
          item.note = r.note.trim().slice(0, 200);
        }
        inferred.push(item);
      }
      if (inferred.length) out.inferredPhaseStatus = inferred.slice(0, 40);
    }
    return out;
  } catch {
    return { summary: s };
  }
}

function buildPrompt(ctx: SyncSummaryContext): string {
  const phases = ctx.phases
    .slice(0, 40)
    .map((p) => `- ${p.id}: ${p.progressStatus}${p.title ? ` (${p.title})` : ""}`)
    .join("\n");
  const commits = ctx.recentCommits.slice(0, 10).join("\n") || "(none)";
  const dirty =
    ctx.dirtyNames.slice(0, 40).join("\n") ||
    (ctx.dirty ? "(dirty but names unavailable)" : "(clean)");

  return `You summarize a local engineering workspace for a human reviewing sync status.

READ-ONLY. You MUST NOT:
- write, edit, delete, or create any files
- run git commit / push / checkout that changes the tree
- approve or reject Gateway workflow nodes
- invent secrets or paste large diffs

Context (structured; no full diffs):
branch: ${ctx.branch ?? "(unknown)"}
head: ${ctx.head ?? "(unknown)"}
dirty: ${ctx.dirty === null ? "unknown" : ctx.dirty}

Progress phases:
${phases || "(none)"}

Recent commits (oneline):
${commits}

Uncommitted path names only:
${dirty}

Heuristic summary (fallback):
${ctx.ruleSummary}

Reply with ONLY a JSON object (no markdown fences):
{
  "summary": "3-8 short sentences in the same language as phase titles when possible",
  "inferredPhaseStatus": [
    { "id": "Mxx-Pxx", "status": "approved|pending|done|unknown", "note": "optional hint" }
  ]
}
inferredPhaseStatus is OPTIONAL guesswork for humans only — never applied automatically.
`;
}

/**
 * Short Local Agent call for project.sync summary. Never writes files.
 * Returns null on failure / empty (caller keeps rule summary).
 */
export async function generateSyncSummaryWithAi(opts: {
  apiKey: string;
  model: string;
  ctx: SyncSummaryContext;
  timeoutMs?: number;
}): Promise<SyncAiSummaryResult | null> {
  const timeoutMs = opts.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const prompt = buildPrompt(opts.ctx);

  const createOptions: AgentOptions = {
    apiKey: opts.apiKey,
    model: { id: opts.model },
    local: {
      cwd: opts.ctx.cwd,
      settingSources: [],
    },
  };
  assertNoCloud(createOptions);

  const work = async (): Promise<SyncAiSummaryResult | null> => {
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
        log.warn("project.sync AI stream error; waiting anyway", { error: message });
      }
      const result = await run.wait();
      if (result.status === "error" || result.status === "cancelled") {
        log.warn("project.sync AI run not finished", { status: result.status });
        return null;
      }
      return parseSyncAiJson(text);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.warn("project.sync AI summary failed", { error: message });
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
  };

  try {
    return await Promise.race([
      work(),
      new Promise<null>((resolve) => {
        setTimeout(() => resolve(null), timeoutMs);
      }),
    ]);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    log.warn("project.sync AI summary error", { error: message });
    return null;
  }
}
