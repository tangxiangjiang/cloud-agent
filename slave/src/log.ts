// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

export type LogLevel = "debug" | "info" | "warn" | "error";

const LEVEL_ORDER: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
};

let minLevel: LogLevel = "info";

export function setLogLevel(level: LogLevel): void {
  minLevel = level;
}

/** Redact secrets that may appear in free-form strings. */
export function redact(value: string): string {
  if (!value) return value;
  let out = value;
  // Bearer tokens / long hex-ish secrets
  out = out.replace(/\b(Bearer\s+)[A-Za-z0-9._\-+=/]{8,}/gi, "$1***");
  out = out.replace(/\b[A-Za-z0-9]{32,}\b/g, "***");
  return out;
}

function shouldLog(level: LogLevel): boolean {
  return LEVEL_ORDER[level] >= LEVEL_ORDER[minLevel];
}

function write(level: LogLevel, msg: string, fields?: Record<string, unknown>): void {
  if (!shouldLog(level)) return;
  const line: Record<string, unknown> = {
    ts: new Date().toISOString(),
    level,
    msg: redact(msg),
  };
  if (fields) {
    for (const [k, v] of Object.entries(fields)) {
      if (typeof v === "string") {
        line[k] = redact(v);
      } else {
        line[k] = v;
      }
    }
  }
  const text = JSON.stringify(line);
  if (level === "error") {
    console.error(text);
  } else if (level === "warn") {
    console.warn(text);
  } else {
    console.log(text);
  }
}

export const log = {
  debug: (msg: string, fields?: Record<string, unknown>) => write("debug", msg, fields),
  info: (msg: string, fields?: Record<string, unknown>) => write("info", msg, fields),
  warn: (msg: string, fields?: Record<string, unknown>) => write("warn", msg, fields),
  error: (msg: string, fields?: Record<string, unknown>) => write("error", msg, fields),
};
