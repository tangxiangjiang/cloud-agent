# M11-P01 — 设计与配置契约

## 目标

把 slave-master 架构写进权威文档与可校验 schema：Master 配置、子 Slave 单工程形态、控制面消息形状（可先 JSON Schema 草案）。

## 范围

**允许：** `doc/slave-master.md`、`doc/architecture.md`（角色图补丁）、`doc/api-outline.md`（草案表）、`contracts/**`（可选 schema）、roadmap 交叉链接  

**禁止：** 实现 Master 进程 / App UI / 改 Gateway 行为（除文档）

## 上下文

- [doc/slave-master.md](../../../slave-master.md)
- 现网 [slave/config.example.yaml](../../../../slave/config.example.yaml)

## 完成定义（DoD）

- [x] `doc/slave-master.md` 含角色图、配置示例、控制面/HTTP 草案、迁移与非目标
- [x] **决议**节：共享 token、权威源、三态、串行幂等、配额、stop、收养、slaveId 迁移
- [x] Master `slaves[]` 与「一 Slave 一 project」约束成文
- [x] `api-outline` 增加 `/v1/masters*` 与 Master WS 条目（标注 M11）
- [x] （可选）`contracts/schemas` 增加 master-config / master WS 消息草案
- [x] architecture 角色表增加 slave-master / 指向本文

## 禁止事项

- 在配置示例中写入真实 API Key / token
- 要求 App 传入任意未校验 cwd 绕过 Master
