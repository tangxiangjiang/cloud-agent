# M02 — Gateway HTTP 基础

## 目标

用 Go 实现 Gateway 的 HTTP 面：健康检查、配对换 token、Slave/仓库列表占位、Task 的创建/查询/取消（内存或 sqlite 即可）。

## 非目标

- 不做 WebSocket  
- 不连真实 Slave / SDK  
- 不做 Flutter  

## 依赖

M01

## 验收

- `GET /v1/health` 可用  
- 配对后 Bearer token 可访问受保护接口  
- `POST/GET /v1/tasks`、`POST .../cancel` 行为符合 [api-outline](../../../api-outline.md) 草案子集  
- 有基础测试或可手跑的 curl 脚本  

## Phases

| Phase | 文档 |
|-------|------|
| M02-P01 | [Go module 与 HTTP 服务器骨架](../phases/M02-P01-gateway-skeleton.md) |
| M02-P02 | [配对鉴权与 middleware](../phases/M02-P02-auth-pair.md) |
| M02-P03 | [Task 存储与 CRUD/取消](../phases/M02-P03-task-crud.md) |
| M02-P04 | [Slaves 列表占位 API](../phases/M02-P04-slaves-placeholder.md) |
