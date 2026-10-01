// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { SDKMessage } from "@cursor/sdk";
import type { EventKind } from "../gateway/types.js";

export type MappedEmit = (kind: EventKind, payload: Record<string, unknown>) => void;

/** Tracks last emitted high-level phase to avoid status spam. */
export type InteractionPhaseState = {
  phase?: "thinking" | "writing" | "tool" | string;
};

function summarizeArgs(args: unknown): string | undefined {
  if (args === undefined || args === null) return undefined;
  try {
    const s = JSON.stringify(args);
    if (s.length <= 200) return s;
    return `${s.slice(0, 197)}...`;
  } catch {
    return undefined;
  }
}

/** Prefer path / command / pattern over raw JSON for UI labels. */
export function toolDetail(name: string, args: unknown): string | undefined {
  if (args && typeof args === "object" && !Array.isArray(args)) {
    const a = args as Record<string, unknown>;
    for (const key of ["path", "target", "file", "filePath", "command", "pattern", "query", "glob"]) {
      const v = a[key];
      if (typeof v === "string" && v.trim()) {
        const s = v.trim();
        return s.length <= 160 ? s : `${s.slice(0, 157)}...`;
      }
    }
  }
  void name;
  return summarizeArgs(args);
}

function emitPhase(
  emit: MappedEmit,
  state: InteractionPhaseState | undefined,
  phase: string,
  extra?: Record<string, unknown>,
): void {
  if (state) {
    if (state.phase === phase) return;
    state.phase = phase;
  }
  emit("status", { status: phase, ...extra });
}

/**
 * Map Cursor SDK stream messages → cloud-agent task.event kinds.
 * Best-effort for tool/thinking; usage/request noise ignored.
 *
 * When `skipAssistantText` is true (token deltas already emitted via onDelta),
 * do not re-emit full assistant text blocks (avoids duplicate bubbles).
 */
export function mapSdkMessage(
  event: SDKMessage,
  emit: MappedEmit,
  opts?: { skipAssistantText?: boolean; phaseState?: InteractionPhaseState },
): void {
  switch (event.type) {
    case "assistant":
      if (opts?.skipAssistantText) break;
      for (const block of event.message.content) {
        if (block.type === "text" && block.text) {
          emitPhase(emit, opts?.phaseState, "writing");
          emit("assistant.delta", { text: block.text });
        }
      }
      break;
    case "thinking": {
      const text = typeof event.text === "string" ? event.text.trim() : "";
      emitPhase(emit, opts?.phaseState, "thinking", {
        ...(text ? { preview: text.length <= 120 ? text : `${text.slice(0, 117)}...` } : {}),
      });
      break;
    }
    case "tool_call":
      if (event.status === "running") {
        const payload: Record<string, unknown> = {
          name: event.name,
          callId: event.call_id,
        };
        const summary = toolDetail(event.name, event.args);
        if (summary !== undefined) payload.summary = summary;
        emitPhase(emit, opts?.phaseState, "tool", { name: event.name });
        emit("tool.started", payload);
      } else {
        emit("tool.finished", {
          name: event.name,
          ok: event.status === "completed",
          callId: event.call_id,
        });
      }
      break;
    case "status":
      if (event.status === "CREATING") {
        emitPhase(emit, opts?.phaseState, "creating");
      } else if (event.status === "RUNNING") {
        emitPhase(emit, opts?.phaseState, "running");
      } else if (event.status === "FINISHED") {
        emitPhase(emit, opts?.phaseState, "finished");
      } else if (event.status === "ERROR") {
        emitPhase(emit, opts?.phaseState, "error");
      } else if (event.status === "CANCELLED") {
        emitPhase(emit, opts?.phaseState, "cancelled");
      }
      break;
    default:
      break;
  }
}

type DeltaUpdate = {
  type?: string;
  text?: string;
  callId?: string;
  toolCall?: { type?: string; args?: unknown; result?: { status?: string } };
};

/**
 * Map raw SDK InteractionUpdate (onDelta) → task.event.
 * Prefer text-delta for true token streaming; surface thinking + early tool starts.
 */
export function mapInteractionDelta(
  update: DeltaUpdate,
  emit: MappedEmit,
  state?: InteractionPhaseState,
): void {
  if (update.type === "thinking-delta") {
    emitPhase(emit, state, "thinking");
    return;
  }
  if (update.type === "thinking-completed") {
    // Stay on thinking until text/tool; no-op if already moved on.
    return;
  }
  if (update.type === "text-delta" && typeof update.text === "string" && update.text) {
    emitPhase(emit, state, "writing");
    emit("assistant.delta", { text: update.text });
    return;
  }
  if (update.type === "tool-call-started") {
    const name = update.toolCall?.type?.trim() || "tool";
    const callId = typeof update.callId === "string" ? update.callId : undefined;
    const payload: Record<string, unknown> = { name };
    if (callId) payload.callId = callId;
    const summary = toolDetail(name, update.toolCall?.args);
    if (summary !== undefined) payload.summary = summary;
    emitPhase(emit, state, "tool", { name });
    emit("tool.started", payload);
    return;
  }
  if (update.type === "tool-call-completed") {
    const name = update.toolCall?.type?.trim() || "tool";
    const callId = typeof update.callId === "string" ? update.callId : undefined;
    const resultStatus = update.toolCall?.result?.status;
    emit("tool.finished", {
      name,
      ok: resultStatus !== "error",
      ...(callId ? { callId } : {}),
    });
  }
}
