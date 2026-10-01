// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import path from "node:path";
import { describe, it } from "node:test";
import {
  ConfigError,
  isAllowedCwd,
  isSyncAiSummaryEnabled,
  validateConfig,
} from "./config.js";

const absA = path.resolve("/tmp/repo-a");
const absB = path.resolve("/tmp/repo-b");

describe("validateConfig", () => {
  it("accepts absolute cwd whitelist", () => {
    const cfg = validateConfig({
      gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
      slaveId: "slave_devpc",
      repos: [{ id: "r1", name: "a", cwd: absA }],
    });
    assert.equal(cfg.apiKeyEnv, "CURSOR_API_KEY");
    assert.equal(cfg.repos[0]?.cwd, path.normalize(absA));
    assert.equal(isAllowedCwd(cfg, absA), true);
    assert.equal(isAllowedCwd(cfg, absB), false);
  });

  it("rejects relative cwd", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          repos: [{ id: "r1", name: "a", cwd: "./relative" }],
        }),
      (e: unknown) => e instanceof ConfigError && /absolute path/.test(e.message),
    );
  });

  it("rejects cloud key", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          cloud: {},
          repos: [{ id: "r1", name: "a", cwd: absA }],
        }),
      (e: unknown) => e instanceof ConfigError && /cloud is not allowed/.test(e.message),
    );
  });

  it("rejects empty repos", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          repos: [],
        }),
      ConfigError,
    );
  });

  it("rejects apiKey literal in config", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          apiKey: "cursor_should_not_be_here",
          repos: [{ id: "r1", name: "a", cwd: absA }],
        }),
      (e: unknown) => e instanceof ConfigError && /apiKey must not appear/.test(e.message),
    );
  });

  it("accepts projects with relative index", () => {
    const cfg = validateConfig({
      gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
      slaveId: "slave_devpc",
      projects: [
        {
          id: "r1",
          name: "cloud-agent",
          cwd: absA,
          index: "ai/milestones.json",
        },
      ],
    });
    assert.equal(cfg.projects[0]?.index, "ai/milestones.json");
    assert.equal(cfg.repos[0]?.id, "r1");
    assert.equal(cfg.syncAiSummary, true);
  });

  it("honors syncAiSummary false and SYNC_AI_SUMMARY env", () => {
    const cfg = validateConfig({
      gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
      slaveId: "slave_devpc",
      syncAiSummary: false,
      projects: [{ id: "r1", name: "a", cwd: absA }],
    });
    assert.equal(cfg.syncAiSummary, false);
    assert.equal(isSyncAiSummaryEnabled(cfg), false);

    const prev = process.env.SYNC_AI_SUMMARY;
    try {
      process.env.SYNC_AI_SUMMARY = "0";
      const on = validateConfig({
        gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
        slaveId: "slave_devpc",
        syncAiSummary: true,
        projects: [{ id: "r1", name: "a", cwd: absA }],
      });
      assert.equal(isSyncAiSummaryEnabled(on), false);
      process.env.SYNC_AI_SUMMARY = "1";
      const forced = validateConfig({
        gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
        slaveId: "slave_devpc",
        syncAiSummary: false,
        projects: [{ id: "r1", name: "a", cwd: absA }],
      });
      assert.equal(isSyncAiSummaryEnabled(forced), true);
    } finally {
      if (prev === undefined) delete process.env.SYNC_AI_SUMMARY;
      else process.env.SYNC_AI_SUMMARY = prev;
    }
  });

  it("defaults defaultModel/autoModelId to default and optimizeFor to cost", () => {
    const cfg = validateConfig({
      gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
      slaveId: "slave_devpc",
      repos: [{ id: "r1", name: "a", cwd: absA }],
    });
    assert.equal(cfg.defaultModel, "default");
    assert.equal(cfg.autoModelId, "default");
    assert.equal(cfg.optimizeFor, "cost");
    assert.equal(cfg.taskTimeoutMs, 3_600_000);
    assert.equal(cfg.idleTimeoutMs, 600_000);
  });

  it("accepts hang timeouts including 0 to disable", () => {
    const cfg = validateConfig({
      gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
      slaveId: "slave_devpc",
      taskTimeoutMs: 0,
      idleTimeoutMs: 120_000,
      repos: [{ id: "r1", name: "a", cwd: absA }],
    });
    assert.equal(cfg.taskTimeoutMs, 0);
    assert.equal(cfg.idleTimeoutMs, 120_000);
  });

  it("rejects negative hang timeouts", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          taskTimeoutMs: -1,
          repos: [{ id: "r1", name: "a", cwd: absA }],
        }),
      (e: unknown) => e instanceof ConfigError && /taskTimeoutMs/.test(e.message),
    );
  });

  it("rejects invalid optimizeFor", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          optimizeFor: "default",
          repos: [{ id: "r1", name: "a", cwd: absA }],
        }),
      (e: unknown) => e instanceof ConfigError && /optimizeFor/.test(e.message),
    );
  });

  it("rejects absolute or parent index path", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          projects: [
            { id: "r1", name: "a", cwd: absA, index: "../escape.json" },
          ],
        }),
      (e: unknown) => e instanceof ConfigError && /relative path/.test(e.message),
    );
  });

  it("rejects multi-project without allowMultiProject", () => {
    assert.throws(
      () =>
        validateConfig({
          gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
          slaveId: "slave_devpc",
          projects: [
            { id: "r1", name: "a", cwd: absA },
            { id: "r2", name: "b", cwd: absB },
          ],
        }),
      (e: unknown) =>
        e instanceof ConfigError && /exactly one entry/.test(e.message),
    );
  });

  it("allows multi-project with allowMultiProject", () => {
    const cfg = validateConfig(
      {
        gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
        slaveId: "slave_devpc",
        projects: [
          { id: "r1", name: "a", cwd: absA },
          { id: "r2", name: "b", cwd: absB },
        ],
      },
      { allowMultiProject: true },
    );
    assert.equal(cfg.projects.length, 2);
  });
});
