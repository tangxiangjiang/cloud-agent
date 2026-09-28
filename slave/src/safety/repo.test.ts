// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import path from "node:path";
import { describe, it } from "node:test";
import { validateConfig } from "../config.js";
import { resolveAssignedRepo } from "./repo.js";

const abs = path.resolve("/tmp/safe-repo");

const cfg = validateConfig({
  gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
  slaveId: "s1",
  repos: [{ id: "r1", name: "safe", cwd: abs }],
});

describe("resolveAssignedRepo", () => {
  it("resolves whitelist repoId", () => {
    const r = resolveAssignedRepo(cfg, { id: "t1", repoId: "r1", prompt: "hi" });
    assert.equal(r.ok, true);
    if (r.ok) assert.equal(r.repo.cwd, path.normalize(abs));
  });

  it("rejects unknown repoId", () => {
    const r = resolveAssignedRepo(cfg, { id: "t1", repoId: "nope", prompt: "hi" });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.code, "repo_not_whitelisted");
  });

  it("rejects task.cwd even when repoId is valid", () => {
    const r = resolveAssignedRepo(cfg, {
      id: "t1",
      repoId: "r1",
      prompt: "hi",
      cwd: "/evil/path",
    } as { id: string; repoId: string; prompt: string; cwd: string });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.code, "cwd_not_allowed");
  });

  it("rejects missing repoId", () => {
    const r = resolveAssignedRepo(cfg, { id: "t1", prompt: "hi" });
    assert.equal(r.ok, false);
    if (!r.ok) assert.equal(r.code, "repo_required");
  });
});
