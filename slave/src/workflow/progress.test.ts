// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import {
  markPhaseApproved,
  phaseKeyFromNode,
  resolveProgressPath,
  writeProgressOnApprove,
} from "./progress.js";

describe("resolveProgressPath", () => {
  const cwd = path.resolve("/tmp/repo-root");

  it("accepts relative progress.md under cwd", () => {
    const r = resolveProgressPath(cwd, "ai/progress.md");
    assert.equal(r.ok, true);
    if (r.ok) assert.equal(r.relPath, "ai/progress.md");
  });

  it("rejects escape and absolute and wrong basename", () => {
    assert.equal(resolveProgressPath(cwd, "../outside/progress.md").ok, false);
    assert.equal(resolveProgressPath(cwd, "/etc/progress.md").ok, false);
    assert.equal(resolveProgressPath(cwd, "doc/README.md").ok, false);
    assert.equal(resolveProgressPath(cwd, "").ok, false);
  });
});

describe("phaseKeyFromNode", () => {
  it("reads from id or phaseRef filename", () => {
    assert.equal(
      phaseKeyFromNode({ id: "M05-P05", dependsOn: [], status: "approved" }),
      "M05-P05",
    );
    assert.equal(
      phaseKeyFromNode({
        id: "N1",
        dependsOn: [],
        status: "approved",
        phaseRef: "doc/roadmaps/cloud-agent/phases/M05-P05-approve-progress.md",
      }),
      "M05-P05",
    );
  });
});

describe("markPhaseApproved", () => {
  it("checks the checkbox with @approved stamp", () => {
    const src = `# Progress\n\n- [ ] M05-P05\n- [ ] M06-P01\n`;
    const { next, changed } = markPhaseApproved(
      src,
      "M05-P05",
      "2026-09-28T12:00:00.000Z",
    );
    assert.equal(changed, true);
    assert.match(next, /- \[x\] M05-P05 @approved 2026-09-28T12:00:00\.000Z/);
    assert.match(next, /- \[ \] M06-P01/);
  });

  it("updates table status cell without rewriting other columns freely", () => {
    const src = `| Bundle / Node | 状态 | 备注 |\n| M05-P05 | pending | x |\n`;
    const { next } = markPhaseApproved(src, "M05-P05", "2026-09-28T12:00:00.000Z");
    assert.match(next, /\| M05-P05 \| approved \|/);
  });
});

describe("writeProgressOnApprove", () => {
  it("writes only progress.md under whitelist cwd", () => {
    const root = mkdtempSync(path.join(tmpdir(), "ca-prog-"));
    mkdirSync(path.join(root, "ai"), { recursive: true });
    writeFileSync(
      path.join(root, "ai", "progress.md"),
      "# P\n\n- [ ] M05-P05\n- [ ] M06-P01\n",
      "utf8",
    );
    writeFileSync(path.join(root, "README.md"), "do not touch\n", "utf8");

    const result = writeProgressOnApprove({
      repoCwd: root,
      progressDoc: "ai/progress.md",
      node: {
        id: "N1",
        dependsOn: [],
        status: "approved",
        phaseRef: "phases/M05-P05-approve-progress.md",
      },
      approvedAt: "2026-09-28T12:00:00.000Z",
    });
    assert.equal(result.written, true);
    assert.equal(result.phaseKey, "M05-P05");
    const text = readFileSync(path.join(root, "ai", "progress.md"), "utf8");
    assert.match(text, /- \[x\] M05-P05 @approved 2026-09-28T12:00:00\.000Z/);
    assert.equal(readFileSync(path.join(root, "README.md"), "utf8"), "do not touch\n");
  });
});
