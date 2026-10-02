// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { readFileSync } from "node:fs";
import path from "node:path";
import type { WorkflowNode } from "./types.js";

/**
 * Hard execution contract prepended to every roadmap phase / node run.
 * Phase markdown alone is not enough — agents otherwise skim, partial-finish,
 * or wander into later phases / unrelated refactors.
 */
export function buildPhaseExecutionPreamble(node: WorkflowNode): string {
  const lines = [
    "# Workflow phase execution contract",
    "",
    "You are a Local coding agent executing **ONE** workflow node from a roadmap.",
    "**Full fidelity is required.** Shipping an MVP, stub, stub-with-TODO, or \"minimal skeleton\" when the phase asks for complete behavior is a **contract violation**.",
    "",
    "## Identity",
    `- nodeId: \`${node.id}\``,
  ];
  const title = node.title?.trim();
  if (title) lines.push(`- title: ${title}`);
  if (node.phaseRef?.trim()) lines.push(`- phaseRef: \`${node.phaseRef.trim()}\``);
  lines.push(
    "",
    "## Hard rules (do not violate)",
    "1. **Single phase only** — implement this node alone. Do **not** start later phases, sibling milestones, or drive-by refactors outside the phase scope.",
    "2. **Finish thoroughly** — every Goal / Work / DoD / Exit / 完成定义 / 禁止事项 item is mandatory. No half-done checklists, deferred work, or \"follow-up later\".",
    "3. **NO stubs / MVP shortcuts** — forbidden unless the phase **explicitly** says stub/MVP/placeholder is the deliverable:",
    "   - empty/`unimplemented`/`todo!`/`panic(\"todo\")`/`throw new Error(\"TODO\")` bodies",
    "   - `NotImplemented`, `pass`-only, `return null` stand-ins for real logic",
    "   - interface-only or facade-only with no real implementation behind it",
    "   - \"minimal happy path\" when the phase lists full surface / port / parity",
    "   - deleting or skipping cases to make tests green without real behavior",
    "4. **No empty / progress-only runs** — finishing with zero repo changes, or only ticking `progress.md`, is a failure. Produce real code/docs required by the phase.",
    "5. **Evidence / harness phases** — if the phase is evidence-close / harness, you MUST run the harness and leave green evidence (code or harness artifacts). Checkbox theatre without harness output is forbidden.",
    "6. **Port / copy existing code when cited** — if the phase points at nlib, Nexus, sibling packages, prior modules, or \"port from X\":",
    "   - **read that source first** and transfer the **full** behavior, not a rewrite-from-scratch toy",
    "   - keep semantics, edge cases, and public surface; adapt names/paths only as the design requires",
    "   - do **not** invent a thinner substitute and call it done",
    "7. **Stay on rails** — obey 范围 / 允许 paths and 禁止事项. Prefer existing project patterns; do not invent parallel architecture.",
    "8. **Verify with evidence** — run the tests/builds/checks the phase (or dodChecks) requires; fix failures before you stop. If the phase lists commands, run them.",
    "9. **Read before write** — open linked design/docs under the repo that the phase cites; do not guess APIs from memory when the doc exists.",
    "10. **Checked ≠ done** — if Work/DoD boxes are already `[x]` but the codebase does not satisfy them, implement until they truly hold.",
    "11. **Stop only when complete** — in your final summary you MUST list:",
    "   - which DoD/Work items are satisfied",
    "   - source paths you ported/copied from (if any)",
    "   - explicit confirmation: **no intentional stubs/MVP left for this phase**",
    "   Do **not** mark the workflow node approved; human review is required.",
    "12. **progress.md** — do not tick progress checkboxes unless the phase explicitly asks for design-doc edits; approval still gates progress.",
  );
  const dod = node.dodChecks?.map((c) => c.trim()).filter(Boolean) ?? [];
  if (dod.length > 0) {
    lines.push("", "## Required checks (dodChecks)");
    for (const c of dod) lines.push(`- ${c}`);
  }
  return lines.join("\n");
}

const STOP_TAIL = [
  "## Pre-stop self-check",
  "- Did I implement the **full** phase surface, or did I leave stubs/MVP?",
  "- If a reference implementation was cited, did I port it rather than invent a thinner version?",
  "- Is my working tree **empty** or **only progress.md**? If yes, I am not done — keep working.",
  "- Evidence/harness phase: did I actually run harness and record green evidence?",
  "- Would a reviewer reject this as incomplete? If yes, keep working.",
  "",
  "When finished, stop. Do not mark the workflow node approved; human review is required.",
].join("\n");


/** Build Local Agent prompt for a DAG node (inline or phase_file under repo cwd). */
export function buildNodePrompt(node: WorkflowNode, repoCwd: string): string {
  const parts: string[] = [buildPhaseExecutionPreamble(node)];

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
      parts.push(
        [
          "## Phase document (source of truth for this node)",
          `path: \`${node.phaseRef}\``,
          "",
          readFileSync(phasePath, "utf8").trimEnd(),
        ].join("\n"),
      );
    } catch {
      parts.push(`Execute phase file (missing on disk — still stay in scope): ${node.phaseRef}`);
    }
  } else if (node.prompt?.inline) {
    parts.push(node.prompt.inline.trim());
  } else {
    parts.push(`Execute workflow node ${node.id}`);
  }

  if (node.prompt?.extra?.trim()) {
    parts.push(node.prompt.extra.trim());
  }
  parts.push(STOP_TAIL);
  return parts.join("\n\n");
}

/** Prompt for App revise follow-up (natural-language instruction only). */
export function buildRevisePrompt(
  node: WorkflowNode,
  instruction: string,
): string {
  const title = node.title?.trim();
  const header = title
    ? `Human revise for workflow node ${node.id} (${title})`
    : `Human revise for workflow node ${node.id}`;
  return [
    buildPhaseExecutionPreamble(node),
    header,
    "## Revise instruction (priority for this turn)",
    instruction.trim(),
    "Apply the requested changes **within this node's phase scope**. Do not expand into unrelated phases.",
    STOP_TAIL,
  ].join("\n\n");
}
