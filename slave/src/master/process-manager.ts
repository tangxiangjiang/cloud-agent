// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { spawn, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import {
  childConfigPath,
  childLogPath,
  childPidPath,
  childSpawnTokenPath,
  clearSlaveStateDir,
  writeChildSlaveConfig,
} from "./child-config.js";
import { MasterControlError } from "./errors.js";
import { SerialQueue } from "./serial-queue.js";
import type {
  MasterConfig,
  MasterSlaveEntry,
  SlaveProcessState,
  SlaveRuntimeSnapshot,
} from "./types.js";

export type CancelSlaveTasks = (slaveId: string) => Promise<void>;

export type ProcessManagerOptions = {
  masterConfigPath: string;
  /** Optional: cancel running tasks on Gateway before SIGTERM (P04). */
  cancelSlaveTasks?: CancelSlaveTasks;
};

type RuntimeSlot = {
  entry: MasterSlaveEntry;
  state: SlaveProcessState;
  child: ChildProcess | null;
  pid: number | null;
  spawnToken: string | null;
  lastError: string | null;
  startedAt: string | null;
};

function pidAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms));
}

function readPidFile(file: string): number | null {
  try {
    const n = Number.parseInt(fs.readFileSync(file, "utf8").trim(), 10);
    return Number.isFinite(n) && n > 0 ? n : null;
  } catch {
    return null;
  }
}

/**
 * Local process lifecycle for child Slaves.
 * Cold start: all stopped. Start only when requested.
 */
export class ProcessManager {
  private cfg: MasterConfig;
  private readonly masterConfigPath: string;
  private readonly cancelSlaveTasks: CancelSlaveTasks | undefined;
  private readonly slots = new Map<string, RuntimeSlot>();
  private readonly queue = new SerialQueue();
  private startInFlight = 0;

  constructor(cfg: MasterConfig, opts: ProcessManagerOptions) {
    this.cfg = cfg;
    this.masterConfigPath = path.resolve(opts.masterConfigPath);
    this.cancelSlaveTasks = opts.cancelSlaveTasks;
    for (const entry of cfg.slaves) {
      this.slots.set(entry.id, this.newSlot(entry));
    }
  }

  private newSlot(entry: MasterSlaveEntry): RuntimeSlot {
    return {
      entry,
      state: "stopped",
      child: null,
      pid: null,
      spawnToken: null,
      lastError: null,
      startedAt: null,
    };
  }

  getConfig(): MasterConfig {
    return this.cfg;
  }

  /** Reload config object (after upsert/delete) and sync slots. */
  applyConfig(cfg: MasterConfig): void {
    this.cfg = cfg;
    const keep = new Set(cfg.slaves.map((s) => s.id));
    for (const id of [...this.slots.keys()]) {
      if (!keep.has(id)) {
        const slot = this.slots.get(id);
        if (
          slot &&
          (slot.state === "running" ||
            slot.state === "starting" ||
            slot.state === "stopping")
        ) {
          throw new MasterControlError(
            "not_stopped",
            `cannot remove running slave ${id}; stop first`,
          );
        }
        clearSlaveStateDir(this.masterConfigPath, id);
        this.slots.delete(id);
      }
    }
    for (const entry of cfg.slaves) {
      const existing = this.slots.get(entry.id);
      if (existing) {
        existing.entry = entry;
      } else {
        this.slots.set(entry.id, this.newSlot(entry));
      }
    }
  }

  listSnapshots(): SlaveRuntimeSnapshot[] {
    return this.cfg.slaves.map((e) => this.snapshot(e.id));
  }

  snapshot(slaveId: string): SlaveRuntimeSnapshot {
    const slot = this.requireSlot(slaveId);
    return {
      id: slot.entry.id,
      ...(slot.entry.name !== undefined ? { name: slot.entry.name } : {}),
      enabled: slot.entry.enabled,
      project: slot.entry.project,
      state: slot.state,
      pid: slot.pid,
      lastError: slot.lastError,
      startedAt: slot.startedAt,
    };
  }

