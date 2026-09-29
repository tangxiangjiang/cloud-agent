// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import type { SlaveConfig } from "../config.js";
import { parseProgressPhases } from "./parseProgress.js";
import { collectProjectSync } from "./syncCollect.js";

describe("parseProgressPhases", () => {
  it("reads checkboxes and table rows", () => {
    const md = `# Progress

- [x] M01-P01 @approved 2026-09-28T12:00:00.000Z
- [x] M01-P02
- [ ] M01-P03

| Bundle | 状态 | 备注 |
| M02-P01 | pending | x |
| M02-P02 | approved | y |
`;
    const rows = parseProgressPhases(md);
    const byId = Object.fromEntries(rows.map((r) => [r.id, r.progressStatus]));
    assert.equal(byId["M01-P01"], "approved");
    assert.equal(byId["M01-P02"], "done");
    assert.equal(byId["M01-P03"], "pending");
    assert.equal(byId["M02-P01"], "pending");
    assert.equal(byId["M02-P02"], "approved");
  });
});

describe("collectProjectSync", () => {
  function cfgFor(cwd: string, id = "r_test"): SlaveConfig {
    return {
      gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
      slaveId: "slave_test",
      projects: [{ id, name: "t", cwd, index: "ai/milestones.json" }],
      repos: [{ id, name: "t", cwd, index: "ai/milestones.json" }],
      apiKeyEnv: "CURSOR_API_KEY",
      tokenEnv: "GATEWAY_TOKEN",
      defaultModel: "composer-2.5",
    };
  }

  it("degrades without git and without progress file", async () => {
    const root = mkdtempSync(path.join(tmpdir(), "ca-sync-"));
    mkdirSync(path.join(root, "ai"), { recursive: true });
    writeFileSync(
      path.join(root, "ai", "milestones.json"),
      JSON.stringify({
        schemaVersion: 1,
        milestones: [
          {
            id: "M01",
            title: "Demo",
            progressDoc: "ai/progress.md",
            phases: [
              {
                id: "M01-P01",
                title: "First",
                phaseRef: "doc/p.md",
                dependsOn: [],
              },
            ],
          },
        ],
      }),
      "utf8",
    );
    // no git init, no progress.md

    const payload = await collectProjectSync({
      cfg: cfgFor(root),
      slaveId: "slave_test",
      requestId: "req_1",
      repoId: "r_test",
      now: () => new Date("2026-09-29T00:00:00.000Z"),
    });

    assert.equal(payload.schemaVersion, 1);
    assert.equal(payload.requestId, "req_1");
    assert.equal(payload.repoId, "r_test");
    assert.equal(payload.cwd, root);
    assert.equal(payload.branch, null);
    assert.equal(payload.head, null);
    assert.ok(payload.error);
    assert.match(payload.summary, /git/i);
    assert.ok(payload.milestonesIndex?.milestones.length === 1);
    assert.equal(payload.phases[0]?.id, "M01-P01");
    assert.equal(payload.phases[0]?.progressStatus, "pending");
  });

  it("reads progress when present and reports unknown repo", async () => {
    const root = mkdtempSync(path.join(tmpdir(), "ca-sync2-"));
    mkdirSync(path.join(root, "ai"), { recursive: true });
    writeFileSync(
      path.join(root, "ai", "milestones.json"),
      JSON.stringify({
        schemaVersion: 1,
        milestones: [
          {
            id: "M01",
            title: "Demo",
            progressDoc: "ai/progress.md",
            phases: [
              {
                id: "M01-P01",
                title: "First",
                phaseRef: "doc/p.md",
                dependsOn: [],
              },
            ],
          },
        ],
      }),
      "utf8",
    );
    writeFileSync(
      path.join(root, "ai", "progress.md"),
      "# P\n\n- [x] M01-P01 @approved 2026-09-29T00:00:00.000Z\n",
      "utf8",
    );

    const ok = await collectProjectSync({
      cfg: cfgFor(root),
      slaveId: "slave_test",
      requestId: "req_2",
      repoId: "r_test",
    });
    assert.equal(ok.progressDoc, "ai/progress.md");
    assert.equal(ok.phases.find((p) => p.id === "M01-P01")?.progressStatus, "approved");
    assert.match(ok.summary, /approved/);

    const bad = await collectProjectSync({
      cfg: cfgFor(root),
      slaveId: "slave_test",
      requestId: "req_3",
      repoId: "r_missing",
    });
    assert.ok(bad.error);
    assert.match(bad.summary, /whitelist/i);
  });
});
