// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { sleptGapMs, startSleepWakeWatch } from "./sleepWake.js";

describe("sleptGapMs", () => {
  it("ignores normal ticks", () => {
    assert.equal(sleptGapMs(1_000, 3_000, 20_000), null);
  });

  it("detects a sleep-sized jump", () => {
    assert.equal(sleptGapMs(1_000, 121_000, 20_000), 120_000);
  });
});

describe("startSleepWakeWatch", () => {
  it("fires onWake when injected clock jumps", () => {
    const wakes: number[] = [];
    let now = 0;
    let tick: (() => void) | null = null;
    const stop = startSleepWakeWatch((ms) => wakes.push(ms), {
      tickMs: 10,
      jumpMs: 50,
      now: () => now,
      setIntervalFn: (fn) => {
        tick = fn;
        return 1 as unknown as ReturnType<typeof setInterval>;
      },
      clearIntervalFn: () => {
        tick = null;
      },
    });
    now = 10;
    tick?.();
    assert.deepEqual(wakes, []);
    now = 10_000;
    tick?.();
    assert.deepEqual(wakes, [9_990]);
    stop();
  });
});
