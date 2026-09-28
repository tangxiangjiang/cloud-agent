# M02-P01 — Go module 与 HTTP 服务器骨架

## 目标

在 `gateway/` 初始化 Go module，拉起可配置端口的 HTTP 服务器，实现 `GET /v1/health`。

## 范围

**允许：** `gateway/**`  

**禁止：** 鉴权、Task、WS、连 Slave  

## 上下文

- [doc/api-outline.md](../../../../api-outline.md)
- M01 目录约定

## 完成定义（DoD）

- [x] `go.mod` 存在，主包可 `go run` / `go build`
- [x] `GET /v1/health` 返回 JSON `{"ok":true}`
- [x] 端口可配置（env 或 flag）
- [x] 简短 `gateway/README.md` 启动说明

## 禁止事项

- 引入与本里程碑无关的大型框架堆砌（保持简单：标准库或轻量路由即可）
