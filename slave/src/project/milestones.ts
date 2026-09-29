// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import { readFileSync } from "node:fs";
import path from "node:path";

export interface MilestonePhase {
  id: string;
  title: string;
  phaseRef: string;
  dependsOn: string[];
  model?: string;
  onFailure?: string;
  prompt?: {
    mode?: string;
    extra?: string;
    inline?: string;
  };
}

export interface Milestone {
  id: string;
  title: string;
  progressDoc?: string;
  phases: MilestonePhase[];
}

export interface ProjectIndex {
  schemaVersion: number;
  milestones: Milestone[];
}

export class MilestoneIndexError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "MilestoneIndexError";
  }
}

/**
 * Load project milestone index. `indexRel` is relative to project cwd.
 * Missing file → empty milestones (slave still registers the project).
 */
export function loadProjectMilestones(
  projectCwd: string,
  indexRel: string | undefined,
): { indexRel: string | null; milestones: Milestone[]; error?: string } {
  const rel = (indexRel ?? "").trim();
  if (!rel) {
    return { indexRel: null, milestones: [] };
  }
  if (path.isAbsolute(rel) || rel.split(/[/\\]/).includes("..")) {
    return {
      indexRel: rel,
      milestones: [],
      error: "index must be a relative path without ..",
    };
  }
  const abs = path.resolve(projectCwd, rel);
  const root = path.resolve(projectCwd);
  if (!abs.startsWith(root + path.sep) && abs !== root) {
    return {
      indexRel: rel,
      milestones: [],
      error: "index escapes project cwd",
    };
  }
  let text: string;
  try {
    text = readFileSync(abs, "utf8");
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { indexRel: rel, milestones: [], error: `cannot read index: ${msg}` };
  }
  try {
    const parsed = JSON.parse(text) as unknown;
    const milestones = parseMilestones(parsed);
    return { indexRel: rel.replace(/\\/g, "/"), milestones };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { indexRel: rel, milestones: [], error: msg };
  }
}

export function parseMilestones(raw: unknown): Milestone[] {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
    throw new MilestoneIndexError("index root must be an object");
  }
  const obj = raw as { milestones?: unknown };
  if (!Array.isArray(obj.milestones)) {
    throw new MilestoneIndexError("milestones must be an array");
  }
  const out: Milestone[] = [];
  for (let i = 0; i < obj.milestones.length; i++) {
    const m = obj.milestones[i];
    if (m === null || typeof m !== "object" || Array.isArray(m)) {
      throw new MilestoneIndexError(`milestones[${i}] must be an object`);
    }
    const mm = m as Record<string, unknown>;
    const id = String(mm.id ?? "").trim();
    const title = String(mm.title ?? "").trim() || id;
    if (!id) throw new MilestoneIndexError(`milestones[${i}].id required`);
    if (!Array.isArray(mm.phases) || mm.phases.length === 0) {
      throw new MilestoneIndexError(`milestones[${i}].phases required`);
    }
    const phases: MilestonePhase[] = [];
    for (let j = 0; j < mm.phases.length; j++) {
      const p = mm.phases[j];
      if (p === null || typeof p !== "object" || Array.isArray(p)) {
        throw new MilestoneIndexError(`milestones[${i}].phases[${j}] invalid`);
      }
      const pp = p as Record<string, unknown>;
      const pid = String(pp.id ?? "").trim();
      const phaseRef = String(pp.phaseRef ?? "").trim();
      if (!pid) throw new MilestoneIndexError(`phase id required at [${i}][${j}]`);
      if (!phaseRef) {
        throw new MilestoneIndexError(`phaseRef required for ${pid}`);
      }
      const dependsOn = Array.isArray(pp.dependsOn)
        ? pp.dependsOn.map((d) => String(d))
        : [];
      const phase: MilestonePhase = {
        id: pid,
        title: String(pp.title ?? "").trim() || pid,
        phaseRef,
        dependsOn,
      };
      if (typeof pp.model === "string" && pp.model.trim()) {
        phase.model = pp.model.trim();
      }
      if (typeof pp.onFailure === "string" && pp.onFailure.trim()) {
        phase.onFailure = pp.onFailure.trim();
      }
      if (pp.prompt && typeof pp.prompt === "object" && !Array.isArray(pp.prompt)) {
        const pr = pp.prompt as Record<string, unknown>;
        phase.prompt = {
          mode: typeof pr.mode === "string" ? pr.mode : "phase_file",
          ...(typeof pr.extra === "string" ? { extra: pr.extra } : {}),
          ...(typeof pr.inline === "string" ? { inline: pr.inline } : {}),
        };
      } else {
        phase.prompt = { mode: "phase_file" };
      }
      phases.push(phase);
    }
    const milestone: Milestone = { id, title, phases };
    if (typeof mm.progressDoc === "string" && mm.progressDoc.trim()) {
      milestone.progressDoc = mm.progressDoc.trim();
    }
    out.push(milestone);
  }
  return out;
}
