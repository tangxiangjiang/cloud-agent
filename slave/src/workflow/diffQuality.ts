// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { WorkflowNode } from "./types.js";
import type { NodeDiffFile } from "./diff.js";

const PROGRESS_RE = /(^|\/)progress\.md$/i;
const CODE_PATH_RE =
  /\.(ts|tsx|js|jsx|mjs|cjs|go|rs|py|c|cc|cpp|cxx|h|hpp|ny|nex|ni|nu|swift|kt|kts|java|cs|rb|php|zig|mm|m)$/i;
const CODE_NAME_RE =
  /(^|\/)(Cargo\.toml|go\.mod|go\.sum|package\.json|CMakeLists\.txt|Makefile|meson\.build)$/i;

export type PhaseKind = {
  designLock: boolean;
  evidence: boolean;
};

/** Heuristic from node id / title / phaseRef. */
export function phaseKindFromNode(node: WorkflowNode): PhaseKind {
  const id = String(node.id ?? "").toLowerCase();
  const blob = `${id} ${node.title ?? ""} ${node.phaseRef ?? ""}`.toLowerCase();
  const designLock =
    /design[-_ ]?lock/.test(blob) ||
    /(^|[^a-z0-9])p00([^a-z0-9]|$)/.test(id) ||
    /-p00-/.test(blob);
  const evidence =
    /evidence|harness|收口|evidence-close|harness-close/.test(blob) ||
    /-p0[0-9]+-.*(close|evidence|harness)/.test(blob);
  return { designLock, evidence };
}

export type DiffAssessment = {
  empty: boolean;
  progressOnly: boolean;
  hasCode: boolean;
  substantive: boolean;
  reason: string;
};

export function isProgressPath(p: string): boolean {
  return PROGRESS_RE.test(p.replace(/\\/g, "/"));
}

export function isCodePath(p: string): boolean {
  const n = p.replace(/\\/g, "/");
  return CODE_PATH_RE.test(n) || CODE_NAME_RE.test(n);
}

/**
 * Rubber-stamp guard: empty / progress.md-only diffs are never substantive.
 * Evidence/harness phases may be docs-only (Record / status / catalogs) when
 * the work is verify+record harness numbers — code changes are preferred but
 * not required. Design-lock may be docs-only.
 */
export function assessNodeDiff(
  files: readonly Pick<NodeDiffFile, "path">[],
  kind: PhaseKind = { designLock: false, evidence: false },
): DiffAssessment {
  if (!files.length) {
    return {
      empty: true,
      progressOnly: false,
      hasCode: false,
      substantive: false,
      reason: "empty diff (no changes vs baseline) — refuse rubber-stamp review",
    };
  }
  const nonProgress = files.filter((f) => !isProgressPath(f.path));
  if (nonProgress.length === 0) {
    return {
      empty: false,
      progressOnly: true,
      hasCode: false,
      substantive: false,
      reason: "diff is only progress.md — refuse progress-only approve/stamp",
    };
  }
  const hasCode = nonProgress.some((f) => isCodePath(f.path));
  if (kind.evidence) {
    return {
      empty: false,
      progressOnly: false,
      hasCode,
      substantive: true,
      reason: hasCode
        ? "evidence phase has code/harness artifact changes"
        : "evidence phase has Record/docs changes (harness numbers belong in phase docs)",
    };
  }
  if (kind.designLock) {
    return {
      empty: false,
      progressOnly: false,
      hasCode,
      substantive: true,
      reason: "design-lock docs/code changes accepted",
    };
  }
  // Default implementation phases: any non-progress change counts (code preferred).
  return {
    empty: false,
    progressOnly: false,
    hasCode,
    substantive: true,
    reason: hasCode ? "has code changes" : "has non-progress doc changes",
  };
}
