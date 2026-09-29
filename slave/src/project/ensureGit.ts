// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { access } from "node:fs/promises";
import { constants } from "node:fs";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { log } from "../log.js";
import type { ProjectConfig } from "../config.js";

const execFileAsync = promisify(execFile);

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout, stderr } = await execFileAsync("git", args, {
    cwd,
    maxBuffer: 10 * 1024 * 1024,
    windowsHide: true,
  });
  const err = stderr?.toString()?.trim();
  if (err) {
    // git often writes hints to stderr; ignore unless command failed (exec throws).
  }
  return stdout.toString();
}

/** True when cwd is inside a git work tree. */
export async function isGitWorkTree(cwd: string): Promise<boolean> {
  try {
    const out = (await git(cwd, ["rev-parse", "--is-inside-work-tree"])).trim();
    return out === "true";
  } catch {
    return false;
  }
}

export async function hasHeadCommit(cwd: string): Promise<boolean> {
  try {
    const sha = (await git(cwd, ["rev-parse", "HEAD"])).trim();
    return Boolean(sha);
  } catch {
    return false;
  }
}

/**
 * Ensure project cwd is a git repo with at least one commit (needed for Diff baseline).
 * If missing: git init + local identity + initial commit of current tree.
 */
export async function ensureProjectGitRepo(
  project: ProjectConfig,
): Promise<{ ok: boolean; created: boolean; error?: string }> {
  try {
    await access(project.cwd, constants.F_OK);
  } catch {
    return { ok: false, created: false, error: `cwd missing: ${project.cwd}` };
  }

  const inTree = await isGitWorkTree(project.cwd);
  if (inTree && (await hasHeadCommit(project.cwd))) {
    return { ok: true, created: false };
  }

  try {
    if (!inTree) {
      log.info("project has no git repo; auto-init", {
        projectId: project.id,
        cwd: project.cwd,
      });
      await git(project.cwd, ["init"]);
    } else {
      log.info("project git has no HEAD; creating initial commit", {
        projectId: project.id,
        cwd: project.cwd,
      });
    }

    // Local-only identity so commit works without global git config.
    await git(project.cwd, ["config", "user.email", "cloud-agent@localhost"]);
    await git(project.cwd, ["config", "user.name", "cloud-agent"]);

    await git(project.cwd, ["add", "-A"]);
    const dirty = (await git(project.cwd, ["status", "--porcelain"])).trim();
    if (dirty) {
      await git(project.cwd, [
        "commit",
        "-m",
        "chore: initial commit (auto-init by cloud-agent slave)",
      ]);
    } else {
      await git(project.cwd, [
        "commit",
        "--allow-empty",
        "-m",
        "chore: initial commit (auto-init by cloud-agent slave)",
      ]);
    }

    log.info("project git ready", {
      projectId: project.id,
      cwd: project.cwd,
      created: true,
    });
    return { ok: true, created: true };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    log.error("project git auto-init failed", {
      projectId: project.id,
      cwd: project.cwd,
      error: message,
    });
    return { ok: false, created: false, error: message };
  }
}

/** Run ensure for all whitelist projects (Slave startup). */
export async function ensureAllProjectGitRepos(
  projects: ProjectConfig[],
): Promise<void> {
  for (const p of projects) {
    const r = await ensureProjectGitRepo(p);
    if (!r.ok) {
      log.warn("skip project git ensure", {
        projectId: p.id,
        error: r.error ?? "unknown",
      });
    }
  }
}
