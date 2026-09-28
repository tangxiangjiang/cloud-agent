# M05-P03 — Diff 采集与只读 API

## 目标

节点进入 awaiting_review 时，Slave 采集相对基线的 diff；Gateway 提供 `GET .../nodes/{id}/diff` 只读返回。

## 范围

**允许：** `slave/**`、`gateway/**`  

**禁止：** 任何写回源码的 HTTP API  

## 上下文

- [doc/flutter-app.md](../../../../flutter-app.md) 只读 diff
- [doc/api-outline.md](../../../../api-outline.md)

## 完成定义（DoD）

- [x] diff 含文件列表与 unified patch（或按文件 hunks）
- [x] GET diff 需鉴权；响应符合 contracts
- [x] 基线策略文档化（如节点开始时的 git stash/ref 或 task 开始 commit）
- [x] 无「apply patch」类接口

## 禁止事项

- App 提交编辑后的文件内容覆盖仓库
