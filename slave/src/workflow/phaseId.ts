// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/**
 * Phase node ids used in progress.md / milestones.
 * - Legacy cloud-agent: M05-P05
 * - NyraLang roadmaps (m_v0.N): m_v0.1_p0
 */

/** Full-string match (approve / catch-up). */
export const PHASE_ID_RE = /^(?:M\d{2}-P\d{2}|m_v\d+\.\d+_p\d+)$/;

/** Capture group for path basename / loose text. */
export const PHASE_ID_CAPTURE = /(M\d{2}-P\d{2}|m_v\d+\.\d+_p\d+)/;

export function isPhaseId(id: string): boolean {
  return PHASE_ID_RE.test(String(id ?? "").trim());
}

/** Prefer exact id; else first capture from path/text. */
export function extractPhaseId(text: string): string | null {
  const t = String(text ?? "").trim();
  if (!t) return null;
  if (PHASE_ID_RE.test(t)) return t;
  const m = t.match(PHASE_ID_CAPTURE);
  return m?.[1] ?? null;
}
