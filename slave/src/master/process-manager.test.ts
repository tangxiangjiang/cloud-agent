// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import { saveMasterConfig, validateMasterConfig } from "./config.js";
import { MasterControlError } from "./errors.js";
import { ProcessManager } from "./process-manager.js";
import { childPidPath, childSpawnTokenPath } from "./child-config.js";

const absA = path.resolve("/tmp/ws-a-master-pm");
const absB = path.resolve("/tmp/ws-b-master-pm");

function makeCfg(dir: string, extras: Record<string, unknown> = {}) {
  const cfg = validateMasterConfig({
    masterId: "master_test",
    gatewayUrl: "ws://127.0.0.1:8080/v1/master/ws",
    slaveCommand: [process.execPath, "-e", "setInterval(()=>{}, 60000)"],
    slaveCwd: dir,
    gracePeriodMs: 500,
    maxRunningSlaves: 2,
    maxConcurrentStarts: 1,
    slaves: [
      {
        id: "slave_a",
        enabled: true,
        project: { id: "r_a", name: "a", cwd: absA },
      },
      {
        id: "slave_b",
        enabled: true,
        project: { id: "r_b", name: "b", cwd: absB },
      },
    ],
    ...extras,
  });
  const configPath = path.join(dir, "master.config.yaml");
  saveMasterConfig(configPath, cfg);
  return { cfg, configPath };
}

describe("ProcessManager", () => {
  it("cold start: all stopped, no auto start", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "master-pm-"));
    const { cfg, configPath } = makeCfg(dir);
    const pm = new ProcessManager(cfg, { masterConfigPath: configPath });
    const snaps = pm.listSnapshots();
    assert.equal(snaps.length, 2);
    assert.ok(snaps.every((s) => s.state === "stopped"));
    assert.ok(snaps.every((s) => s.pid == null));
  });

  it("start/stop one slave without affecting the other", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "master-pm-"));
    const { cfg, configPath } = makeCfg(dir);
    const pm = new ProcessManager(cfg, { masterConfigPath: configPath });
    const a = await pm.start("slave_a");
    assert.equal(a.state, "running");
    assert.ok(a.pid && a.pid > 0);
    assert.equal(pm.snapshot("slave_b").state, "stopped");

    const stopped = await pm.stop("slave_a");
    assert.equal(stopped.state, "stopped");
    assert.equal(pm.snapshot("slave_b").state, "stopped");
  });

  it("rejects start when disabled", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "master-pm-"));
    const { cfg, configPath } = makeCfg(dir, {
      slaves: [
        {
          id: "slave_a",
          enabled: false,
          project: { id: "r_a", name: "a", cwd: absA },
        },
      ],
    });
    const pm = new ProcessManager(cfg, { masterConfigPath: configPath });
    await assert.rejects(
      () => pm.start("slave_a"),
      (e: unknown) =>
        e instanceof MasterControlError && e.code === "slave_disabled",
    );
  });

  it("adopts live pid without second spawn", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "master-pm-"));
    const { cfg, configPath } = makeCfg(dir);
    const pm1 = new ProcessManager(cfg, { masterConfigPath: configPath });
    const first = await pm1.start("slave_a");
    assert.equal(first.state, "running");
    const pid = first.pid!;

    // Simulate Master restart: new manager, adopt orphan.
    const pm2 = new ProcessManager(cfg, { masterConfigPath: configPath });
    pm2.adoptOrphans();
    const adopted = pm2.snapshot("slave_a");
    assert.equal(adopted.state, "running");
    assert.equal(adopted.pid, pid);

    const again = await pm2.start("slave_a");
    assert.equal(again.pid, pid);

    await pm2.stop("slave_a");
  });

  it("writes state under .local/slaves/<id>/", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "master-pm-"));
    const { cfg, configPath } = makeCfg(dir);
    const pm = new ProcessManager(cfg, { masterConfigPath: configPath });
    await pm.start("slave_a");
    assert.ok(fs.existsSync(childPidPath(configPath, "slave_a")));
    assert.ok(fs.existsSync(childSpawnTokenPath(configPath, "slave_a")));
    await pm.stop("slave_a");
  });
});
