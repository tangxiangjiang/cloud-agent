# 通信选型：HTTP vs HTTP + WebSocket

## 结论（冻结）

采用 **HTTP + WebSocket 混合**。

| 通道 | 用途 |
|------|------|
| **HTTP** | 鉴权、配对、列出 slave/仓库、创建任务、跟进任务、取消、拉取历史与终态 |
| **WebSocket** | 单任务/会话的实时事件：状态变更、assistant 增量文本、工具调用起止、错误、结束 |

纯 HTTP（轮询）仅可作为**离线兜底**（WS 断线时用 `GET /tasks/{id}` 对齐状态），不作为主路径。

## 为什么不只用 HTTP

Agent 是长时、多事件过程：

- 一次 run 可持续数分钟，事件频率从「几秒一条状态」到「流式 token」
- 纯轮询：要么延迟大，要么请求极密 → 费电、费流量、手机后台易被杀
- 取消、进度条、日志流都依赖低延迟推送

HTTP 很适合「发令 + 查档」；不适合「旁听 Agent 干活」。

## 为什么不做成纯 WebSocket

全部塞进一条 WS 会带来：

- 鉴权、重放、幂等、缓存、CDN/代理兼容都变麻烦  
- Flutter 侧重连、心跳、半包协议，调试成本高于 REST  
- 任务创建更适合有明确 request/response 与 HTTP 状态码  

混合职责清晰，实现和维护都更简单。

## 与 SSE 的对比

| 方案 | 优点 | 缺点 | 本项目 |
|------|------|------|--------|
| HTTP 轮询 | 实现最简单 | 延迟/耗电差 | 仅兜底 |
| HTTP + SSE | 单向推送够用、实现比 WS 轻 | 双向弱；部分代理对长连接不友好；Flutter 生态略别扭 | 备选 |
| **HTTP + WebSocket** | 双向、推送自然、取消/订阅模型清晰 | 需心跳与重连 | **首选** |

若第一期只做「看日志、不能从推送通道回传」，SSE 可替代 WS；一旦要「订阅多任务 / 轻量信令」，WS 更省事。故文档直接定混合 WS。

## 协议约定（概要）

### HTTP

- Base：`https://{gateway}/v1`
- 鉴权：`Authorization: Bearer <device_or_user_token>`
- 幂等：创建任务可带 `Idempotency-Key`
- 详见 [api-outline.md](./api-outline.md)

### WebSocket

- URL：`wss://{gateway}/v1/ws?token=...`（或连接后首帧 auth）
- 连接后客户端发送 `subscribe`：`{ "type": "subscribe", "taskId": "..." }`
- 服务端推送统一信封：

```json
{
  "type": "task.event",
  "taskId": "tsk_xxx",
  "seq": 42,
  "at": "2026-09-28T03:00:00.000Z",
  "event": {
    "kind": "assistant.delta | tool.started | tool.finished | status | error | done",
    "payload": {}
  }
}
```

- `seq` 单调递增；断线重连后 App 带 `lastSeq`，Gateway 补发或提示用 HTTP 拉快照
- 心跳：客户端 `ping` / 服务端 `pong`，建议 20–30s

### 职责划分（避免双写混乱）

| 动作 | 走哪 |
|------|------|
| 创建 / 跟进 / 取消任务 | HTTP |
| 列表、详情、历史消息 | HTTP |
| 直播进度与流式文本 | WebSocket |
| WS 断线期间状态 | HTTP `GET /tasks/{id}` 对齐，连上后再 subscribe |

## Flutter 侧注意点

- HTTP：`dio` 或官方 `http`
- WS：`web_socket_channel`；前台保活策略按 iOS/Android 分别处理
- 状态机：`connecting → subscribed → streaming → terminal`；terminal 后仍以 HTTP 详情为准做最终展示
- 不要在 WS 里传 `CURSOR_API_KEY`

## 演进

1. **MVP**：HTTP 全量 + WS 只推 `status` / `assistant.delta` / `done`  
2. **下一期**：工具事件、多任务订阅、`lastSeq` 补发  
3. **可选**：SSE 兼容端点给 Web 管理页
