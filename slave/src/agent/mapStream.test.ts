// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { SDKMessage } from "@cursor/sdk";
import { mapInteractionDelta, mapSdkMessage } from "./mapStream.js";

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

  it("skips assistant text when skipAssistantText", () => {
    const out: Array<{ kind: string }> = [];
    mapSdkMessage(
      {
        type: "assistant",
        agent_id: "a",
        run_id: "r",
        message: {
          role: "assistant",
          content: [{ type: "text", text: "hello" }],
        },
      } as SDKMessage,
      (kind) => out.push({ kind }),
      { skipAssistantText: true },
    );
    assert.deepEqual(out, []);
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

describe("mapInteractionDelta", () => {
  it("maps text-delta chunks to assistant.delta", () => {
    const out: Array<{ kind: string; payload: Record<string, unknown> }> = [];
    mapInteractionDelta({ type: "text-delta", text: "Hel" }, (k, p) => out.push({ kind: k, payload: p }));
    mapInteractionDelta({ type: "text-delta", text: "lo" }, (k, p) => out.push({ kind: k, payload: p }));
    mapInteractionDelta({ type: "thinking-delta", text: "…" }, (k, p) => out.push({ kind: k, payload: p }));
    assert.deepEqual(out, [
      { kind: "assistant.delta", payload: { text: "Hel" } },
      { kind: "assistant.delta", payload: { text: "lo" } },
    ]);
  });
});
