// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { applyChatModePrefix, normalizeChatMode } from "./chatMode.js";

describe("chatMode", () => {
  it("normalizes modes", () => {
    assert.equal(normalizeChatMode("ASK"), "ask");
    assert.equal(normalizeChatMode("plan"), "plan");
    assert.equal(normalizeChatMode(undefined), "agent");
  });

  it("prefixes ask and plan", () => {
    const ask = applyChatModePrefix("explain main", "ask");
    assert.ok(ask.includes("[Mode: Ask"));
    assert.ok(ask.includes("explain main"));

    const plan = applyChatModePrefix("ship M09", "plan");
    assert.ok(plan.includes("[Mode: Plan"));
    assert.ok(plan.includes("ship M09"));

    assert.equal(applyChatModePrefix("fix bug", "agent"), "fix bug");
  });

  it("does not double-prefix when Gateway already injected", () => {
    const fromGw =
      "[Mode: Ask — READ-ONLY. Do not write, edit, delete, or create files; do not run mutating git commands; answer from inspection only.]\n\nexplain";
    assert.equal(applyChatModePrefix(fromGw, "ask"), fromGw.trim());
  });
});
