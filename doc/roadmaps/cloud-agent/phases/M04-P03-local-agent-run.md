# M04-P03 — Local Agent create/send/stream/wait

## 目标

对领取的 task：校验 repo 白名单 → `Agent.create({ local: { cwd }, model, apiKey })` → `send` → stream 映射为契约事件 → **必须** `wait` → dispose。

## 范围

**允许：** `slave/**` Agent 执行器  

**禁止：** `cloud` runtime；approve/progress 逻辑  

## 上下文

- Cursor SDK TypeScript Local runtime
- [doc/local-slave.md](../../../../local-slave.md)

## 完成定义（DoD）

- [x] 显式只使用 local；代码审查无 cloud 默认分支误用
- [x] stream 事件映射 assistant.delta / status / done（工具事件尽力）
- [x] 每 run 调用 wait；失败区分启动失败 vs run error（若可映射）
- [x] 用真实或最小本地目录跑通一次（文档记录所需 env）

## 禁止事项

- 将整仓上传/clone 到 Cursor 云端 VM 的任何实现
- `settingSources: "all"` 作为默认
