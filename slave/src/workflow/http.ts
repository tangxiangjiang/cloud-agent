// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { AssignedTask } from "../gateway/types.js";
import type { WorkflowNode, WorkflowRun } from "./types.js";

export function gatewayHttpBase(wsUrl: string): string {
  const u = new URL(wsUrl);
  u.protocol = u.protocol === "wss:" ? "https:" : "http:";
  u.pathname = "";
  u.search = "";
  u.hash = "";
  // URL with empty pathname may still end with /
  return u.origin;
}

export class GatewayHttpApi {
  constructor(
    private readonly baseUrl: string,
    private readonly token: string,
  ) {}

  async createTask(input: {
    slaveId: string;
    repoId: string;
    prompt: string;
    model?: string;
    workflowId: string;
    nodeId: string;
  }): Promise<AssignedTask> {
    const res = await fetch(`${this.baseUrl}/v1/tasks`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${this.token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        slaveId: input.slaveId,
        repoId: input.repoId,
        prompt: input.prompt,
        model: input.model ?? "",
        workflowId: input.workflowId,
        nodeId: input.nodeId,
      }),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`createTask ${res.status}: ${text}`);
    }
    return (await res.json()) as AssignedTask;
  }

  async patchNode(
    workflowId: string,
    nodeId: string,
    patch: { status?: string; taskId?: string | null; unitId?: string | null },
  ): Promise<WorkflowRun> {
    const res = await fetch(
      `${this.baseUrl}/v1/workflows/${encodeURIComponent(workflowId)}/nodes/${encodeURIComponent(nodeId)}`,
      {
        method: "PATCH",
        headers: {
          Authorization: `Bearer ${this.token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(patch),
      },
    );
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`patchNode ${res.status}: ${text}`);
    }
    return (await res.json()) as WorkflowRun;
  }

  async getWorkflow(workflowId: string): Promise<WorkflowRun> {
    const res = await fetch(
      `${this.baseUrl}/v1/workflows/${encodeURIComponent(workflowId)}`,
      {
        headers: { Authorization: `Bearer ${this.token}` },
      },
    );
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`getWorkflow ${res.status}: ${text}`);
    }
    return (await res.json()) as WorkflowRun;
  }
}

export type { WorkflowNode };
