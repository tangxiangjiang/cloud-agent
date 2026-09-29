// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { execFile } from "node:child_process";
import { readFileSync } from "node:fs";
import { promisify } from "node:util";
import path from "node:path";
import { findRepo, type SlaveConfig } from "../config.js";
import { log } from "../log.js";
import { loadProjectMilestones, type Milestone } from "./milestones.js";
import { parseProgressPhases, type ProgressStatus } from "./parseProgress.js";
import type {
  InferredPhaseStatus,
  SyncAiSummaryResult,
  SyncSummaryContext,
} from "./syncSummaryAi.js";
import { sanitizeSyncSummary } from "./syncSummaryAi.js";
import { resolveProgressPath } from "../workflow/progress.js";

const execFileAsync = promisify(execFile);

export interface ProjectSyncPhase {
  id: string;
  progressStatus: ProgressStatus | string;
  title?: string;
}

/** schemaVersion=1 payload for Gateway project_sync (read-only collect). */
export interface ProjectSyncPayload {
  schemaVersion: 1;
  requestId: string;
  slaveId: string;
  repoId: string;
  cwd: string;
  syncedAt: string;
  branch: string | null;
  head: string | null;
  dirty: boolean | null;
  progressDoc: string | null;
  milestonesIndex: {
    schemaVersion: number;
    milestones: Array<{
      id: string;
      title: string;
      progressDoc?: string;
      phases: Array<{ id: string; title: string; phaseRef: string; dependsOn: string[] }>;
    }>;
  } | null;
  phases: ProjectSyncPhase[];
  recentCommits: string[];
  summary: string;
  /** How summary was produced. */
  summarySource: "rule" | "ai";
  /** Optional AI hints only — Gateway must not auto-apply. */
  inferredPhaseStatus?: InferredPhaseStatus[];
  activeWorkflows: unknown[];
  warnings: string[];
  error?: string;
}

export type SyncSummaryGenerator = (
  ctx: SyncSummaryContext,
) => Promise<SyncAiSummaryResult | null>;

export interface CollectProjectSyncInput {
  cfg: SlaveConfig;
  slaveId: string;
  requestId: string;
  repoId: string;
  now?: () => Date;
  /** Optional AI enricher; failures keep rule summary (M08-P05). */
  generateSummary?: SyncSummaryGenerator;
  /** AI call timeout (default 20s). */
  aiTimeoutMs?: number;
}

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await execFileAsync("git", args, {
    cwd,
    maxBuffer: 2 * 1024 * 1024,
    windowsHide: true,
  });
  return stdout.toString();
}

async function collectGit(cwd: string): Promise<{
  branch: string | null;
  head: string | null;
  dirty: boolean | null;
  recentCommits: string[];
  dirtyNames: string[];
  error?: string;
}> {
  try {
    const inside = (await git(cwd, ["rev-parse", "--is-inside-work-tree"])).trim();
    if (inside !== "true") {
      return {
        branch: null,
        head: null,
        dirty: null,
        recentCommits: [],
        dirtyNames: [],
        error: "not a git work tree",
      };
    }
  } catch {
    return {
      branch: null,
      head: null,
      dirty: null,
      recentCommits: [],
      dirtyNames: [],
      error: "git unavailable or not a repository",
    };
  }

  let branch: string | null = null;
  let head: string | null = null;
  let dirty: boolean | null = null;
  const recentCommits: string[] = [];
  const dirtyNames: string[] = [];
  const errs: string[] = [];

  try {
    branch = (await git(cwd, ["rev-parse", "--abbrev-ref", "HEAD"])).trim() || null;
  } catch {
    errs.push("branch");
  }
  try {
    const full = (await git(cwd, ["rev-parse", "HEAD"])).trim();
    head = full ? full.slice(0, 12) : null;
  } catch {
    errs.push("head");
  }
  try {
    const porcelain = await git(cwd, ["status", "--porcelain"]);
    dirty = porcelain.trim().length > 0;
    for (const line of porcelain.split(/\r?\n/)) {
      const t = line.trimEnd();
      if (!t) continue;
      // status XY + space + path (or rename "a -> b")
      const pathPart = t.length > 3 ? t.slice(3).trim() : t;
      const name = pathPart.includes(" -> ")
        ? pathPart.split(" -> ").pop()!.trim()
        : pathPart;
      if (name) dirtyNames.push(name.slice(0, 200));
      if (dirtyNames.length >= 40) break;
    }
  } catch {
    errs.push("dirty");
  }
  try {
    const logOut = await git(cwd, ["log", "-n", "10", "--oneline"]);
    for (const line of logOut.split(/\r?\n/)) {
      const t = line.trim();
      if (t) recentCommits.push(t.slice(0, 200));
    }
  } catch {
    // optional
  }

  return {
    branch,
    head,
    dirty,
    recentCommits,
    dirtyNames,
    ...(errs.length ? { error: `git partial failure: ${errs.join(",")}` } : {}),
  };
}

