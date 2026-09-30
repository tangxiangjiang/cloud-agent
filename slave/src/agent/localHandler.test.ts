// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { AgentOptions } from "@cursor/sdk";
import { assertNoCloud, resolveTaskModel } from "./localHandler.js";

describe("assertNoCloud", () => {
  it("accepts local-only options", () => {
    const opts: AgentOptions = {
      model: { id: "composer-2.5" },
      local: { cwd: "/tmp/repo", settingSources: [] },
    };
    assert.doesNotThrow(() => assertNoCloud(opts));
  });

  it("rejects cloud field", () => {
    const opts = {
      model: { id: "composer-2.5" },
      local: { cwd: "/tmp/repo" },
      cloud: { repos: [{ url: "https://example.com/r.git" }] },
    } as AgentOptions;
    assert.throws(() => assertNoCloud(opts), /cloud runtime is forbidden/);
  });

  it('rejects settingSources "all"', () => {
    const opts: AgentOptions = {
      model: { id: "composer-2.5" },
      local: { cwd: "/tmp/repo", settingSources: ["all"] },
    };
    assert.throws(() => assertNoCloud(opts), /settingSources/);
  });
});

describe("resolveTaskModel", () => {
  it("maps auto to default (Pro-safe without catalog)", () => {
    assert.equal(resolveTaskModel("auto", "composer-2.5"), "default");
    assert.equal(resolveTaskModel("default", "composer-2.5"), "default");
  });

  it("maps empty to defaultModel id when concrete", () => {
    assert.equal(resolveTaskModel("", "composer-2.5"), "composer-2.5");
  });

  it("passes through explicit ids", () => {
    assert.equal(
      resolveTaskModel("gpt-5.6-sol-medium", "default"),
      "gpt-5.6-sol-medium",
    );
  });
});
