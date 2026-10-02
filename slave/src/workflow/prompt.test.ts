// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import {
  buildNodePrompt,
  buildPhaseExecutionPreamble,
  buildRevisePrompt,
} from "./prompt.js";
import type { WorkflowNode } from "./types.js";

function node(partial: Partial<WorkflowNode> & { id: string }): WorkflowNode {
  return {
    dependsOn: [],
    status: "ready",
    ...partial,
  };
}

describe("buildPhaseExecutionPreamble", () => {
  it("includes identity and hard rules", () => {
    const p = buildPhaseExecutionPreamble(
      node({
        id: "M12-P01",
        title: "Interface",
        phaseRef: "docs/p.md",
        dodChecks: ["go test ./..."],
      }),
    );
    assert.match(p, /Single phase only/);
    assert.match(p, /Finish thoroughly/);
    assert.match(p, /NO stubs \/ MVP shortcuts/);
    assert.match(p, /Port \/ copy existing code when cited/);
    assert.match(p, /nodeId: `M12-P01`/);
    assert.match(p, /phaseRef: `docs\/p\.md`/);
    assert.match(p, /go test \.\/\.\.\./);
  });
});

describe("buildNodePrompt", () => {
  it("wraps phase file with contract and path header", () => {
    const dir = mkdtempSync(path.join(tmpdir(), "phase-prompt-"));
    writeFileSync(
      path.join(dir, "phase.md"),
      "# Goal\n\n## 完成定义（DoD）\n\n- [ ] ship it\n",
      "utf8",
    );
    const prompt = buildNodePrompt(
      node({
        id: "M01-P01",
        title: "Ship",
        phaseRef: "phase.md",
      }),
      dir,
    );
    assert.match(prompt, /Workflow phase execution contract/);
    assert.match(prompt, /Phase document \(source of truth/);
    assert.match(prompt, /path: `phase\.md`/);
    assert.match(prompt, /完成定义/);
    assert.match(prompt, /human review is required/);
    assert.match(prompt, /Pre-stop self-check/);
    assert.match(prompt, /stubs\/MVP/);
    assert.ok(prompt.indexOf("execution contract") < prompt.indexOf("Phase document"));
  });

  it("rejects phaseRef escape", () => {
    assert.throws(
      () =>
        buildNodePrompt(
          node({ id: "X", phaseRef: "../outside.md" }),
          mkdtempSync(path.join(tmpdir(), "phase-esc-")),
        ),
      /escapes/,
    );
  });
});

describe("buildRevisePrompt", () => {
  it("keeps contract and revise instruction", () => {
    const p = buildRevisePrompt(
      node({ id: "N1", title: "T", phaseRef: "a.md" }),
      "fix the failing test",
    );
    assert.match(p, /execution contract/);
    assert.match(p, /fix the failing test/);
    assert.match(p, /within this node's phase scope/);
  });
});