  /** Adopt orphan pid files from a previous Master process (best-effort). */
  adoptOrphans(): void {
    for (const entry of this.cfg.slaves) {
      const slot = this.slots.get(entry.id);
      if (!slot || slot.state === "running") continue;
      const pidFile = childPidPath(this.masterConfigPath, entry.id);
      const tokenFile = childSpawnTokenPath(this.masterConfigPath, entry.id);
      const pid = readPidFile(pidFile);
      if (pid == null || !pidAlive(pid)) {
        this.clearPidArtifacts(entry.id);
        continue;
      }
      let token: string | null = null;
      try {
        token = fs.readFileSync(tokenFile, "utf8").trim() || null;
      } catch {
        token = null;
      }
      slot.state = "running";
      slot.pid = pid;
      slot.child = null;
      slot.spawnToken = token;
      slot.startedAt = new Date().toISOString();
      slot.lastError = null;
    }
  }

  start(slaveId: string): Promise<SlaveRuntimeSnapshot> {
    return this.queue.enqueue(slaveId, () => this.startUnlocked(slaveId));
  }

  stop(slaveId: string): Promise<SlaveRuntimeSnapshot> {
    return this.queue.enqueue(slaveId, () => this.stopUnlocked(slaveId));
  }

  restart(slaveId: string): Promise<SlaveRuntimeSnapshot> {
    return this.queue.enqueue(slaveId, async () => {
      await this.stopUnlocked(slaveId);
      return this.startUnlocked(slaveId);
    });
  }

  private requireSlot(slaveId: string): RuntimeSlot {
    const slot = this.slots.get(slaveId);
    if (!slot) {
      throw new MasterControlError(
        "unknown_slave",
        `unknown slave id: ${slaveId}`,
      );
    }
    return slot;
  }

  private countRunningOrStarting(): number {
    let n = 0;
    for (const s of this.slots.values()) {
      if (s.state === "running" || s.state === "starting") n += 1;
    }
    return n;
  }

  private clearPidArtifacts(slaveId: string): void {
    for (const f of [
      childPidPath(this.masterConfigPath, slaveId),
      childSpawnTokenPath(this.masterConfigPath, slaveId),
    ]) {
      try {
        fs.unlinkSync(f);
      } catch {
        /* ignore */
      }
    }
  }

