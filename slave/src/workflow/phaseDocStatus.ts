// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import type { WorkflowNode } from "./types.js";

export type PhaseDocLifecycle = "awaiting_review" | "approved";

/** Resolve phaseRef under repo cwd; reject escape. */
export function resolvePhaseDocPath(
  repoCwd: string,
  phaseRef: string | null | undefined,
): { ok: true; absPath: string; relPath: string } | { ok: false; reason: string } {
  const raw = (phaseRef ?? "").trim();
  if (!raw) return { ok: false, reason: "phaseRef not set" };
  if (path.isAbsolute(raw)) {
    return { ok: false, reason: "phaseRef must be relative to repo cwd" };
  }
  const root = path.resolve(repoCwd);
  const abs = path.resolve(root, raw);
  const rel = path.relative(root, abs);
  if (rel.startsWith("..") || path.isAbsolute(rel)) {
    return { ok: false, reason: "phaseRef escapes repo cwd" };
  }
  return { ok: true, absPath: abs, relPath: rel.split(path.sep).join("/") };
}

/**
 * Pick Status text matching the file's existing casing style when possible.
 */
export function statusLabelForLifecycle(
  lifecycle: PhaseDocLifecycle,
  previousValue: string,
): string {
  const prev = previousValue.trim();
  const lower = prev.toLowerCase();
  if (lifecycle === "awaiting_review") {
    if (lower.includes("ready") && lower.includes("review")) return prev; // already
    if (/^[a-z]/.test(prev) && !/^[A-Z]/.test(prev)) {
      return "ready for human review";
    }
    return "Ready for human review";
  }
  // approved → Done / done
  if (lower === "done" || lower.startsWith("done ")) return prev.startsWith("d") ? "done" : "Done";
  if (/^[a-z]/.test(prev) && prev === lower) return "done";
  return "Done";
}

/**
 * Update a markdown `**Status:** …` / `Status: …` field (first match only).
 * Does not invent a Status line if none exists.
 */
export function markMarkdownStatusField(
  content: string,
  lifecycle: PhaseDocLifecycle,
): { next: string; changed: boolean; previous?: string; nextStatus?: string } {
  const re = /^([ \t]*(?:\*\*)?Status:\s*(?:\*\*)?[ \t]*)([^\n]*?)([ \t]*)$/im;
  const m = content.match(re);
  if (!m || m.index === undefined) {
    return { next: content, changed: false };
  }
  const prefix = m[1] ?? "";
  const previous = (m[2] ?? "").trim();
  const suffix = m[3] ?? "";
  const nextStatus = statusLabelForLifecycle(lifecycle, previous || "Pending");
  if (previous.toLowerCase() === nextStatus.toLowerCase()) {
    return { next: content, changed: false, previous, nextStatus };
  }
  const replacement = `${prefix}${nextStatus}${suffix}`;
  const next =
    content.slice(0, m.index) + replacement + content.slice(m.index + m[0].length);
  return { next, changed: true, previous, nextStatus };
}

/** Write phaseRef Status on awaiting_review / approve (dual SoT sync). */
export function writePhaseDocStatusOnLifecycle(opts: {
  repoCwd: string;
  node: WorkflowNode;
  lifecycle: PhaseDocLifecycle;
}): {
  written: boolean;
  path?: string;
  previous?: string;
  nextStatus?: string;
  reason?: string;
} {
  const resolved = resolvePhaseDocPath(opts.repoCwd, opts.node.phaseRef);
  if (!resolved.ok) {
    return { written: false, reason: resolved.reason };
  }
  let prev: string;
  try {
    prev = readFileSync(resolved.absPath, "utf8");
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return { written: false, path: resolved.relPath, reason: `read failed: ${message}` };
  }
  const marked = markMarkdownStatusField(prev, opts.lifecycle);
  if (!marked.changed || marked.next === prev) {
    return {
      written: false,
      path: resolved.relPath,
      ...(marked.previous !== undefined ? { previous: marked.previous } : {}),
      ...(marked.nextStatus !== undefined ? { nextStatus: marked.nextStatus } : {}),
      reason: marked.previous ? "already marked" : "no Status field",
    };
  }
  writeFileSync(resolved.absPath, marked.next, "utf8");
  return {
    written: true,
    path: resolved.relPath,
    ...(marked.previous !== undefined ? { previous: marked.previous } : {}),
    ...(marked.nextStatus !== undefined ? { nextStatus: marked.nextStatus } : {}),
  };
}
