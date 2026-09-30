// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import YAML from "yaml";
import { migrateSlaveConfigToMaster } from "./migrate.js";

describe("migrateSlaveConfigToMaster", () => {
  it("primary keeps old slaveId; others get slave_<repoId>", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "master-mig-"));
    const absA = path.join(dir, "proj-a");
    const absB = path.join(dir, "proj-b");
    fs.mkdirSync(absA);
    fs.mkdirSync(absB);
    const from = path.join(dir, "config.yaml");
    const to = path.join(dir, "master.config.yaml");
    fs.writeFileSync(
      from,
      YAML.stringify({
        gatewayUrl: "ws://127.0.0.1:8080/v1/slave/ws",
        slaveId: "slave_devpc",
        name: "pc",
        projects: [
          { id: "r_a", name: "a", cwd: absA },
          { id: "r_b", name: "b", cwd: absB },
        ],
      }),
      "utf8",
    );

    const { master, mapping } = migrateSlaveConfigToMaster({
      fromSlaveConfig: from,
      toMasterConfig: to,
    });

    assert.equal(master.slaves.length, 2);
    const primary = mapping.find((m) => m.isPrimary)!;
    assert.equal(primary.newSlaveId, "slave_devpc");
    const other = mapping.find((m) => !m.isPrimary)!;
    assert.equal(other.newSlaveId, "slave_r_b");
    assert.ok(fs.existsSync(to));
  });
});
