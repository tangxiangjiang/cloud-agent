// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * Read-only parse of progress.md checkbox / table rows (M08 sync).
 * Does not write files.
 */

const PHASE_ID_RE = /M\d{2}-P\d{2}/;

export type ProgressStatus = "approved" | "done" | "pending" | "unknown";

export interface ProgressPhaseRow {
  id: string;
  progressStatus: ProgressStatus;
}

function rank(s: ProgressStatus): number {
  switch (s) {
    case "approved":
      return 3;
    case "done":
      return 2;
    case "pending":
      return 1;
    default:
      return 0;
  }
}

function mergeStatus(prev: ProgressStatus | undefined, next: ProgressStatus): ProgressStatus {
  if (!prev) return next;
  return rank(next) >= rank(prev) ? next : prev;
}

function normalizeTableStatus(raw: string): ProgressStatus {
  const t = raw.trim().toLowerCase();
  if (!t) return "unknown";
  if (t.includes("approved") || t === "x" || t === "done" || t === "ok") {
    if (t.includes("approved")) return "approved";
    if (t === "done" || t === "ok" || t === "x") return "done";
  }
  if (t.includes("pending") || t.includes("todo") || t === "-" || t === "") {
    return "pending";
  }
  if (t.includes("ready") || t.includes("running")) return "pending";
  return "unknown";
}

/** Extract phase checkbox / table statuses from progress markdown. */
export function parseProgressPhases(content: string): ProgressPhaseRow[] {
  const byId = new Map<string, ProgressStatus>();
  const text = content.replace(/\r\n/g, "\n");

  const checkboxRe =
    /^[ \t]*- \[([ xX])\][ \t]+(M\d{2}-P\d{2})(?:[ \t]+([^\n]*))?$/gm;
  let m: RegExpExecArray | null;
  while ((m = checkboxRe.exec(text)) !== null) {
    const checked = m[1] !== " ";
    const id = m[2]!;
    const rest = (m[3] ?? "").toLowerCase();
    let status: ProgressStatus;
    if (checked && rest.includes("@approved")) {
      status = "approved";
    } else if (checked) {
      status = "done";
    } else {
      status = "pending";
    }
    byId.set(id, mergeStatus(byId.get(id), status));
  }

  const tableRe = /^\s*\|\s*(M\d{2}-P\d{2})\s*\|\s*([^|\n]*)\|/gm;
  while ((m = tableRe.exec(text)) !== null) {
    const id = m[1]!;
    const status = normalizeTableStatus(m[2] ?? "");
    byId.set(id, mergeStatus(byId.get(id), status));
  }

  // Loose mentions of Mxx-Pxx @approved elsewhere
  const looseRe = /(M\d{2}-P\d{2})[^\n]*@approved/gi;
  while ((m = looseRe.exec(text)) !== null) {
    const id = m[1]!;
    if (PHASE_ID_RE.test(id)) {
      byId.set(id, mergeStatus(byId.get(id), "approved"));
    }
  }

  return [...byId.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([id, progressStatus]) => ({ id, progressStatus }));
}
