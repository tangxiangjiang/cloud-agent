// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { schedulableNodes, type WorkflowNode } from "./types.js";

describe("schedulableNodes", () => {
  it("allows root ready with empty dependsOn", () => {
    const nodes: WorkflowNode[] = [
      { id: "A", dependsOn: [], status: "ready" },
      { id: "B", dependsOn: ["A"], status: "pending" },
    ];
    const ready = schedulableNodes(nodes);
    assert.equal(ready.length, 1);
    assert.equal(ready[0]?.id, "A");
  });

  it("does not schedule B while A is awaiting_review", () => {
    const nodes: WorkflowNode[] = [
      { id: "A", dependsOn: [], status: "awaiting_review", taskId: "tsk_1" },
      { id: "B", dependsOn: ["A"], status: "pending" },
    ];
    assert.deepEqual(
      schedulableNodes(nodes).map((n) => n.id),
      [],
    );
  });

  it("schedules B only after A approved", () => {
    const nodes: WorkflowNode[] = [
      { id: "A", dependsOn: [], status: "approved", taskId: "tsk_1" },
      { id: "B", dependsOn: ["A"], status: "ready" },
    ];
    assert.deepEqual(
      schedulableNodes(nodes).map((n) => n.id),
      ["B"],
    );
  });
});
