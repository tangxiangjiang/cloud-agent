# M04-P01 — Slave TS 工程与配置加载

## 目标

初始化 `slave/` TypeScript 工程：配置加载（gatewayUrl、slaveId、repos 白名单、apiKeyEnv）、日志基础。

## 范围

**允许：** `slave/**`（工程配置、config 类型、README）  

**禁止：** 尚不调用 SDK 跑任务（可留空接口）  

## 上下文

- [doc/local-slave.md](../../../../local-slave.md)

## 完成定义（DoD）

- [ ] `package.json` + TS 构建/运行脚本
- [ ] 配置文件或 env 示例（**不含真实 API Key**）
- [ ] 启动时校验：repos cwd 为绝对路径白名单列表
- [ ] `slave/README.md` 说明依赖 Node 版本

## 禁止事项

- `cloud: {}` 或 Cloud Agent 配置项作为默认
