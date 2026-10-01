// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import path from "node:path";
import { describe, it } from "node:test";
import {
  MasterConfigError,
  upsertSlaveEntry,
  validateMasterConfig,
} from "./config.js";

const absA = path.resolve("/tmp/ws-a");
const absB = path.resolve("/tmp/ws-b");

function baseRaw(overrides: Record<string, unknown> = {}) {
  return {
    masterId: "master_dev",
    gatewayUrl: "ws://127.0.0.1:8080/v1/master/ws",
    slaves: [
      {
        id: "slave_a",
        enabled: true,
        project: { id: "r_a", name: "a", cwd: absA },
      },
    ],
    ...overrides,
  };
}

describe("validateMasterConfig", () => {
  it("accepts minimal config and derives slaveGatewayUrl", () => {
    const cfg = validateMasterConfig(baseRaw());
    assert.equal(cfg.masterId, "master_dev");
    assert.equal(cfg.maxRunningSlaves, 4);
    assert.equal(cfg.defaults.slaveGatewayUrl, "ws://127.0.0.1:8080/v1/slave/ws");
    assert.equal(cfg.defaults.taskTimeoutMs, 3_600_000);
    assert.equal(cfg.defaults.idleTimeoutMs, 600_000);
    assert.equal(cfg.slaves[0]?.enabled, true);
  });

  it("honors defaults hang timeouts including 0", () => {
    const cfg = validateMasterConfig(
      baseRaw({
        defaults: { taskTimeoutMs: 0, idleTimeoutMs: 0 },
      }),
    );
    assert.equal(cfg.defaults.taskTimeoutMs, 0);
    assert.equal(cfg.defaults.idleTimeoutMs, 0);
  });

  it("rejects duplicate project id", () => {
    assert.throws(
      () =>
        validateMasterConfig(
          baseRaw({
            slaves: [
              {
                id: "slave_a",
                project: { id: "r_a", name: "a", cwd: absA },
              },
              {
                id: "slave_b",
                project: { id: "r_a", name: "b", cwd: absB },
              },
            ],
          }),
        ),
      (e: unknown) =>
        e instanceof MasterConfigError && /repo_conflict/.test(e.message),
    );
  });

  it("rejects cwd outside allowedRoots", () => {
    assert.throws(
      () =>
        validateMasterConfig(
          baseRaw({
            allowedRoots: [path.resolve("/tmp/other")],
          }),
        ),
      (e: unknown) =>
        e instanceof MasterConfigError && /cwd_denied/.test(e.message),
    );
  });

  it("upsert rejects conflicting cwd", () => {
    const cfg = validateMasterConfig(
      baseRaw({
        slaves: [
          {
            id: "slave_a",
            project: { id: "r_a", name: "a", cwd: absA },
          },
          {
            id: "slave_b",
            project: { id: "r_b", name: "b", cwd: absB },
          },
        ],
      }),
    );
    assert.throws(
      () =>
        upsertSlaveEntry(cfg, {
          id: "slave_c",
          enabled: true,
          project: { id: "r_c", name: "c", cwd: absA },
        }),
      (e: unknown) =>
        e instanceof MasterConfigError && /repo_conflict/.test(e.message),
    );
  });
});
