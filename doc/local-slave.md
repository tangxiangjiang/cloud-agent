# Local Slave（Node）

## 定位

跑在开发者本机的常驻 **Node** 进程（文档里也叫 Local Worker）：

- 出站连接 Go Gateway，领取任务 / 接收取消 / **接收审核结果**  
- **加载** `ai/bundles` 计划；按节点执行后 **写出** `ai/units/**` 任务单元经 Gateway 推手机  
- 用官方 `@cursor/sdk` **Local runtime** 执行每个节点  
- 跑完进入 `awaiting_review`，上报只读 diff；App revise / approve  
- **仅 approve 后**更新 `ai/progress.md`（或 Bundle 指定的 progressDoc）

不对公网暴露；不嵌入 Flutter。Gateway 仍是 Go。  
工作流与 DAG 语义见 [workflow.md](./workflow.md)。

## 职责边界

| 组件 | 语言 | 做什么 |
|------|------|--------|
| Gateway | Go | 鉴权、任务队列、对 App 的 HTTP/WS、审计 |
| **Local Slave** | **Node (TypeScript)** | 唯一直接调用 `@cursor/sdk` 的进程 |

```
Flutter ──HTTPS/WSS──► Go Gateway ◄──出站 WS/HTTP── Node Slave
                                              │
                                              ▼
                                     @cursor/sdk local.cwd
```

## 与 SDK 的对应关系

| 场景 | SDK |
|------|-----|
| 新任务 | `Agent.create({ apiKey, model, local: { cwd } })` → `send(prompt)` |
| 跟进 | `Agent.resume(agentId)` 或复用未 dispose 的 agent → `send` |
| 观察 | `for await (const e of run.stream())` → 映射为 Gateway 事件 |
| 结束 | **必须** `await run.wait()` |
| 取消 | `run.supports("cancel")` 时 `run.cancel()` |
| 清理 | `await using` / `dispose`，避免泄漏 |

注意：

- 显式传 `local: { cwd }`，**不要**传 `cloud`  
- Model 对 local **必填**；可用 `composer-2.5` 或网关下发的 model id  
- `CURSOR_API_KEY` 仅本机环境变量 / 本地密钥文件  
- 默认不要加载 `settingSources: "all"`  

## 配置示例（概念）

```yaml
gatewayUrl: wss://gateway.example.com/v1/slave
slaveId: slave_devpc
repos:
  - id: r_cloud_agent
    name: cloud-agent
    cwd: E:/workspace/cloud-agent
apiKeyEnv: CURSOR_API_KEY
```

## 安全要点

- `cwd` 白名单：拒绝任意路径任务  
- 日志脱敏：避免打印完整 API Key、`.env` 内容  
- 后续：hooks 限制 `beforeShellExecution`  

## 进程模型（建议）

- 单 Slave 内任务串行（个人机器稳妥）或有限并发  
- 用 `pm2` / Windows 服务 / systemd 保活  
- 与 Gateway 断线：任务保持 `queued`，重连后领取  

## 运行依赖（本机）

- Node.js >= 18（以官方 SDK 要求为准）  
- `@cursor/sdk`  
- `CURSOR_API_KEY`  
- 已配置白名单仓库的本地路径  

不需要在本机再跑一层 Go Worker。
