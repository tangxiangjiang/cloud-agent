// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  parseSyncAiJson,
  sanitizeSyncSummary,
} from "./syncSummaryAi.js";

describe("parseSyncAiJson", () => {
  it("parses JSON summary and inferredPhaseStatus", () => {
    const r = parseSyncAiJson(`{
  "summary": "Branch feat-a is clean; M01-P01 approved.",
  "inferredPhaseStatus": [
    { "id": "M01-P01", "status": "approved", "note": "checkbox" }
  ]
}`);
    assert.ok(r);
    assert.match(r!.summary, /feat-a/);
    assert.equal(r!.inferredPhaseStatus?.[0]?.id, "M01-P01");
  });

  it("accepts plain text and fenced JSON", () => {
    const plain = parseSyncAiJson("Short human summary about progress.");
    assert.equal(plain?.summary.startsWith("Short"), true);

    const fenced = parseSyncAiJson(
      "```json\n{\"summary\":\"From fence\"}\n```",
    );
    assert.equal(fenced?.summary, "From fence");
  });
});

describe("sanitizeSyncSummary", () => {
  it("falls back when empty", () => {
    assert.equal(sanitizeSyncSummary("  ", "rule"), "rule");
  });
});
