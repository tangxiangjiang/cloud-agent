// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  caffeinateArgs,
  holdMacAwake,
  shouldHoldMacAwake,
} from "./macAwake.js";

describe("shouldHoldMacAwake", () => {
  it("only on darwin unless disabled", () => {
    assert.equal(shouldHoldMacAwake("linux", {}), false);
    assert.equal(shouldHoldMacAwake("darwin", {}), true);
    assert.equal(shouldHoldMacAwake("darwin", { MAC_PREVENT_SLEEP: "0" }), false);
  });
});

describe("caffeinateArgs", () => {
  it("waits on this pid with idle+system assertions", () => {
    assert.deepEqual(caffeinateArgs(42), ["-i", "-s", "-w", "42"]);
  });
});

describe("holdMacAwake", () => {
  it("spawns caffeinate on darwin", () => {
    const spawned: unknown[] = [];
    const child = {
      unref() {
        spawned.push("unref");
      },
    };
    const out = holdMacAwake({
      platform: "darwin",
      env: {},
      pid: 9,
      spawnFn: ((cmd, args, opts) => {
        spawned.push({ cmd, args, opts });
        return child;
      }) as typeof import("node:child_process").spawn,
    });
    assert.equal(out, child);
    assert.equal((spawned[0] as { cmd: string }).cmd, "caffeinate");
    assert.deepEqual((spawned[0] as { args: string[] }).args, [
      "-i",
      "-s",
      "-w",
      "9",
    ]);
  });
});
