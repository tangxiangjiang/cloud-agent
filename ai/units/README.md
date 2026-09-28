# ai/units

存放 **任务单元（TaskUnit）**：Slave 结构化生成后，经 Gateway 推到手机。

建议布局：

```
ai/units/<workflowId>/<nodeId>.json
```

- 运行中由 Slave 写入/更新，一般 **不要手改**  
- 可加入 `.gitignore` 忽略运行产物，或只提交样例  
- 字段见 [../SCHEMA.md](../SCHEMA.md)

手机侧消费的是 Gateway API/WS 推送的同等结构；磁盘文件便于本机调试与审计。
