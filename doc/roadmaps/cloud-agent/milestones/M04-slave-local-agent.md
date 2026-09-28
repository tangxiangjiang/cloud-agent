# M04 — Node Slave + Local Agent

## 目标

Node（TypeScript）Slave 使用官方 `@cursor/sdk` **Local runtime** 执行单任务，流式事件经 Gateway 到达订阅端。禁止 Cloud Agent 代码路径。

## 非目标

- 不做 DAG / 审核通过写进度  
- 不做 Flutter  
- 不暴露 Slave 公网端口  

## 依赖

M03

## 验收

- 配置白名单 `cwd` + `CURSOR_API_KEY` 后，创建 task 能真正改本地测试仓库或 dry-run 仓库  
- 事件含 status / assistant.delta / done（工具事件有则更好）  
- 取消能传到 `run.cancel()`（若 supports）  
- 文档说明本机启动方式  

## Phases

| Phase | 文档 |
|-------|------|
| M04-P01 | [Slave TS 工程与配置加载](../phases/M04-P01-slave-project.md) |
| M04-P02 | [接入 Gateway 出站客户端](../phases/M04-P02-slave-gateway-client.md) |
| M04-P03 | [Local Agent create/send/stream/wait](../phases/M04-P03-local-agent-run.md) |
| M04-P04 | [取消、错误分类与安全默认](../phases/M04-P04-cancel-safety.md) |
