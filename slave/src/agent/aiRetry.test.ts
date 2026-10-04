// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  aiRetryDelayMs,
  DEFAULT_AI_RETRY_BASE_DELAY_MS,
  isRetryableAiError,
} from "./aiRetry.js";

describe("isRetryableAiError", () => {
  it("honors SDK isRetryable flag", () => {
    assert.equal(isRetryableAiError({ isRetryable: true, message: "x" }), true);
    assert.equal(isRetryableAiError({ isRetryable: false, message: "ECONNRESET" }), false);
  });

  it("treats common network codes/messages as retryable", () => {
    assert.equal(isRetryableAiError({ code: "ECONNRESET" }), true);
    assert.equal(isRetryableAiError({ code: "ETIMEDOUT" }), true);
    assert.equal(isRetryableAiError({ message: "fetch failed" }), true);
    assert.equal(isRetryableAiError({ message: "socket hang up" }), true);
    assert.equal(isRetryableAiError({ status: 503, message: "unavailable" }), true);
    assert.equal(isRetryableAiError({ status: 429 }), true);
  });

  it("does not retry auth failures", () => {
    assert.equal(isRetryableAiError({ status: 401, message: "unauthorized" }), false);
    assert.equal(isRetryableAiError({ message: "Invalid API key" }), false);
    assert.equal(isRetryableAiError({ message: "authentication failed" }), false);
  });
});

describe("aiRetryDelayMs", () => {
  it("backs off from base and caps", () => {
    assert.equal(aiRetryDelayMs(2, 1000), 1000);
    assert.equal(aiRetryDelayMs(3, 1000), 2000);
    assert.equal(aiRetryDelayMs(4, 1000), 4000);
    assert.equal(aiRetryDelayMs(20, DEFAULT_AI_RETRY_BASE_DELAY_MS), 60_000);
  });
});
