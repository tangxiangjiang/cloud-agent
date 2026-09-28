// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { attemptRunCancel, shouldRelayStreamEvent } from "./cancel.js";

describe("cancel helpers", () => {
  it("attempts cancel when supported", async () => {
    let called = false;
    const r = await attemptRunCancel({
      id: "run1",
      supports: () => true,
      cancel: async () => {
        called = true;
      },
    });
    assert.deepEqual(r, { supported: true });
    assert.equal(called, true);
  });

  it("records reason when unsupported", async () => {
    const r = await attemptRunCancel({
      id: "run1",
      supports: () => false,
      unsupportedReason: () => "detached handle",
      cancel: async () => {
        throw new Error("should not call");
      },
    });
    assert.equal(r.supported, false);
    if (!r.supported) assert.equal(r.reason, "detached handle");
  });

  it("stops stream relay after cancel requested", () => {
    assert.equal(shouldRelayStreamEvent(false), true);
    assert.equal(shouldRelayStreamEvent(true), false);
  });
});
