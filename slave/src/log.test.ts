// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { redact, sanitizeFields } from "./log.js";

describe("log redaction", () => {
  it("redacts apiKey / token field values", () => {
    const out = sanitizeFields({
      apiKey: "cursor_super_secret_value_here",
      token: "gateway-bearer-token-value-abc",
      taskId: "task_1",
    });
    assert.equal(out.apiKey, "***");
    assert.equal(out.token, "***");
    assert.equal(out.taskId, "task_1");
  });

  it("redacts long opaque strings in free text", () => {
    const secret = "abcdefghij1234567890XYZ_secret";
    assert.equal(redact(secret), "***");
  });
});
