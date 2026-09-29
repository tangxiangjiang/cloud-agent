---
name: plan-from-roadmap
description: >-
  From doc/roadmaps milestones/phases, generate or update AI-executable plan
  bundles under ai/bundles and keep ai/ index consistent. Use when planning
  work for Local Slave, compiling milestones into DAG JSON, or refreshing
  execution units for the phone workflow.
---

# plan-from-roadmap

## 目标

把 `doc/roadmaps/` 里的 milestone/phase **收成** `ai/bundles/<id>.json`，供本机 Slave 调度，并经 Gateway 变成手机上的任务单元。

## 必读

- `ai/README.md`、`ai/SCHEMA.md`
- `doc/architecture.md`、`doc/workflow.md`、`doc/workflow-ingest.md`
- 目标 epic：`doc/roadmaps/cloud-agent/README.md`

## 步骤

1. 确认 milestone 范围（用户指定如 `M01`，或当前未完成项）。  
2. 阅读对应 `milestones/*.md` 与 `phases/Mxx-P*.md`。  
3. 生成/更新 `ai/bundles/<milestone-or-epic>.json`：  
   - `phaseRef` 指向现有 phase 文件（相对仓库根）  
   - `dependsOn` 按 milestone 内顺序或文中依赖；**无环**  
   - `progressDoc`: `ai/progress.md`  
   - `prompt.mode`: `phase_file`  
4. 校验：每个 `phaseRef` 文件存在；`SCHEMA.md` 必填字段齐全。  
5. **更新计划索引** `ai/index.json`（及可读摘要 `ai/INDEX.md`）：  
   - 新增/更新一条 `plans[]`：`id`（短名）、`title`、`bundle`（相对仓库根或外仓相对路径）、`bundleId`、`repoId`、`roadmapRef`、`progressDoc`、`tags`  
   - 外仓工程可加 `cwdHint`（Slave `repos[].cwd`）  
   - 若用户指定，可改 `default`  
6. 可选：更新 `ai/bundles/README.md` 一句「当前计划」。  
7. **不要**自动 start 工作流、不要跳过 App 审核、不要写 `ai/units/`（那是 Slave 的事）。  
8. 向用户展示：`plan id`、bundle 路径、节点列表，以及启动命令：  
   `python run.py up --plan <id>`（或 `--agent --plan <id>`）。

## 索引约定

权威文件：`ai/index.json`。人读：`ai/INDEX.md`。  
`run.py plans` 列出；启动只认 **plan id**，不必手写 JSON 路径。

## 禁止

- Cloud Agent / 云端整仓  
- 把 `CURSOR_API_KEY` 写入任何文件  
- 在 App 可编辑源码的假设下设计字段  
- 删改 phase 设计正文除非用户明确要求微调 DoD

## 安装到 Cursor

本文件位于 `ai/skills/plan-from-roadmap/SKILL.md`。  
要让 Agent 自动发现，复制或链接到项目 `.cursor/skills/plan-from-roadmap/SKILL.md`。
