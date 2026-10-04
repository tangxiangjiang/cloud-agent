// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

/** Default Local Agent AI access attempts (including the first try). */
export const DEFAULT_AI_RETRY_ATTEMPTS = 5;
/** Base backoff before the second attempt (ms); doubles each time, capped. */
export const DEFAULT_AI_RETRY_BASE_DELAY_MS = 3_000;
const MAX_AI_RETRY_DELAY_MS = 60_000;

const AUTH_RE =
  /\b(401|403|unauthorized|forbidden|invalid[_\s-]?api[_\s-]?key|authentication|api[_\s-]?key\s+(?:missing|invalid|revoked))\b/i;

const NETWORK_RE =
  /\b(econnreset|econnrefused|etimedout|enotfound|eai_again|enetunreach|ehostunreach|epipe|socket hang up|network|fetch failed|und_err|connect(?:ion)?\s+timed?\s*out|tls|ssl|temporar(?:y|ily)\s+unavailable|rate\s*limit|too many requests|502|503|504|gateway|overloaded|unavailable)\b/i;

/**
 * Whether an Agent create/send/run failure looks transient (network / retryable SDK).
 * Auth and policy errors are never retried.
 */
export function isRetryableAiError(err: unknown): boolean {
  if (err == null) return false;

  if (typeof err === "object") {
    const o = err as {
      isRetryable?: unknown;
      code?: unknown;
      message?: unknown;
      status?: unknown;
      statusCode?: unknown;
    };
    if (o.isRetryable === true) return true;
    if (o.isRetryable === false) return false;

    const status = Number(o.status ?? o.statusCode);
    if (status === 401 || status === 403) return false;
    if (status === 429 || status === 502 || status === 503 || status === 504) {
      return true;
    }

    const code = String(o.code ?? "").toLowerCase();
    if (
      code === "econnreset" ||
      code === "econnrefused" ||
      code === "etimedout" ||
      code === "enotfound" ||
      code === "eai_again" ||
      code === "enetunreach" ||
      code === "ehostunreach" ||
      code === "epipe" ||
      code.startsWith("und_err")
    ) {
      return true;
    }

    const message = String(o.message ?? "");
    if (message && AUTH_RE.test(message)) return false;
    if (message && NETWORK_RE.test(message)) return true;
  }

  if (typeof err === "string") {
    if (AUTH_RE.test(err)) return false;
    return NETWORK_RE.test(err);
  }

  return false;
}

/** Backoff before attempt `nextAttempt` (2-based after first failure). */
export function aiRetryDelayMs(
  nextAttempt: number,
  baseDelayMs: number = DEFAULT_AI_RETRY_BASE_DELAY_MS,
): number {
  const base = Math.max(0, baseDelayMs);
  if (base === 0) return 0;
  const exp = Math.max(0, nextAttempt - 2);
  const ms = base * 2 ** exp;
  return Math.min(ms, MAX_AI_RETRY_DELAY_MS);
}

export function sleep(ms: number): Promise<void> {
  if (ms <= 0) return Promise.resolve();
  return new Promise((resolve) => setTimeout(resolve, ms));
}
