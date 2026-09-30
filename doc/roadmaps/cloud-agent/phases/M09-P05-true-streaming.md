# M09-P05 — 对话真流式（token / text-delta）

## 目标

工程 Chat 的 assistant 回复在 App 上**边生成边显示**，而不是整段生成完才一次性出现。

## 范围

**允许：** `slave/src/agent/**`（`onDelta` / `text-delta` → `assistant.delta`）、必要单测；可选 App 更早订阅 WS / HTTP events 对齐 node logs  

**禁止：** 改审核闸门；Cloud Agent；把完整日志改成非 delta 协议

## 上下文

- [doc/project-chat.md](../../../project-chat.md)「流式」节  
- 现状：`agent.send` + `run.stream()` 映射整段 `assistant` 消息（粒度粗）；App 在 `sendMessage` 返回后才 `_attachStream`，晚订阅会 replay 成一坨  

## 完成定义（DoD）

- [ ] Local Agent：`send` 使用 SDK **`onDelta`（`text-delta`）**（或等价细粒度 API）向 Gateway 发高频 `assistant.delta`
- [ ] 仍保留 `run.stream()` 映射 tool/status/`done`（或文档说明等价路径）
- [ ] App：生成过程中气泡文本持续增长（手测或 widget/集成可观察多帧变化）
- [ ] 可选：发送后尽早订阅 WS，或先 HTTP `events?afterSeq=` 再 WS（对齐 node logs），减少晚订阅整包 replay
- [ ] 单测：delta 映射至少覆盖「多段 text-delta 拼成全文」
- [ ] 取消：cancel 后尽快停止再发新 delta

## 禁止事项

- 为「省事件」在 Slave 侧故意攒满整段再发一个 delta
- 破坏现有 `assistant.delta` payload 形状（`{ text }` 追加语义）导致旧 App 不可用
