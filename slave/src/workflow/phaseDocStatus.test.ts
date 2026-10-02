// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import {
  markMarkdownStatusField,
  statusLabelForLifecycle,
  writePhaseDocStatusOnLifecycle,
} from "./phaseDocStatus.js";

describe("markMarkdownStatusField", () => {
  it("updates **Status:** Pending → Done on approve", () => {
    const src = "# X\n\n**Status:** Pending  \n\n## Goal\n";
    const r = markMarkdownStatusField(src, "approved");
    assert.equal(r.changed, true);
    assert.match(r.next, /\*\*Status:\*\* Done/);
    assert.equal(r.previous, "Pending");
  });

  it("updates Status to Ready for human review", () => {
    const src = "**Status:** Planned\n";
    const r = markMarkdownStatusField(src, "awaiting_review");
    assert.equal(r.changed, true);
    assert.match(r.next, /\*\*Status:\*\* Ready for human review/);
  });

  it("preserves lowercase done style", () => {
    assert.equal(statusLabelForLifecycle("approved", "pending"), "done");
    const src = "**Status:** pending\n";
    const r = markMarkdownStatusField(src, "approved");
    assert.match(r.next, /\*\*Status:\*\* done/);
  });

  it("no-ops when no Status field", () => {
    const r = markMarkdownStatusField("# no status\n", "approved");
    assert.equal(r.changed, false);
  });
});

describe("writePhaseDocStatusOnLifecycle", () => {
  it("writes phaseRef Status under cwd", () => {
    const root = mkdtempSync(path.join(tmpdir(), "phase-status-"));
    mkdirSync(path.join(root, "docs"), { recursive: true });
    const rel = "docs/p.md";
    writeFileSync(
      path.join(root, rel),
      "# T\n\n**Status:** Pending\n\n## Work\n",
      "utf8",
    );
    const r = writePhaseDocStatusOnLifecycle({
      repoCwd: root,
      node: {
        id: "M05-P10",
        dependsOn: [],
        status: "awaiting_review",
        phaseRef: rel,
      },
      lifecycle: "awaiting_review",
    });
    assert.equal(r.written, true);
    assert.equal(r.path, rel);
    const text = readFileSync(path.join(root, rel), "utf8");
    assert.match(text, /\*\*Status:\*\* Ready for human review/);

    const r2 = writePhaseDocStatusOnLifecycle({
      repoCwd: root,
      node: {
        id: "M05-P10",
        dependsOn: [],
        status: "approved",
        phaseRef: rel,
      },
      lifecycle: "approved",
    });
    assert.equal(r2.written, true);
    const text2 = readFileSync(path.join(root, rel), "utf8");
    assert.match(text2, /\*\*Status:\*\* Done/);
  });
});
