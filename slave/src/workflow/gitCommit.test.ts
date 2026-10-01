// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import {
  commitOnApprove,
  fallbackCommitMessage,
  sanitizeCommitMessage,
} from "./gitCommit.js";
import type { WorkflowNode } from "./types.js";

function gitInit(dir: string): void {
  execFileSync("git", ["init"], { cwd: dir, windowsHide: true });
  execFileSync("git", ["config", "user.email", "t@test"], {
    cwd: dir,
    windowsHide: true,
  });
  execFileSync("git", ["config", "user.name", "t"], {
    cwd: dir,
    windowsHide: true,
  });
}

const node: WorkflowNode = {
  id: "M01-P01",
  title: "Confirm Flutter project",
  dependsOn: [],
  status: "approved",
};

describe("gitCommit helpers", () => {
  it("builds fallback conventional-ish message", () => {
    const msg = fallbackCommitMessage({ node, comment: "lgtm" });
    assert.match(msg, /^approve\(M01-P01\): Confirm Flutter project/);
    assert.match(msg, /Review comment: lgtm/);
  });

  it("sanitizes fenced AI output", () => {
    const out = sanitizeCommitMessage(
      "```\nfeat(M01-P01): add shell\n\nbody\n```",
      "fallback",
    );
    assert.equal(out, "feat(M01-P01): add shell\n\nbody");
  });

  it("strips Chinese agent narration before conventional subject", () => {
    const out = sanitizeCommitMessage(
      "查看变更内容以起草准确的提交信息。approve(M12-P03): Kernel allocators",
      "fallback",
    );
    assert.equal(out, "approve(M12-P03): Kernel allocators");
  });

  it("strips multi-sentence CN preamble and keeps last conventional line", () => {
    const out = sanitizeCommitMessage(
      "先看一下具体 diff，再按常规提交格式写出提交说明。approve(M12-P01): Platform ABI + heap + link\n\nDetails.",
      "fallback",
    );
    assert.equal(
      out,
      "approve(M12-P01): Platform ABI + heap + link\n\nDetails.",
    );
  });

  it("falls back when AI output is pure narration", () => {
    const out = sanitizeCommitMessage(
      "查看变更内容以便写准确的提交说明。工作区状态与节点描述不一致。",
      "approve(M12-P02): fallback title",
    );
    assert.equal(out, "approve(M12-P02): fallback title");
  });
});

describe("commitOnApprove", () => {
  it("creates a local commit from working tree with AI message", async () => {
    const dir = mkdtempSync(path.join(tmpdir(), "ca-commit2-"));
    gitInit(dir);
    writeFileSync(path.join(dir, "a.txt"), "1\n", "utf8");
    execFileSync("git", ["add", "a.txt"], { cwd: dir, windowsHide: true });
    execFileSync("git", ["commit", "-m", "init"], {
      cwd: dir,
      windowsHide: true,
    });
    writeFileSync(path.join(dir, "a.txt"), "2\n", "utf8");

    const result = await commitOnApprove({
      repoCwd: dir,
      node,
      comment: "ok",
      generateMessage: async () => "feat(M01-P01): bump a.txt\n\nfrom AI",
    });
    assert.equal(result.ok, true);
    if (!result.ok || result.skipped) {
      assert.fail(JSON.stringify(result));
    }
    assert.equal(result.ai, true);
    assert.match(result.message, /feat\(M01-P01\): bump a\.txt/);
    assert.match(result.sha, /^[0-9a-f]{7,40}$/i);

    const log = execFileSync("git", ["log", "-1", "--pretty=%s"], {
      cwd: dir,
      encoding: "utf8",
      windowsHide: true,
    }).trim();
    assert.equal(log, "feat(M01-P01): bump a.txt");
  });

  it("skips when clean", async () => {
    const dir = mkdtempSync(path.join(tmpdir(), "ca-commit3-"));
    gitInit(dir);
    writeFileSync(path.join(dir, "a.txt"), "1\n", "utf8");
    execFileSync("git", ["add", "a.txt"], { cwd: dir, windowsHide: true });
    execFileSync("git", ["commit", "-m", "init"], {
      cwd: dir,
      windowsHide: true,
    });

    const result = await commitOnApprove({
      repoCwd: dir,
      node,
      generateMessage: null,
    });
    assert.equal(result.ok, true);
    if (!result.ok) assert.fail();
    assert.equal(result.skipped, true);
  });

  it("uses fallback when AI returns null", async () => {
    const dir = mkdtempSync(path.join(tmpdir(), "ca-commit4-"));
    gitInit(dir);
    writeFileSync(path.join(dir, "a.txt"), "1\n", "utf8");
    execFileSync("git", ["add", "a.txt"], { cwd: dir, windowsHide: true });
    execFileSync("git", ["commit", "-m", "init"], {
      cwd: dir,
      windowsHide: true,
    });
    writeFileSync(path.join(dir, "a.txt"), "2\n", "utf8");

    const result = await commitOnApprove({
      repoCwd: dir,
      node,
      generateMessage: async () => null,
    });
    assert.equal(result.ok, true);
    if (!result.ok || result.skipped) assert.fail(JSON.stringify(result));
    assert.equal(result.ai, false);
    assert.match(result.message, /^approve\(M01-P01\):/);
  });
});
