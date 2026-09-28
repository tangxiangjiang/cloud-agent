// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { execFile } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

export interface NodeDiffFile {
  path: string;
  status?: "added" | "modified" | "deleted" | "renamed" | string;
  additions?: number;
  deletions?: number;
  unifiedDiff?: string;
}

export interface NodeDiffPayload {
  workflowId: string;
  nodeId: string;
  baseline: string | null;
  files: NodeDiffFile[];
}

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await execFileAsync("git", args, {
    cwd,
    maxBuffer: 20 * 1024 * 1024,
    windowsHide: true,
  });
  return stdout.toString();
}

/** Capture baseline at node start: current HEAD commit. */
export async function captureBaseline(repoCwd: string): Promise<string | null> {
  try {
    const sha = (await git(repoCwd, ["rev-parse", "HEAD"])).trim();
    if (!sha) return null;
    return `git:${sha}`;
  } catch {
    return null;
  }
}

/**
 * Collect working-tree + index changes relative to baseline commit.
 * Baseline format: `git:<sha>` from captureBaseline.
 */
export async function collectNodeDiff(
  repoCwd: string,
  workflowId: string,
  nodeId: string,
  baseline: string | null,
): Promise<NodeDiffPayload> {
  const sha = baseline?.startsWith("git:") ? baseline.slice(4) : null;
  const files: NodeDiffFile[] = [];

  if (sha) {
    try {
      const nameStatus = await git(repoCwd, [
        "diff",
        "--name-status",
        sha,
      ]);
      const paths = parseNameStatus(nameStatus);
      const unified = await git(repoCwd, ["diff", sha]);
      const byPath = splitUnifiedByFile(unified);
      for (const { path: p, status } of paths) {
        const ud = byPath.get(normalizeGitPath(p)) ?? "";
        const { additions, deletions } = countDiffStats(ud);
        files.push({
          path: p,
          status,
          additions,
          deletions,
          unifiedDiff: ud || placeholderDiff(p, status),
        });
      }
    } catch {
      // fall through to untracked-only
    }

    try {
      const untracked = await git(repoCwd, [
        "ls-files",
        "--others",
        "--exclude-standard",
      ]);
      for (const line of untracked.split(/\r?\n/)) {
        const p = line.trim();
        if (!p) continue;
        if (files.some((f) => f.path === p)) continue;
        const content = safeRead(path.join(repoCwd, p));
        const ud = untrackedUnified(p, content);
        const { additions, deletions } = countDiffStats(ud);
        files.push({
          path: p,
          status: "added",
          additions,
          deletions,
          unifiedDiff: ud,
        });
      }
    } catch {
      /* ignore */
    }
  }

  return {
    workflowId,
    nodeId,
    baseline,
    files,
  };
}

export function parseNameStatus(text: string): Array<{ path: string; status: string }> {
  const out: Array<{ path: string; status: string }> = [];
  for (const line of text.split(/\r?\n/)) {
    if (!line.trim()) continue;
    const parts = line.split(/\t/);
    const code = parts[0] ?? "";
    if (code.startsWith("R") && parts.length >= 3) {
      out.push({ path: parts[2]!, status: "renamed" });
      continue;
    }
    const p = parts[1];
    if (!p) continue;
    const status =
      code.startsWith("A") ? "added" : code.startsWith("D") ? "deleted" : "modified";
    out.push({ path: p, status });
  }
  return out;
}

/** Split `git diff` output into per-file unified patches. */
export function splitUnifiedByFile(diffText: string): Map<string, string> {
  const map = new Map<string, string>();
  if (!diffText.trim()) return map;
  const chunks = diffText.split(/^diff --git /m);
  for (const chunk of chunks) {
    if (!chunk.trim()) continue;
    const body = chunk.startsWith("a/") || chunk.includes("\n") ? `diff --git ${chunk}` : chunk;
    const m = body.match(/^diff --git a\/(.+?) b\/(.+?)$/m);
    if (!m) continue;
    const filePath = normalizeGitPath(m[2] ?? m[1] ?? "");
    if (!filePath) continue;
    map.set(filePath, body.trimEnd() + "\n");
  }
  return map;
}

export function countDiffStats(unified: string): { additions: number; deletions: number } {
  let additions = 0;
  let deletions = 0;
  for (const line of unified.split(/\r?\n/)) {
    if (line.startsWith("+++") || line.startsWith("---") || line.startsWith("@@")) continue;
    if (line.startsWith("+")) additions++;
    else if (line.startsWith("-")) deletions++;
  }
  return { additions, deletions };
}

function normalizeGitPath(p: string): string {
  return p.replace(/\\/g, "/");
}

function placeholderDiff(p: string, status: string): string {
  return `diff --git a/${p} b/${p}\n# ${status} (binary or empty patch)\n`;
}

function untrackedUnified(p: string, content: string): string {
  const lines = content.split(/\r?\n/);
  // drop trailing empty from split
  if (lines.length && lines[lines.length - 1] === "") lines.pop();
  const body = lines.map((l) => `+${l}`).join("\n");
  const n = Math.max(lines.length, 1);
  return (
    `diff --git a/${p} b/${p}\n` +
    `new file mode 100644\n` +
    `--- /dev/null\n` +
    `+++ b/${p}\n` +
    `@@ -0,0 +1,${n} @@\n` +
    (body ? `${body}\n` : "+\n")
  );
}

function safeRead(abs: string): string {
  try {
    return readFileSync(abs, "utf8");
  } catch {
    return "";
  }
}