function compactMilestones(milestones: Milestone[]): NonNullable<
  ProjectSyncPayload["milestonesIndex"]
> {
  return {
    schemaVersion: 1,
    milestones: milestones.map((m) => ({
      id: m.id,
      title: m.title,
      ...(m.progressDoc ? { progressDoc: m.progressDoc } : {}),
      phases: m.phases.map((p) => ({
        id: p.id,
        title: p.title,
        phaseRef: p.phaseRef,
        dependsOn: [...p.dependsOn],
      })),
    })),
  };
}

function pickProgressDoc(
  projectCwd: string,
  milestones: Milestone[],
  indexHint?: string,
): string | null {
  for (const m of milestones) {
    if (m.progressDoc?.trim()) {
      const r = resolveProgressPath(projectCwd, m.progressDoc.trim());
      if (r.ok) return r.relPath;
    }
  }
  // Common default when index omitted progressDoc
  const fallback = "ai/progress.md";
  const r = resolveProgressPath(projectCwd, fallback);
  if (r.ok) {
    try {
      readFileSync(r.absPath, "utf8");
      return r.relPath;
    } catch {
      /* missing */
    }
  }
  void indexHint;
  return null;
}

function titleMap(milestones: Milestone[]): Map<string, string> {
  const map = new Map<string, string>();
  for (const m of milestones) {
    for (const p of m.phases) {
      map.set(p.id, p.title);
    }
  }
  return map;
}

function buildRuleSummary(opts: {
  branch: string | null;
  dirty: boolean | null;
  phases: ProjectSyncPhase[];
  milestoneCount: number;
  gitError?: string;
  progressMissing: boolean;
  unknownRepo?: boolean;
}): string {
  if (opts.unknownRepo) {
    return "repoId not in slave whitelist; sync skipped.";
  }
  const parts: string[] = [];
  if (opts.gitError) {
    parts.push(`git: ${opts.gitError}`);
  } else if (opts.branch) {
    parts.push(`branch ${opts.branch}`);
  } else {
    parts.push("no branch");
  }
  if (opts.dirty === true) parts.push("working tree dirty");
  else if (opts.dirty === false) parts.push("working tree clean");
  const approved = opts.phases.filter((p) => p.progressStatus === "approved").length;
  const done = opts.phases.filter((p) => p.progressStatus === "done").length;
  const pending = opts.phases.filter((p) => p.progressStatus === "pending").length;
  if (opts.progressMissing) {
    parts.push("no progress.md");
  } else {
    parts.push(
      `progress: ${approved} approved, ${done} done, ${pending} pending (of ${opts.phases.length})`,
    );
  }
  parts.push(`milestones index: ${opts.milestoneCount}`);
  return parts.join("; ") + ".";
}

export { buildRuleSummary };

/**
 * Structured read-only collect for project.sync.
 * Never writes workspace files; never runs git commit.
 */
