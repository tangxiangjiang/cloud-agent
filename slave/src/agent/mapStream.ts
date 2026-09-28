// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { SDKMessage } from "@cursor/sdk";
import type { EventKind } from "../gateway/types.js";

export type MappedEmit = (kind: EventKind, payload: Record<string, unknown>) => void;

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

/**
 * Map Cursor SDK stream messages → cloud-agent task.event kinds.
 * Best-effort for tool events; ignores thinking/usage/request noise.
 */
export function mapSdkMessage(event: SDKMessage, emit: MappedEmit): void {
  switch (event.type) {
    case "assistant":
      for (const block of event.message.content) {
        if (block.type === "text" && block.text) {
          emit("assistant.delta", { text: block.text });
        }
      }
      break;
    case "tool_call":
      if (event.status === "running") {
        const payload: Record<string, unknown> = {
          name: event.name,
          callId: event.call_id,
        };
        const summary = summarizeArgs(event.args);
        if (summary !== undefined) payload.summary = summary;
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
      if (event.status === "RUNNING") {
        emit("status", { status: "running" });
      }
      break;
    default:
      break;
  }
}
