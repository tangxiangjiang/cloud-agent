// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

export type WorkflowNodeStatus =
  | "pending"
  | "ready"
  | "running"
  | "awaiting_review"
  | "approved"
  | "rejected"
  | "failed"
  | "cancelled"
  | "skipped";

export interface WorkflowPrompt {
  mode?: "phase_file" | "inline" | string;
  extra?: string;
  inline?: string;
}

export interface WorkflowNode {
  id: string;
  phaseRef?: string | null;
  title?: string | null;
  dependsOn: string[];
  status: WorkflowNodeStatus | string;
  taskId?: string | null;
  unitId?: string | null;
  model?: string | null;
  dodChecks?: string[];
  onFailure?: "stop" | "skip" | "retry" | string | null;
  prompt?: WorkflowPrompt | null;
}

export interface ReviseEntry {
  nodeId: string;
  instruction: string;
  at: string;
  taskId?: string | null;
}

export interface WorkflowRun {
  id: string;
  bundleId: string;
  bundleRef?: string | null;
  slaveId?: string | null;
  repoId: string;
  progressDoc?: string | null;
  status: string;
  nodes: WorkflowNode[];
  reviseHistory?: ReviseEntry[];
  createdAt: string;
  updatedAt?: string | null;
}

export interface WorkflowReviseMessage {
  workflowId: string;
  nodeId: string;
  instruction: string;
}

export interface WorkflowReviewMessage {
  workflowId: string;
  nodeId: string;
  decision: "approve" | "reject" | string;
  comment?: string;
  /** When true, Gateway auto-approved (M10-P03); progress/commit still required. */
  autoApprove?: boolean;
}

/**
 * Nodes that may run: status===ready AND (no deps OR all deps approved).
 * Awaiting_review does not unlock dependents.
 */
export function schedulableNodes(nodes: WorkflowNode[]): WorkflowNode[] {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  return nodes.filter((n) => {
    if (n.status !== "ready") return false;
    for (const dep of n.dependsOn ?? []) {
      const d = byId.get(dep);
      if (!d || d.status !== "approved") return false;
    }
    return true;
  });
}
