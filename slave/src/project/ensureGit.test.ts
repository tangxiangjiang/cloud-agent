// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import {
  ensureProjectGitRepo,
  hasHeadCommit,
  isGitWorkTree,
} from "./ensureGit.js";

describe("ensureProjectGitRepo", () => {
  it("no-ops when repo already has HEAD", async () => {
    const cwd = mkdtempSync(path.join(tmpdir(), "ca-git-ok-"));
    execFileSync("git", ["init"], { cwd, windowsHide: true });
    execFileSync("git", ["config", "user.email", "t@example.com"], {
      cwd,
      windowsHide: true,
    });
    execFileSync("git", ["config", "user.name", "t"], { cwd, windowsHide: true });
    writeFileSync(path.join(cwd, "a.txt"), "1\n");
    execFileSync("git", ["add", "a.txt"], { cwd, windowsHide: true });
    execFileSync("git", ["commit", "-m", "init"], { cwd, windowsHide: true });

    const r = await ensureProjectGitRepo({
      id: "r1",
      name: "t",
      cwd,
    });
    assert.equal(r.ok, true);
    assert.equal(r.created, false);
  });

  it("inits and commits when no git", async () => {
    const cwd = mkdtempSync(path.join(tmpdir(), "ca-git-new-"));
    writeFileSync(path.join(cwd, "README.md"), "# demo\n");

    assert.equal(await isGitWorkTree(cwd), false);

    const r = await ensureProjectGitRepo({
      id: "r_flutter",
      name: "flutter-chat-local",
      cwd,
    });
    assert.equal(r.ok, true);
    assert.equal(r.created, true);
    assert.equal(await isGitWorkTree(cwd), true);
    assert.equal(await hasHeadCommit(cwd), true);
  });
});
