// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  HangTimeoutWatch,
  hangTimeoutMessage,
  type HangTimerHandle,
  type HangTimeoutReason,
} from "./hangTimeout.js";

type FakeTimer = { due: number; fn: () => void };

function createFakeClock() {
  let now = 0;
  let nextId = 1;
  const timers = new Map<number, FakeTimer>();

  const clock = {
    now: () => now,
    setTimeout: (fn: () => void, ms: number): HangTimerHandle => {
      const id = nextId++;
      timers.set(id, { due: now + ms, fn });
      return id as unknown as HangTimerHandle;
    },
    clearTimeout: (id: HangTimerHandle) => {
      timers.delete(id as unknown as number);
    },
    advance(ms: number) {
      now += ms;
      // Fire due timers in due-order; callbacks may schedule more.
      for (;;) {
        let next: { id: number; t: FakeTimer } | null = null;
        for (const [id, t] of timers) {
          if (t.due <= now && (!next || t.due < next.t.due)) {
            next = { id, t };
          }
        }
        if (!next) break;
        timers.delete(next.id);
        next.t.fn();
      }
    },
  };
  return clock;
}

describe("HangTimeoutWatch", () => {
  it("fires task_timeout after wall-clock TTL", () => {
    const clock = createFakeClock();
    const reasons: HangTimeoutReason[] = [];
    const hang = new HangTimeoutWatch({
      taskTimeoutMs: 1000,
      idleTimeoutMs: 0,
      onTimeout: (r) => reasons.push(r),
      clock,
    });
    hang.start();
    clock.advance(999);
    assert.deepEqual(reasons, []);
    clock.advance(1);
    assert.deepEqual(reasons, ["task_timeout"]);
  });

  it("fires idle_timeout when untouched", () => {
    const clock = createFakeClock();
    const reasons: HangTimeoutReason[] = [];
    const hang = new HangTimeoutWatch({
      taskTimeoutMs: 0,
      idleTimeoutMs: 500,
      onTimeout: (r) => reasons.push(r),
      clock,
    });
    hang.start();
    clock.advance(499);
    assert.deepEqual(reasons, []);
    clock.advance(1);
    assert.deepEqual(reasons, ["idle_timeout"]);
  });

  it("touch resets idle timer", () => {
    const clock = createFakeClock();
    const reasons: HangTimeoutReason[] = [];
    const hang = new HangTimeoutWatch({
      taskTimeoutMs: 0,
      idleTimeoutMs: 500,
      onTimeout: (r) => reasons.push(r),
      clock,
    });
    hang.start();
    clock.advance(400);
    hang.touch();
    clock.advance(400);
    assert.deepEqual(reasons, []);
    clock.advance(100);
    assert.deepEqual(reasons, ["idle_timeout"]);
  });

  it("does not fire after stop", () => {
    const clock = createFakeClock();
    const reasons: HangTimeoutReason[] = [];
    const hang = new HangTimeoutWatch({
      taskTimeoutMs: 100,
      idleTimeoutMs: 100,
      onTimeout: (r) => reasons.push(r),
      clock,
    });
    hang.start();
    hang.stop();
    clock.advance(1000);
    assert.deepEqual(reasons, []);
  });

  it("both 0 disables all timers", () => {
    const clock = createFakeClock();
    const reasons: HangTimeoutReason[] = [];
    const hang = new HangTimeoutWatch({
      taskTimeoutMs: 0,
      idleTimeoutMs: 0,
      onTimeout: (r) => reasons.push(r),
      clock,
    });
    hang.start();
    hang.touch();
    clock.advance(60_000);
    assert.deepEqual(reasons, []);
  });

  it("fires only once when TTL and idle would both qualify", () => {
    const clock = createFakeClock();
    const reasons: HangTimeoutReason[] = [];
    const hang = new HangTimeoutWatch({
      taskTimeoutMs: 100,
      idleTimeoutMs: 100,
      onTimeout: (r) => reasons.push(r),
      clock,
    });
    hang.start();
    clock.advance(100);
    assert.equal(reasons.length, 1);
    assert.ok(reasons[0] === "task_timeout" || reasons[0] === "idle_timeout");
  });
});

describe("hangTimeoutMessage", () => {
  it("includes configured durations", () => {
    assert.match(
      hangTimeoutMessage("task_timeout", 3_600_000, 600_000),
      /3600000/,
    );
    assert.match(
      hangTimeoutMessage("idle_timeout", 3_600_000, 600_000),
      /600000/,
    );
  });
});