  private async startUnlocked(slaveId: string): Promise<SlaveRuntimeSnapshot> {
    const slot = this.requireSlot(slaveId);
    if (!slot.entry.enabled) {
      throw new MasterControlError(
        "slave_disabled",
        `slave ${slaveId} is disabled`,
      );
    }
    if (slot.state === "running") {
      return this.snapshot(slaveId);
    }
    if (slot.state === "starting" || slot.state === "stopping") {
      throw new MasterControlError(
        "busy",
        `slave ${slaveId} is busy (${slot.state})`,
      );
    }

    // Reconcile: live pid → adopt (停新保旧), never double-spawn.
    const existingPid = readPidFile(
      childPidPath(this.masterConfigPath, slaveId),
    );
    if (existingPid != null && pidAlive(existingPid)) {
      slot.state = "running";
      slot.pid = existingPid;
      slot.child = null;
      slot.startedAt = new Date().toISOString();
      slot.lastError = null;
      try {
        slot.spawnToken = fs
          .readFileSync(childSpawnTokenPath(this.masterConfigPath, slaveId), "utf8")
          .trim();
      } catch {
        slot.spawnToken = null;
      }
      return this.snapshot(slaveId);
    }
    this.clearPidArtifacts(slaveId);

    if (this.countRunningOrStarting() >= this.cfg.maxRunningSlaves) {
      throw new MasterControlError(
        "slave_limit",
        `maxRunningSlaves (${this.cfg.maxRunningSlaves}) reached`,
      );
    }
    if (this.startInFlight >= this.cfg.maxConcurrentStarts) {
      throw new MasterControlError(
        "start_busy",
        `maxConcurrentStarts (${this.cfg.maxConcurrentStarts}) reached`,
      );
    }

    this.startInFlight += 1;
    slot.state = "starting";
    slot.lastError = null;
    try {
      const configPath = writeChildSlaveConfig(
        this.masterConfigPath,
        this.cfg,
        slot.entry,
      );
      const logPath = childLogPath(this.masterConfigPath, slaveId);
      fs.mkdirSync(path.dirname(logPath), { recursive: true });
      const logFd = fs.openSync(logPath, "a");
      const [cmd, ...args] = this.cfg.slaveCommand;
      if (!cmd) {
        throw new Error("slaveCommand is empty");
      }
      const spawnToken = randomUUID();
      const childArgs = [...args, "--config", configPath];
      // Windows: .cmd/.bat cannot be spawned without a shell (EINVAL).
      const useShell =
        process.platform === "win32" && /\.(cmd|bat)$/i.test(cmd);
      const child = spawn(cmd, childArgs, {
        cwd: this.cfg.slaveCwd,
        env: { ...process.env },
        stdio: ["ignore", logFd, logFd],
        windowsHide: true,
        shell: useShell,
      });
      fs.closeSync(logFd);

      if (child.pid == null) {
        throw new Error("failed to spawn child (no pid)");
      }

      const stateDir = path.dirname(
        childPidPath(this.masterConfigPath, slaveId),
      );
      fs.mkdirSync(stateDir, { recursive: true });
      fs.writeFileSync(
        childPidPath(this.masterConfigPath, slaveId),
        String(child.pid),
        "utf8",
      );
      fs.writeFileSync(
        childSpawnTokenPath(this.masterConfigPath, slaveId),
        spawnToken,
        "utf8",
      );

      slot.child = child;
      slot.pid = child.pid;
      slot.spawnToken = spawnToken;
      slot.startedAt = new Date().toISOString();
      slot.state = "running";

      // Allow one-shot CLI (`master start`) to exit while child keeps running.
      // Serve stays alive via Gateway WS / heartbeats.
      child.unref();

      child.on("exit", (code, signal) => {
        if (this.slots.get(slaveId)?.child === child) {
          slot.child = null;
          slot.pid = null;
          slot.spawnToken = null;
          if (slot.state !== "stopping") {
            slot.state =
              code === 0 || signal === "SIGTERM" ? "stopped" : "error";
            if (slot.state === "error") {
              slot.lastError = `exited code=${code} signal=${signal}`;
            }
          } else {
            slot.state = "stopped";
          }
          this.clearPidArtifacts(slaveId);
        }
      });

      return this.snapshot(slaveId);
    } catch (err) {
      slot.state = "error";
      slot.lastError = err instanceof Error ? err.message : String(err);
      slot.child = null;
      slot.pid = null;
      slot.spawnToken = null;
      throw err;
    } finally {
      this.startInFlight -= 1;
    }
  }

  private async stopUnlocked(slaveId: string): Promise<SlaveRuntimeSnapshot> {
    const slot = this.requireSlot(slaveId);
    if (slot.state === "stopped") {
      return this.snapshot(slaveId);
    }
    if (slot.state === "stopping") {
      throw new MasterControlError(
        "busy",
        `slave ${slaveId} is already stopping`,
      );
    }

    slot.state = "stopping";
    try {
      if (this.cancelSlaveTasks) {
        try {
          await this.cancelSlaveTasks(slaveId);
        } catch {
          slot.lastError = "cancel_skipped_gateway_unreachable";
        }
      }

      const pid = slot.pid;
      const child = slot.child;
      if (child && child.exitCode === null) {
        try {
          child.kill("SIGTERM");
        } catch {
          /* ignore */
        }
      } else if (pid != null && pidAlive(pid)) {
        try {
          process.kill(pid, "SIGTERM");
        } catch {
          /* ignore */
        }
      }

      const deadline = Date.now() + this.cfg.gracePeriodMs;
      while (Date.now() < deadline) {
        if (pid == null || !pidAlive(pid)) break;
        await sleep(100);
      }

      if (pid != null && pidAlive(pid)) {
        try {
          process.kill(pid, "SIGKILL");
        } catch {
          /* ignore */
        }
        if (child && child.exitCode === null) {
          try {
            child.kill("SIGKILL");
          } catch {
            /* ignore */
          }
        }
        await sleep(100);
      }

      slot.child = null;
      slot.pid = null;
      slot.spawnToken = null;
      slot.state = "stopped";
      slot.startedAt = null;
      this.clearPidArtifacts(slaveId);
      try {
        fs.unlinkSync(childConfigPath(this.masterConfigPath, slaveId));
      } catch {
        /* ignore */
      }
      return this.snapshot(slaveId);
    } catch (err) {
      slot.state = "error";
      slot.lastError = err instanceof Error ? err.message : String(err);
      throw err;
    }
  }
}