export async function collectProjectSync(
  input: CollectProjectSyncInput,
): Promise<ProjectSyncPayload> {
  const now = input.now ?? (() => new Date());
  const syncedAt = now().toISOString();
  const requestId = input.requestId.trim() || `req_${Date.now()}`;
  const repoId = input.repoId.trim();
  const slaveId = input.slaveId.trim();

  const base = (extra: Partial<ProjectSyncPayload> & { cwd: string }): ProjectSyncPayload => ({
    schemaVersion: 1,
    requestId,
    slaveId,
    repoId,
    syncedAt,
    branch: null,
    head: null,
    dirty: null,
    progressDoc: null,
    milestonesIndex: null,
    phases: [],
    recentCommits: [],
    summary: "",
    summarySource: "rule",
    activeWorkflows: [],
    warnings: [],
    ...extra,
  });

  const project = findRepo(input.cfg, repoId);
  if (!project) {
    log.warn("project.sync unknown repoId", { repoId });
    return base({
      cwd: "",
      error: "repoId not registered on this slave",
      summary: buildRuleSummary({
        branch: null,
        dirty: null,
        phases: [],
        milestoneCount: 0,
        progressMissing: true,
        unknownRepo: true,
      }),
    });
  }

  const cwd = project.cwd;
  const loaded = loadProjectMilestones(cwd, project.index);
  const milestones = loaded.milestones;
  const milestonesIndex = milestones.length ? compactMilestones(milestones) : null;
  if (loaded.error) {
    log.warn("project.sync milestone index", { repoId, error: loaded.error });
  }

  const progressDoc = pickProgressDoc(cwd, milestones, project.index);
  const titles = titleMap(milestones);
  let phases: ProjectSyncPhase[] = [];
  let progressMissing = true;

  if (progressDoc) {
    const resolved = resolveProgressPath(cwd, progressDoc);
    if (resolved.ok) {
      try {
        const text = readFileSync(resolved.absPath, "utf8");
        progressMissing = false;
        phases = parseProgressPhases(text).map((row) => {
          const title = titles.get(row.id);
          return title
            ? { id: row.id, progressStatus: row.progressStatus, title }
            : { id: row.id, progressStatus: row.progressStatus };
        });
      } catch {
        progressMissing = true;
      }
    }
  }

  // Ensure index phases appear (pending if absent from progress).
  for (const m of milestones) {
    for (const p of m.phases) {
      if (!phases.some((x) => x.id === p.id)) {
        phases.push({
          id: p.id,
          progressStatus: "pending",
          title: p.title,
        });
      }
    }
  }
  phases.sort((a, b) => a.id.localeCompare(b.id));

  const gitState = await collectGit(cwd);
  const errors: string[] = [];
  if (loaded.error) errors.push(`index: ${loaded.error}`);
  if (gitState.error) errors.push(gitState.error);

  const summary = buildRuleSummary({
    branch: gitState.branch,
    dirty: gitState.dirty,
    phases,
    milestoneCount: milestones.length,
    ...(gitState.error ? { gitError: gitState.error } : {}),
    progressMissing,
  });

  const payload = base({
    cwd,
    branch: gitState.branch,
    head: gitState.head,
    dirty: gitState.dirty,
    progressDoc,
    milestonesIndex,
    phases,
    recentCommits: gitState.recentCommits,
    summary,
    summarySource: "rule",
    ...(errors.length ? { error: errors.join("; ") } : {}),
  });

  if (input.generateSummary) {
    const ctx: SyncSummaryContext = {
      cwd,
      branch: gitState.branch,
      head: gitState.head,
      dirty: gitState.dirty,
      dirtyNames: gitState.dirtyNames,
      recentCommits: gitState.recentCommits,
      phases,
      ruleSummary: summary,
    };
    const timeoutMs = input.aiTimeoutMs ?? 20_000;
    try {
      const ai = await Promise.race([
        input.generateSummary(ctx),
        new Promise<null>((resolve) => {
          setTimeout(() => resolve(null), timeoutMs);
        }),
      ]);
      if (ai?.summary?.trim()) {
        payload.summary = sanitizeSyncSummary(ai.summary, summary);
        payload.summarySource = "ai";
        if (ai.inferredPhaseStatus?.length) {
          payload.inferredPhaseStatus = ai.inferredPhaseStatus;
        }
      } else {
        log.info("project.sync AI summary empty/timeout; keeping rule summary", {
          repoId,
        });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      log.warn("project.sync AI enrich failed; keeping rule summary", {
        repoId,
        error: message,
      });
    }
  }

  // Guard: never include huge blobs (diff / secrets). Cap JSON-ish size soft.
  const approx = JSON.stringify(payload).length;
  if (approx > 200_000) {
    payload.recentCommits = payload.recentCommits.slice(0, 3);
    payload.milestonesIndex = payload.milestonesIndex
      ? {
          schemaVersion: 1,
          milestones: payload.milestonesIndex.milestones.map((m) => ({
            id: m.id,
            title: m.title,
            ...(m.progressDoc ? { progressDoc: m.progressDoc } : {}),
            phases: m.phases.map((p) => ({
              id: p.id,
              title: p.title,
              phaseRef: path.basename(p.phaseRef),
              dependsOn: p.dependsOn,
            })),
          })),
        }
      : null;
    payload.error = [payload.error, "payload truncated for size"]
      .filter(Boolean)
      .join("; ");
  }

  return payload;
}
