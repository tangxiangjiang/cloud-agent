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
5. 更新 `ai/bundles/README.md` 或本索引中「当前计划」一句说明（若有）。  
6. **不要**自动 start 工作流、不要跳过 App 审核、不要写 `ai/units/`（那是 Slave 的事）。  
7. 向用户展示 bundle 路径与节点列表，等人确认。

## 禁止

- Cloud Agent / 云端整仓  
- 把 `CURSOR_API_KEY` 写入任何文件  
- 在 App 可编辑源码的假设下设计字段  
- 删改 phase 设计正文除非用户明确要求微调 DoD

## 源码位置

权威副本：`ai/skills/plan-from-roadmap/SKILL.md`（与本文件保持同步）。
