// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { SDKMessage } from "@cursor/sdk";
import { mapSdkMessage } from "./mapStream.js";

describe("mapSdkMessage", () => {
  it("maps assistant text to assistant.delta", () => {
    const out: Array<{ kind: string; payload: Record<string, unknown> }> = [];
    const event = {
      type: "assistant",
      agent_id: "a",
      run_id: "r",
      message: {
        role: "assistant",
        content: [{ type: "text", text: "hello" }],
      },
    } as SDKMessage;
    mapSdkMessage(event, (kind, payload) => out.push({ kind, payload }));
    assert.deepEqual(out, [{ kind: "assistant.delta", payload: { text: "hello" } }]);
  });

  it("maps tool_call running/completed", () => {
    const out: Array<{ kind: string; payload: Record<string, unknown> }> = [];
    mapSdkMessage(
      {
        type: "tool_call",
        agent_id: "a",
        run_id: "r",
        call_id: "c1",
        name: "read",
        status: "running",
        args: { path: "a.ts" },
      } as SDKMessage,
      (kind, payload) => out.push({ kind, payload }),
    );
    mapSdkMessage(
      {
        type: "tool_call",
        agent_id: "a",
        run_id: "r",
        call_id: "c1",
        name: "read",
        status: "completed",
        result: { ok: true },
      } as SDKMessage,
      (kind, payload) => out.push({ kind, payload }),
    );
    assert.equal(out[0]?.kind, "tool.started");
    assert.equal(out[0]?.payload.name, "read");
    assert.equal(out[1]?.kind, "tool.finished");
    assert.equal(out[1]?.payload.ok, true);
  });

  it("maps RUNNING status", () => {
    const out: Array<{ kind: string }> = [];
    mapSdkMessage(
      {
        type: "status",
        agent_id: "a",
        run_id: "r",
        status: "RUNNING",
      } as SDKMessage,
      (kind) => out.push({ kind }),
    );
    assert.deepEqual(out, [{ kind: "status" }]);
  });
});
