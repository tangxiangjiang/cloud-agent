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

  it("forces serial dependsOn by phase order", () => {
    const ms = parseMilestones({
      schemaVersion: 1,
      milestones: [
        {
          id: "M05",
          title: "Five",
          phases: [
            { id: "M05-P00", title: "a", phaseRef: "a.md", dependsOn: [] },
            { id: "M05-P01", title: "b", phaseRef: "b.md", dependsOn: [] },
            {
              id: "M05-P02",
              title: "c",
              phaseRef: "c.md",
              dependsOn: ["M05-P01"],
            },
            {
              id: "M05-P03",
              title: "d",
              phaseRef: "d.md",
              dependsOn: ["M05-P01"],
            },
          ],
        },
      ],
    });
    const phases = ms[0]!.phases;
    assert.deepEqual(phases[0]!.dependsOn, []);
    assert.deepEqual(phases[1]!.dependsOn, ["M05-P00"]);
    assert.deepEqual(phases[2]!.dependsOn, ["M05-P01"]);
    assert.deepEqual(phases[3]!.dependsOn, ["M05-P02"]);
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
