// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { readFileSync } from "node:fs";
import path from "node:path";
import type { WorkflowNode } from "./types.js";

/** Build Local Agent prompt for a DAG node (inline or phase_file under repo cwd). */
export function buildNodePrompt(node: WorkflowNode, repoCwd: string): string {
  const parts: string[] = [];
  const title = node.title?.trim();
  if (title) parts.push(`# ${title}`);

  const mode = node.prompt?.mode ?? (node.phaseRef ? "phase_file" : "inline");
  if (mode === "inline" && node.prompt?.inline) {
    parts.push(node.prompt.inline.trim());
  } else if (node.phaseRef) {
    const root = path.resolve(repoCwd);
    const phasePath = path.resolve(root, node.phaseRef);
    const rel = path.relative(root, phasePath);
    if (rel.startsWith("..") || path.isAbsolute(rel)) {
      throw new Error(`phaseRef escapes repo cwd: ${node.phaseRef}`);
    }
    try {
      parts.push(readFileSync(phasePath, "utf8"));
    } catch {
      parts.push(`Execute phase: ${node.phaseRef}`);
    }
  } else if (node.prompt?.inline) {
    parts.push(node.prompt.inline.trim());
  } else {
    parts.push(`Execute workflow node ${node.id}`);
  }

  if (node.prompt?.extra?.trim()) {
    parts.push(node.prompt.extra.trim());
  }
  parts.push(
    "When finished, stop. Do not mark the workflow node approved; human review is required.",
  );
  return parts.join("\n\n");
}
