# M07 — 硬化与收尾

## 目标

补齐个人可用的部署与安全默认：审计日志、基础限流、样例 roadmap→DAG、Gateway/Slave 部署说明；收敛文档与实现偏差。

## 非目标

- 不做企业 SSO  
- 不做 Cloud Agent  
- 不做大规模多租户  

## 依赖

M06

## 验收

- 有一份「本机一天跑通」的部署文档  
- 样例 DAG 指向本 roadmaps 下 phase  
- 审计能查到 task/workflow/review 关键动作  
- README / api-outline 与实现无明显矛盾  

## Phases

| Phase | 文档 |
|-------|------|
| M07-P01 | [审计与限流](../phases/M07-P01-audit-ratelimit.md) |
| M07-P02 | [样例 DAG 与部署文档](../phases/M07-P02-sample-dag-deploy.md) |
| M07-P03 | [文档对齐与收尾检查清单](../phases/M07-P03-docs-align.md) |
