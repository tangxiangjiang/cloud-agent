# slave

Node.js / TypeScript Local Slave：出站连接 Gateway，调用官方 `@cursor/sdk`（仅 Local runtime）。

真实 SDK 执行从 roadmap **M04** 开始。

M03 联调可用 Gateway 侧 mock（无 SDK）：

```bash
cd ../gateway
go run ./cmd/mockslave -token <bearer> -id slave_devpc
```

协议：[../gateway/docs/slave-ws.md](../gateway/docs/slave-ws.md)  
参见：[doc/local-slave.md](../doc/local-slave.md)
