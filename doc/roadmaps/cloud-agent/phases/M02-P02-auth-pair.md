# M02-P02 — 配对鉴权与 middleware

## 目标

实现个人场景配对：`POST /v1/auth/pair` 换 Bearer token；受保护路由校验 Authorization。

## 范围

**允许：** `gateway/**`（auth、middleware、简易 token 存储）  

**禁止：** OAuth/SSO、手机端实现  

## 上下文

- [doc/api-outline.md](../../../../api-outline.md) 配对小节
- [doc/architecture.md](../../../../architecture.md) 安全约束

## 完成定义（DoD）

- [ ] 配置或启动时有 pairCode（开发期可打印到日志）
- [ ] 正确 pairCode 返回 token；错误返回 401
- [ ] 无 token 访问受保护路由 401；有效 token 通过
- [ ] 至少 1 个单元测试或集成测试覆盖鉴权

## 禁止事项

- 把 `CURSOR_API_KEY` 放进 token 或返回给客户端
