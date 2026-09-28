# M07-P01 — 审计与限流

## 目标

Gateway 记录关键审计事件（pair、task、workflow、review、revise）；对 pair/revise 等做基础限流。

## 范围

**允许：** `gateway/**`  

**禁止：** 改 App 产品范围  

## 上下文

- architecture 审计要求
- [gateway/docs/audit.md](../../../../gateway/docs/audit.md)

## 完成定义（DoD）

- [x] 审计日志可查询或落文件（格式文档化）
- [x] pair / revise 有速率限制或等价保护
- [x] 审计不含 API Key 明文

## 禁止事项

- 在日志中打印完整 Authorization 与 CURSOR_API_KEY
