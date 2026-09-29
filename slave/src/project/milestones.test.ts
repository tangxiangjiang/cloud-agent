// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import { loadProjectMilestones, parseMilestones } from "./milestones.js";

describe("parseMilestones", () => {
  it("parses milestone with phases", () => {
    const ms = parseMilestones({
      schemaVersion: 1,
      milestones: [
        {
          id: "M01",
          title: "One",
          progressDoc: "ai/progress.md",
          phases: [
            {
              id: "M01-P01",
              title: "First",
              phaseRef: "doc/a.md",
              dependsOn: [],
            },
          ],
        },
      ],
    });
    assert.equal(ms.length, 1);
    assert.equal(ms[0]?.id, "M01");
    assert.equal(ms[0]?.phases[0]?.prompt?.mode, "phase_file");
  });
});

describe("loadProjectMilestones", () => {
  it("loads relative index under cwd", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "ms-idx-"));
    writeFileSync(
      path.join(dir, "idx.json"),
      JSON.stringify({
        schemaVersion: 1,
        milestones: [
          {
            id: "M1",
            title: "T",
            phases: [{ id: "M1-P1", title: "p", phaseRef: "x.md", dependsOn: [] }],
          },
        ],
      }),
      "utf8",
    );
    const r = loadProjectMilestones(dir, "idx.json");
    assert.equal(r.milestones.length, 1);
    assert.equal(r.error, undefined);
  });

  it("rejects path escape", () => {
    const r = loadProjectMilestones("/tmp/proj", "../outside.json");
    assert.equal(r.milestones.length, 0);
    assert.match(r.error ?? "", /\.\.|escape|relative/i);
  });
});
