# M02-P03 — Task 存储与 CRUD/取消

## 目标

实现 Task 创建、详情、列表、取消；状态机符合契约；存储可用内存或 sqlite。

## 范围

**允许：** `gateway/**` Task 领域与 HTTP handlers  

**禁止：** 真实执行 Agent、WS 推送  

## 上下文

- contracts Task schema（M01-P02）
- [doc/api-outline.md](../../../../api-outline.md)

## 完成定义（DoD）

- [ ] `POST /v1/tasks` 支持 Idempotency-Key（重复提交不双建）
- [ ] `GET /v1/tasks`、`GET /v1/tasks/{id}`
- [ ] `POST /v1/tasks/{id}/cancel` → cancelling 或终态 cancelled（无执行器时可直接 cancelled）
- [ ] 状态字段与 contracts 一致
- [ ] 测试覆盖创建与取消主路径

## 禁止事项

- Cloud Agent 字段或 cloud runtime 开关
