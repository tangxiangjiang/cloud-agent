// Copyright (c) 2026 smarttang
// SPDX-License-Identifier: MIT

import type { ProjectConfig, SlaveConfig } from "../config.js";
import { log } from "../log.js";
import {
  loadProjectMilestones,
  type Milestone,
} from "./milestones.js";

/** Catalog entry pushed to Gateway on register / listed in GET /v1/slaves. */
export interface ProjectCatalog {
  id: string;
  name: string;
  cwd: string;
  index: string | null;
  milestones: Milestone[];
}

export function buildProjectCatalog(cfg: SlaveConfig): ProjectCatalog[] {
  return cfg.projects.map((p) => loadOne(p));
}

function loadOne(p: ProjectConfig): ProjectCatalog {
  const loaded = loadProjectMilestones(p.cwd, p.index);
  if (loaded.error) {
    log.warn("project milestone index issue", {
      projectId: p.id,
      index: p.index ?? null,
      error: loaded.error,
    });
  }
  return {
    id: p.id,
    name: p.name,
    cwd: p.cwd,
    index: loaded.indexRel,
    milestones: loaded.milestones,
  };
}

/** Flat repos view for legacy Gateway fields. */
export function projectsAsRepos(projects: ProjectCatalog[]): Array<{
  id: string;
  name: string;
  cwd: string;
}> {
  return projects.map((p) => ({ id: p.id, name: p.name, cwd: p.cwd }));
}
