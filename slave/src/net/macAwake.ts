// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { spawn, type ChildProcess } from "node:child_process";

/**
 * macOS cannot keep a single userspace process running through *full* system
 * sleep (lid close / battery). What it *can* do is a power assertion so idle
 * sleep (and system sleep on AC) does not freeze Master/Slave.
 *
 * Uses `caffeinate -i -s -w <pid>`:
 *   -i  PreventUserIdleSystemSleep
 *   -s  PreventSystemSleep (typically AC power)
 *
 * Disable: MAC_PREVENT_SLEEP=0
 */
export function caffeinateArgs(pid: number): string[] {
  return ["-i", "-s", "-w", String(pid)];
}

export function shouldHoldMacAwake(
  platform: string = process.platform,
  env: NodeJS.ProcessEnv = process.env,
): boolean {
  if (platform !== "darwin") return false;
  const v = env.MAC_PREVENT_SLEEP?.trim().toLowerCase();
  if (v === "0" || v === "false" || v === "off" || v === "no") return false;
  return true;
}

export type HoldMacAwakeOptions = {
  platform?: string;
  env?: NodeJS.ProcessEnv;
  pid?: number;
  spawnFn?: typeof spawn;
  log?: (msg: string, meta?: Record<string, unknown>) => void;
};

/** Hold an idle-sleep assertion for this process. No-op off Darwin. */
export function holdMacAwake(opts?: HoldMacAwakeOptions): ChildProcess | null {
  const platform = opts?.platform ?? process.platform;
  const env = opts?.env ?? process.env;
  if (!shouldHoldMacAwake(platform, env)) return null;
  const pid = opts?.pid ?? process.pid;
  const spawnFn = opts?.spawnFn ?? spawn;
  try {
    const child = spawnFn("caffeinate", caffeinateArgs(pid), {
      stdio: "ignore",
      detached: true,
    });
    child.unref();
    opts?.log?.("macOS power assertion (caffeinate -i -s)", { pid });
    return child;
  } catch (err) {
    opts?.log?.("caffeinate unavailable; idle sleep not prevented", {
      error: err instanceof Error ? err.message : String(err),
    });
    return null;
  }
}
