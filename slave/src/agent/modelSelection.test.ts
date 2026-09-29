// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  isAutoModelId,
  parseOptimizeFor,
  resolveModelSelection,
} from "./modelSelection.js";

describe("isAutoModelId", () => {
  it("recognizes auto aliases", () => {
    assert.equal(isAutoModelId("auto"), true);
    assert.equal(isAutoModelId("Auto"), true);
    assert.equal(isAutoModelId("auto-smart"), true);
    assert.equal(isAutoModelId("composer-2.5"), false);
  });
});

describe("parseOptimizeFor", () => {
  it("defaults to balanced", () => {
    assert.equal(parseOptimizeFor(undefined), "balanced");
    assert.equal(parseOptimizeFor(null), "balanced");
    assert.equal(parseOptimizeFor(""), "balanced");
  });

  it("accepts known modes", () => {
    assert.equal(parseOptimizeFor("cost"), "cost");
    assert.equal(parseOptimizeFor("Balanced"), "balanced");
    assert.equal(parseOptimizeFor("intelligence"), "intelligence");
  });

  it("rejects unknown", () => {
    assert.throws(() => parseOptimizeFor("default"), /optimizeFor/);
  });
});

describe("resolveModelSelection", () => {
  it("maps auto to Cursor Router auto-smart", () => {
    assert.deepEqual(resolveModelSelection("auto", "composer-2.5", "balanced"), {
      id: "auto-smart",
      params: [{ id: "optimize_for", value: "balanced" }],
    });
  });

  it("maps empty to defaultModel (router when default is auto)", () => {
    assert.deepEqual(resolveModelSelection("", "auto-smart", "cost"), {
      id: "auto-smart",
      params: [{ id: "optimize_for", value: "cost" }],
    });
    assert.deepEqual(resolveModelSelection(null, "composer-2.5", "balanced"), {
      id: "composer-2.5",
    });
  });

  it("passes through explicit model ids", () => {
    assert.deepEqual(
      resolveModelSelection("gpt-5.6-sol-medium", "auto-smart", "balanced"),
      { id: "gpt-5.6-sol-medium" },
    );
  });
});
