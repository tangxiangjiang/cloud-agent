// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { countDiffStats, parseNameStatus, splitUnifiedByFile } from "./diff.js";

describe("diff helpers", () => {
  it("parses name-status", () => {
    const rows = parseNameStatus("A\tfoo.md\nM\tbar.ts\nD\told.txt\n");
    assert.deepEqual(rows, [
      { path: "foo.md", status: "added" },
      { path: "bar.ts", status: "modified" },
      { path: "old.txt", status: "deleted" },
    ]);
  });

  it("splits unified diff by file", () => {
    const text = `diff --git a/a.md b/a.md
--- a/a.md
+++ b/a.md
@@ -1 +1 @@
-old
+new
diff --git a/b.md b/b.md
--- a/b.md
+++ b/b.md
@@ -0,0 +1 @@
+hi
`;
    const map = splitUnifiedByFile(text);
    assert.equal(map.size, 2);
    assert.ok(map.get("a.md")?.includes("+new"));
    assert.ok(map.get("b.md")?.includes("+hi"));
  });

  it("counts additions/deletions", () => {
    const { additions, deletions } = countDiffStats(
      "--- a/x\n+++ b/x\n@@ -1 +1,2 @@\n-a\n+b\n+c\n",
    );
    assert.equal(additions, 2);
    assert.equal(deletions, 1);
  });
});
