// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { SDKMessage } from "@cursor/sdk";
import {
  mapInteractionDelta,
  mapSdkMessage,
  toolDetail,
  type InteractionPhaseState,
} from "./mapStream.js";

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
    assert.equal(out.some((x) => x.kind === "assistant.delta"), true);
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
    assert.equal(out[0]?.kind, "status");
    assert.equal(out[0]?.payload.status, "tool");
    assert.equal(out[1]?.kind, "tool.started");
    assert.equal(out[1]?.payload.name, "read");
    assert.equal(out[1]?.payload.summary, "a.ts");
    assert.equal(out[2]?.kind, "tool.finished");
    assert.equal(out[2]?.payload.ok, true);
  });

  it("maps thinking and CREATING/RUNNING status", () => {
    const out: Array<{ kind: string; payload: Record<string, unknown> }> = [];
    const state: InteractionPhaseState = {};
    mapSdkMessage(
      {
        type: "thinking",
        agent_id: "a",
        run_id: "r",
        text: "consider approach",
      } as SDKMessage,
      (kind, payload) => out.push({ kind, payload }),
      { phaseState: state },
    );
    mapSdkMessage(
      {
        type: "status",
        agent_id: "a",
        run_id: "r",
        status: "RUNNING",
      } as SDKMessage,
      (kind, payload) => out.push({ kind, payload }),
      { phaseState: state },
    );
    assert.equal(out[0]?.payload.status, "thinking");
    assert.equal(out[1]?.payload.status, "running");
  });
});

describe("mapInteractionDelta", () => {
  it("maps text-delta chunks to assistant.delta", () => {
    const out: Array<{ kind: string; payload: Record<string, unknown> }> = [];
    const state: InteractionPhaseState = {};
    mapInteractionDelta({ type: "text-delta", text: "Hel" }, (k, p) => out.push({ kind: k, payload: p }), state);
    mapInteractionDelta({ type: "text-delta", text: "lo" }, (k, p) => out.push({ kind: k, payload: p }), state);
    assert.deepEqual(
      out.filter((x) => x.kind === "assistant.delta"),
      [
        { kind: "assistant.delta", payload: { text: "Hel" } },
        { kind: "assistant.delta", payload: { text: "lo" } },
      ],
    );
    assert.equal(out.filter((x) => x.kind === "status").length, 1);
    assert.equal(out.find((x) => x.kind === "status")?.payload.status, "writing");
  });

  it("emits thinking status once then tool from onDelta", () => {
    const out: Array<{ kind: string; payload: Record<string, unknown> }> = [];
    const state: InteractionPhaseState = {};
    mapInteractionDelta({ type: "thinking-delta", text: "…" }, (k, p) => out.push({ kind: k, payload: p }), state);
    mapInteractionDelta({ type: "thinking-delta", text: "more" }, (k, p) => out.push({ kind: k, payload: p }), state);
    mapInteractionDelta(
      {
        type: "tool-call-started",
        callId: "c9",
        toolCall: { type: "read", args: { path: "src/a.ts" } },
      },
      (k, p) => out.push({ kind: k, payload: p }),
      state,
    );
    mapInteractionDelta(
      {
        type: "tool-call-completed",
        callId: "c9",
        toolCall: { type: "read", result: { status: "success" } },
      },
      (k, p) => out.push({ kind: k, payload: p }),
      state,
    );
    assert.equal(out.filter((x) => x.kind === "status" && x.payload.status === "thinking").length, 1);
    assert.equal(out.some((x) => x.kind === "tool.started" && x.payload.summary === "src/a.ts"), true);
    assert.equal(out.some((x) => x.kind === "tool.finished" && x.payload.ok === true), true);
  });
});

describe("toolDetail", () => {
  it("prefers path/command", () => {
    assert.equal(toolDetail("read", { path: "x.go" }), "x.go");
    assert.equal(toolDetail("shell", { command: "ls -la" }), "ls -la");
  });
});
