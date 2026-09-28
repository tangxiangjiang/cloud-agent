// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import type { WorkflowNode } from "./types.js";

const PHASE_ID_RE = /^M\d{2}-P\d{2}$/;
const PHASE_FROM_PATH_RE = /(M\d{2}-P\d{2})/;

/**
 * Resolve progressDoc under repo cwd. Rejects path escape and non-progress files.
 * progressDoc must be a relative path whose basename is `progress.md`.
 */
export function resolveProgressPath(
  repoCwd: string,
  progressDoc: string | null | undefined,
): { ok: true; absPath: string; relPath: string } | { ok: false; reason: string } {
  const raw = (progressDoc ?? "").trim();
  if (!raw) {
    return { ok: false, reason: "progressDoc not set on workflow" };
  }
  if (path.isAbsolute(raw)) {
    return { ok: false, reason: "progressDoc must be relative to repo cwd" };
  }
  const root = path.resolve(repoCwd);
  const abs = path.resolve(root, raw);
  const rel = path.relative(root, abs);
  if (rel.startsWith("..") || path.isAbsolute(rel)) {
    return { ok: false, reason: "progressDoc escapes repo cwd" };
  }
  if (path.basename(abs).toLowerCase() !== "progress.md") {
    return { ok: false, reason: "progressDoc basename must be progress.md" };
  }
  return { ok: true, absPath: abs, relPath: rel.split(path.sep).join("/") };
}

/** Extract Mxx-Pxx from node id, unitId, or phaseRef filename. */
export function phaseKeyFromNode(node: WorkflowNode): string | null {
  const candidates = [node.id, node.unitId ?? "", node.title ?? ""];
  for (const c of candidates) {
    const t = String(c).trim();
    if (PHASE_ID_RE.test(t)) return t;
  }
  const ref = (node.phaseRef ?? "").trim();
  if (ref) {
    const base = path.basename(ref);
    const m = base.match(PHASE_FROM_PATH_RE);
    if (m?.[1]) return m[1];
  }
  return null;
}

/**
 * Mark a phase checkbox approved in progress markdown.
 * Format: `- [x] M05-P05 @approved <iso8601>`
 * Also updates `| M05-P05 | … |` table rows to status `approved` when present.
 * Does not invent new design content beyond the checkbox / status cell.
 */
export function markPhaseApproved(
  content: string,
  phaseKey: string,
  approvedAt: string,
): { next: string; changed: boolean } {
  if (!PHASE_ID_RE.test(phaseKey)) {
    return { next: content, changed: false };
  }
  const stamp = approvedAt.trim() || new Date().toISOString();
  const approvedLine = `- [x] ${phaseKey} @approved ${stamp}`;
  let changed = false;
  let next = content;

  // Same-line only: do not let \s match newlines (would swallow following checkboxes).
  const checkboxRe = new RegExp(
    `^([ \\t]*)- \\[[ xX]\\] ${escapeRegExp(phaseKey)}(?:[ \\t][^\\n]*)?$`,
    "m",
  );
  if (checkboxRe.test(next)) {
    next = next.replace(checkboxRe, `$1${approvedLine}`);
    changed = true;
  } else {
    const append = `\n${approvedLine}\n`;
    next = next.replace(/\s*$/, "") + append;
    changed = true;
  }

  // Table row: | M05-P05 | pending | note | → | M05-P05 | approved | note |
  const tableRe = new RegExp(
    `^(\\s*\\|\\s*)${escapeRegExp(phaseKey)}(\\s*\\|\\s*)([^|\\n]*?)(\\s*\\|)`,
    "gm",
  );
  const tableNext = next.replace(tableRe, (_all, a, b, _status, d) => {
    changed = true;
    return `${a}${phaseKey}${b}approved${d}`;
  });
  next = tableNext;

  return { next, changed };
}

/** Write progress.md for an approved node. Throws on I/O errors after path validation. */
export function writeProgressOnApprove(opts: {
  repoCwd: string;
  progressDoc: string | null | undefined;
  node: WorkflowNode;
  approvedAt?: string;
}): {
  written: boolean;
  phaseKey: string | null;
  path?: string;
  reason?: string;
} {
  const resolved = resolveProgressPath(opts.repoCwd, opts.progressDoc);
  if (!resolved.ok) {
    return { written: false, phaseKey: null, reason: resolved.reason };
  }
  const phaseKey = phaseKeyFromNode(opts.node);
  if (!phaseKey) {
    return {
      written: false,
      phaseKey: null,
      path: resolved.relPath,
      reason: "cannot derive Mxx-Pxx from node",
    };
  }
  const prev = readFileSync(resolved.absPath, "utf8");
  const iso = opts.approvedAt ?? new Date().toISOString();
  const { next, changed } = markPhaseApproved(prev, phaseKey, iso);
  if (!changed || next === prev) {
    // Still rewrite if checkbox already correct — idempotent no-op.
    return { written: false, phaseKey, path: resolved.relPath, reason: "already marked" };
  }
  writeFileSync(resolved.absPath, next, "utf8");
  return { written: true, phaseKey, path: resolved.relPath };
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
