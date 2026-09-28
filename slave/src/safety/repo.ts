// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import path from "node:path";
import type { RepoConfig, SlaveConfig } from "../config.js";
import { findRepo } from "../config.js";
import type { AssignedTask } from "../gateway/types.js";

export type ResolveRepoOk = { ok: true; repo: RepoConfig };
export type ResolveRepoErr = { ok: false; code: string; message: string };
export type ResolveRepoResult = ResolveRepoOk | ResolveRepoErr;

/**
 * Resolve execution cwd from slave whitelist via repoId only.
 * Never trusts App/Gateway-supplied cwd strings.
 */
export function resolveAssignedRepo(
  cfg: SlaveConfig,
  task: AssignedTask,
): ResolveRepoResult {
  const extra = task as AssignedTask & { cwd?: unknown };
  if (extra.cwd !== undefined && extra.cwd !== null && extra.cwd !== "") {
    return {
      ok: false,
      code: "cwd_not_allowed",
      message: "task.cwd is not accepted; use repoId whitelist only",
    };
  }

  const repoId = (task.repoId ?? "").trim();
  if (!repoId) {
    return {
      ok: false,
      code: "repo_required",
      message: "repoId is required",
    };
  }

  const repo = findRepo(cfg, repoId);
  if (!repo) {
    return {
      ok: false,
      code: "repo_not_whitelisted",
      message: `repo not in whitelist: ${repoId}`,
    };
  }

  // Defense in depth: whitelist entry must remain absolute after normalize.
  if (!path.isAbsolute(repo.cwd)) {
    return {
      ok: false,
      code: "cwd_not_absolute",
      message: `whitelisted cwd is not absolute: ${repoId}`,
    };
  }

  return { ok: true, repo };
}
