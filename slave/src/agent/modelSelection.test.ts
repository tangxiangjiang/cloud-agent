// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  isAutoModelId,
  parseOptimizeFor,
  pickAutoTarget,
  resolveModelSelection,
} from "./modelSelection.js";

describe("isAutoModelId", () => {
  it("recognizes auto aliases including default", () => {
    assert.equal(isAutoModelId("auto"), true);
    assert.equal(isAutoModelId("Auto"), true);
    assert.equal(isAutoModelId("auto-smart"), true);
    assert.equal(isAutoModelId("default"), true);
    assert.equal(isAutoModelId("composer-2.5"), false);
  });
});

describe("parseOptimizeFor", () => {
  it("defaults to cost", () => {
    assert.equal(parseOptimizeFor(undefined), "cost");
    assert.equal(parseOptimizeFor(null), "cost");
    assert.equal(parseOptimizeFor(""), "cost");
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

describe("pickAutoTarget", () => {
  it("uses default when Pro catalog only has default", () => {
    assert.deepEqual(pickAutoTarget("auto-smart", ["default"]), {
      id: "default",
      useOptimizeFor: false,
    });
    assert.deepEqual(pickAutoTarget("default", ["default"]), {
      id: "default",
      useOptimizeFor: false,
    });
  });

  it("uses auto-smart when present in catalog", () => {
    assert.deepEqual(
      pickAutoTarget("auto-smart", ["default", "auto-smart", "composer-2.5"]),
      { id: "auto-smart", useOptimizeFor: true },
    );
  });

  it("falls back Pro-safe without catalog", () => {
    assert.deepEqual(pickAutoTarget("default"), {
      id: "default",
      useOptimizeFor: false,
    });
  });
});

describe("resolveModelSelection", () => {
  it("maps App auto to default on Pro (catalog)", () => {
    assert.deepEqual(
      resolveModelSelection("auto", "default", {
        optimizeFor: "cost",
        autoModelId: "default",
        availableModelIds: ["default"],
      }),
      { id: "default" },
    );
  });

  it("maps auto to Cursor Router when auto-smart available", () => {
    assert.deepEqual(
      resolveModelSelection("auto", "default", {
        optimizeFor: "cost",
        autoModelId: "auto-smart",
        availableModelIds: ["auto-smart", "default"],
      }),
      {
        id: "auto-smart",
        params: [{ id: "optimize_for", value: "cost" }],
      },
    );
  });

  it("maps empty to defaultModel id", () => {
    assert.deepEqual(resolveModelSelection("", "composer-2.5", "cost"), {
      id: "composer-2.5",
    });
    assert.deepEqual(
      resolveModelSelection(null, "default", {
        autoModelId: "default",
        availableModelIds: ["default"],
      }),
      { id: "default" },
    );
  });

  it("passes through explicit model ids", () => {
    assert.deepEqual(
      resolveModelSelection("gpt-5.6-sol-medium", "default", {
        availableModelIds: ["default"],
      }),
      { id: "gpt-5.6-sol-medium" },
    );
  });
});
