// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { WorkflowRun } from "../workflow/types.js";

/** Task payload as delivered in Gateway → Slave `task.assign`. */
export interface AssignedTask {
  id: string;
  status?: string;
  slaveId?: string | null;
  repoId?: string | null;
  prompt?: string | null;
  model?: string | null;
  agentId?: string | null;
  /** When set, Local Agent prefers Agent.resume then send (revise follow-up). */
  resumeAgentId?: string | null;
  workflowId?: string | null;
  nodeId?: string | null;
  createdAt?: string;
  updatedAt?: string | null;
  error?: { code?: string; message?: string } | null;
}

export type EventKind =
  | "status"
  | "assistant.delta"
  | "error"
  | "done"
  | string;

export type EmitEvent = (
  taskId: string,
  kind: EventKind,
  payload: Record<string, unknown>,
) => void;

export interface TaskHandlers {
  onAssign(task: AssignedTask, emit: EmitEvent): void | Promise<void>;
  onCancel(taskId: string, emit: EmitEvent): void | Promise<void>;
}

export type InboundMessage =
  | { type: "auth.ok" }
  | { type: "registered"; slaveId?: string }
  | { type: "heartbeat.ok" }
  | { type: "pong" }
  | { type: "task.assign"; task: AssignedTask }
  | { type: "task.cancel"; taskId: string }
  | { type: "workflow.assign"; workflow: WorkflowRun }
  | {
      type: "workflow.revise";
      workflowId: string;
      nodeId: string;
      instruction: string;
    }
  | {
      type: "workflow.review";
      workflowId: string;
      nodeId: string;
      decision: "approve" | "reject" | string;
      comment?: string;
    }
  | { type: "project.sync"; requestId?: string; repoId: string }
  | { type: "project.sync.ok"; requestId?: string }
  | { type: "error"; error?: string }
  | { type: string; [k: string]: unknown };

export type { WorkflowRun };
