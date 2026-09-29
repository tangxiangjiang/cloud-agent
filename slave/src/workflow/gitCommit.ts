// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { log } from "../log.js";
import type { WorkflowNode } from "./types.js";
import { phaseKeyFromNode } from "./progress.js";

const execFileAsync = promisify(execFile);

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await execFileAsync("git", args, {
    cwd,
    maxBuffer: 10 * 1024 * 1024,
    windowsHide: true,
  });
  return stdout.toString();
}

export function fallbackCommitMessage(opts: {
  node: WorkflowNode;
  comment?: string | null;
}): string {
  const phase = phaseKeyFromNode(opts.node);
  const title = (opts.node.title ?? "").trim() || opts.node.id;
  const head = phase
    ? `approve(${phase}): ${title}`
    : `approve(${opts.node.id}): ${title}`;
  const comment = (opts.comment ?? "").trim();
  if (comment) {
    return `${head}\n\nReview comment: ${comment}\n\nApproved via cloud-agent.`;
  }
  return `${head}\n\nApproved via cloud-agent.`;
}

/** Strip markdown fences / chatter; keep a usable commit message. */
export function sanitizeCommitMessage(raw: string, fallback: string): string {
  let s = raw.replace(/\r\n/g, "\n").trim();
  if (!s) return fallback;
  // Drop ``` wrappers if the model wrapped the message.
  s = s.replace(/^```(?:text|gitcommit|commit)?\n?/i, "").replace(/\n?```$/i, "");
  s = s.trim();
  // If model prefixed with labels, take after last heuristic marker.
  const labeled = s.match(
    /(?:^|\n)(?:commit message|message)\s*:\s*\n?([\s\S]+)$/i,
  );
  if (labeled?.[1]) s = labeled[1].trim();
  // Reject if it looks like the model refused or ran tools narrative.
  if (s.length < 3) return fallback;
  if (s.length > 4000) s = s.slice(0, 4000).trimEnd();
  // Subject line soft-cap for readability (keep body).
  const lines = s.split("\n");
  if (lines[0] && lines[0].length > 100) {
    lines[0] = `${lines[0].slice(0, 97)}...`;
    s = lines.join("\n");
  }
  return s || fallback;
}

export async function gitWorkingTreeDirty(repoCwd: string): Promise<boolean> {
  try {
    const out = (await git(repoCwd, ["status", "--porcelain"])).trim();
    return out.length > 0;
  } catch {
    return false;
  }
}

export async function collectCommitContext(repoCwd: string): Promise<{
  status: string;
  diffStat: string;
}> {
  let status = "";
  let diffStat = "";
  try {
    status = await git(repoCwd, ["status", "--porcelain"]);
  } catch {
    status = "";
  }
  try {
    diffStat = await git(repoCwd, ["diff", "--stat", "HEAD"]);
    if (!diffStat.trim()) {
      // Include untracked summary via status only when diff empty.
      diffStat = status;
    }
  } catch {
    diffStat = status;
  }
  // Cap size for AI prompt.
  if (status.length > 8000) status = `${status.slice(0, 8000)}\n…`;
  if (diffStat.length > 8000) diffStat = `${diffStat.slice(0, 8000)}\n…`;
  return { status, diffStat };
}

export type CommitMessageGenerator = (ctx: {
  status: string;
  diffStat: string;
  fallback: string;
  node: WorkflowNode;
  phaseKey: string | null;
}) => Promise<string | null>;

/**
 * Stage all changes under whitelist cwd and create a local commit.
 * Does not push. No-op when working tree clean.
 */
export async function commitOnApprove(opts: {
  repoCwd: string;
  node: WorkflowNode;
  comment?: string | null;
  /** Optional AI message writer; null/throw → fallback. */
  generateMessage?: CommitMessageGenerator | null;
}): Promise<
  | { ok: true; skipped: true; reason: string }
  | { ok: true; skipped: false; sha: string; message: string; ai: boolean }
  | { ok: false; reason: string }
> {
  const fallback = fallbackCommitMessage({
    node: opts.node,
    comment: opts.comment ?? null,
  });

  let dirty = false;
  try {
    dirty = await gitWorkingTreeDirty(opts.repoCwd);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return { ok: false, reason: `git status failed: ${message}` };
  }
  if (!dirty) {
    return { ok: true, skipped: true, reason: "working tree clean" };
  }

  const ctx = await collectCommitContext(opts.repoCwd);
    let message = fallback;
  let ai = false;
  if (opts.generateMessage) {
    try {
      const generated = await Promise.race([
        opts.generateMessage({
          status: ctx.status,
          diffStat: ctx.diffStat,
          fallback,
          node: opts.node,
          phaseKey: phaseKeyFromNode(opts.node),
        }),
        new Promise<null>((resolve) => {
          setTimeout(() => resolve(null), 45_000);
        }),
      ]);
      if (generated?.trim()) {
        message = sanitizeCommitMessage(generated, fallback);
        ai = true;
      } else {
        log.warn("AI commit message timed out or empty; using fallback");
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      log.warn("AI commit message failed; using fallback", { error: msg });
    }
  }

  try {
    await git(opts.repoCwd, ["add", "-A"]);
    // Allow empty? No — we already checked dirty; progress.md should be staged.
    await execFileAsync(
      "git",
      ["commit", "-m", message],
      {
        cwd: opts.repoCwd,
        maxBuffer: 10 * 1024 * 1024,
        windowsHide: true,
        env: {
          ...process.env,
          // Avoid hanging on gpg / editor
          GIT_EDITOR: "true",
        },
      },
    );
    const sha = (await git(opts.repoCwd, ["rev-parse", "HEAD"])).trim();
    return { ok: true, skipped: false, sha, message, ai };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { ok: false, reason: msg };
  }
}
