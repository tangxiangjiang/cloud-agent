// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { describe, it } from "node:test";
import type { SlaveConfig } from "../config.js";
import type { AssignedTask, EmitEvent } from "../gateway/types.js";
import type { NodeDiffPayload } from "./diff.js";
import { GatewayHttpApi } from "./http.js";
import { buildRevisePrompt } from "./prompt.js";
import { SerialDagScheduler } from "./scheduler.js";
import type { WorkflowRun } from "./types.js";

describe("buildRevisePrompt", () => {
  it("includes instruction and never-approve reminder", () => {
    const p = buildRevisePrompt(
      { id: "N1", dependsOn: [], status: "running", title: "Fix API" },
      "把错误码改掉",
    );
    assert.match(p, /N1/);
    assert.match(p, /Fix API/);
    assert.match(p, /把错误码改掉/);
    assert.match(p, /Do not mark the workflow node approved/);
  });
});

describe("SerialDagScheduler.enqueueRevise", () => {
  it("re-runs agent, refreshes diff, returns awaiting_review (never approved)", async () => {
    const cwd = mkdtempSync(path.join(tmpdir(), "ca-revise-"));
    execFileSync("git", ["init"], { cwd, windowsHide: true });
    execFileSync("git", ["config", "user.email", "t@example.com"], {
      cwd,
      windowsHide: true,
    });
    execFileSync("git", ["config", "user.name", "t"], { cwd, windowsHide: true });
    writeFileSync(path.join(cwd, "a.txt"), "v1\n");
    execFileSync("git", ["add", "a.txt"], { cwd, windowsHide: true });
    execFileSync("git", ["commit", "-m", "init"], { cwd, windowsHide: true });
    const sha = execFileSync("git", ["rev-parse", "HEAD"], {
      cwd,
      encoding: "utf8",
      windowsHide: true,
    }).trim();

    const cfg: SlaveConfig = {
      gatewayUrl: "ws://127.0.0.1:9/v1/slave/ws",
      slaveId: "s1",
      tokenEnv: "GATEWAY_TOKEN",
      apiKeyEnv: "CURSOR_API_KEY",
      defaultModel: "composer-2.5",
      optimizeFor: "balanced",
      syncAiSummary: true,
      projects: [{ id: "r1", name: "t", cwd }],
      repos: [{ id: "r1", name: "t", cwd }],
    };

    let run: WorkflowRun = {
      id: "wf_1",
      bundleId: "b",
      repoId: "r1",
      slaveId: "s1",
      status: "running",
      nodes: [
        {
          id: "N1",
          dependsOn: [],
          status: "running",
          title: "node",
          prompt: { mode: "inline", inline: "noop" },
        },
      ],
      createdAt: new Date().toISOString(),
      reviseHistory: [
        {
          nodeId: "N1",
          instruction: "改成 v2",
          at: new Date().toISOString(),
        },
      ],
    };

    const patches: Array<{ status?: string; taskId?: string | null }> = [];
    let putDiff: NodeDiffPayload | null = null;
    let taskSeq = 0;

    const http = {
      async getWorkflow() {
        return run;
      },
      async createTask() {
        taskSeq += 1;
        return {
          id: `tsk_rev_${taskSeq}`,
          workflowId: "wf_1",
          nodeId: "N1",
          repoId: "r1",
          slaveId: "s1",
          prompt: "revise",
        } satisfies AssignedTask;
      },
      async patchNode(
        _wf: string,
        nodeId: string,
        patch: { status?: string; taskId?: string | null },
      ) {
        patches.push(patch);
        const n = run.nodes.find((x) => x.id === nodeId)!;
        if (patch.status) n.status = patch.status;
        if (patch.taskId !== undefined) n.taskId = patch.taskId;
        return run;
      },
      async putNodeDiff(diff: NodeDiffPayload) {
        putDiff = diff;
        return diff;
      },
      async getNodeDiff() {
        return {
          workflowId: "wf_1",
          nodeId: "N1",
          baseline: `git:${sha}`,
          files: [],
        } satisfies NodeDiffPayload;
      },
    } as unknown as GatewayHttpApi;

    const scheduler = new SerialDagScheduler({
      cfg,
      http,
      handlers: {
        async onAssign(task, emit: EmitEvent) {
          // Simulate revise follow-up editing the file.
          writeFileSync(path.join(cwd, "a.txt"), "v2\n");
          assert.ok(task.resumeAgentId === undefined || typeof task.resumeAgentId === "string");
          emit(task.id, "done", { status: "finished", agentId: "agent_rev_1" });
        },
        async onCancel() {},
      },
      emit: () => {},
    });

    // Seed baseline as if first run captured it.
    (scheduler as unknown as { baselines: Map<string, string | null> }).baselines.set(
      "wf_1\0N1",
      `git:${sha}`,
    );

    await scheduler.enqueueRevise({
      workflowId: "wf_1",
      nodeId: "N1",
      instruction: "改成 v2",
    });

    assert.ok(patches.some((p) => p.status === "running" && p.taskId === "tsk_rev_1"));
    assert.ok(patches.some((p) => p.status === "awaiting_review"));
    assert.ok(!patches.some((p) => p.status === "approved"));
    assert.equal(run.nodes[0]?.status, "awaiting_review");
    assert.ok(putDiff);
    assert.equal(putDiff!.baseline, `git:${sha}`);
    assert.ok(putDiff!.files.some((f) => f.path === "a.txt"));
    // Audit on run must still hold instruction (Gateway responsibility; Slave must not clear it).
    assert.equal(run.reviseHistory?.[0]?.instruction, "改成 v2");
  });
});
