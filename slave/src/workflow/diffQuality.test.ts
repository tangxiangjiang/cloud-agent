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

  it("accepts code; evidence requires code", () => {
    assert.equal(isCodePath("stdlib/hash.ny"), true);
    assert.equal(isProgressPath("doc/progress.md"), true);
    const code = assessNodeDiff([{ path: "src/a.go" }, { path: "ai/progress.md" }]);
    assert.equal(code.substantive, true);
    assert.equal(code.hasCode, true);

    const ev = assessNodeDiff([{ path: "docs/evidence.md" }], {
      designLock: false,
      evidence: true,
    });
    assert.equal(ev.substantive, false);

    const evCode = assessNodeDiff(
      [{ path: "docs/evidence.md" }, { path: "tests/harness.ny" }],
      { designLock: false, evidence: true },
    );
    assert.equal(evCode.substantive, true);
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
  });
});
