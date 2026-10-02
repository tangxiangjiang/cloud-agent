// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  assessNodeDiff,
  isCodePath,
  isProgressPath,
  phaseKindFromNode,
} from "./diffQuality.js";

describe("diffQuality", () => {
  it("rejects empty and progress-only", () => {
    assert.equal(assessNodeDiff([]).substantive, false);
    assert.equal(
      assessNodeDiff([{ path: "ai/progress.md" }]).progressOnly,
      true,
    );
    assert.equal(
      assessNodeDiff([{ path: "ai/progress.md" }]).substantive,
      false,
    );
  });

  it("accepts code; evidence allows Record/docs without code", () => {
    assert.equal(isCodePath("stdlib/hash.ny"), true);
    assert.equal(isProgressPath("doc/progress.md"), true);
    const code = assessNodeDiff([{ path: "src/a.go" }, { path: "ai/progress.md" }]);
    assert.equal(code.substantive, true);
    assert.equal(code.hasCode, true);

    const ev = assessNodeDiff(
      [
        { path: "docs/roadmaps/m_v0.1/m_v0.1_p0_wave-a-evidence.md" },
        { path: "docs/status/current.md" },
      ],
      { designLock: false, evidence: true },
    );
    assert.equal(ev.substantive, true);
    assert.equal(ev.hasCode, false);

    const evCode = assessNodeDiff(
      [{ path: "docs/evidence.md" }, { path: "tests/harness.ny" }],
      { designLock: false, evidence: true },
    );
    assert.equal(evCode.substantive, true);
    assert.equal(evCode.hasCode, true);

    // Still refuse progress-only on evidence phases.
    const evProg = assessNodeDiff([{ path: "ai/roadmaps/progress.md" }], {
      designLock: false,
      evidence: true,
    });
    assert.equal(evProg.substantive, false);
  });

  it("design-lock allows docs-only", () => {
    const k = phaseKindFromNode({
      id: "M14-P00",
      title: "Design lock",
      phaseRef: "docs/M14-P00-design-lock.md",
      dependsOn: [],
      status: "ready",
    });
    assert.equal(k.designLock, true);
    const a = assessNodeDiff([{ path: "docs/M14-P00-design-lock.md" }], k);
    assert.equal(a.substantive, true);
  });

  it("detects evidence phase ids", () => {
    const k = phaseKindFromNode({
      id: "M14-P03",
      title: "Evidence close",
      phaseRef: "docs/M14-P03-evidence-close.md",
      dependsOn: [],
      status: "ready",
    });
    assert.equal(k.evidence, true);

    const k2 = phaseKindFromNode({
      id: "m_v0.1_p0",
      title: "m_v0.1_p0 — Wave A face evidence",
      phaseRef: "docs/roadmaps/m_v0.1/m_v0.1_p0_wave-a-evidence.md",
      dependsOn: [],
      status: "ready",
    });
    assert.equal(k2.evidence, true);
  });
});
