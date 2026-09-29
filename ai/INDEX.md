# AI Plan 索引

机器可读权威文件：[index.json](./index.json)  
启动时用计划 **id**，不必记 JSON 路径。

```powershell
python run.py plans
python run.py up --plan stub-two-node
python run.py up --agent --plan sample-dag
python run.py workflow --plan sample-dag
```

| id | 说明 | Bundle |
|----|------|--------|
| `stub-two-node` | 协议冒烟（默认 stub） | `ai/bundles/m05-p02-two-node.json` |
| `sample-dag` | cloud-agent 样例 phases | `examples/sample-dag.json` |

`plan-from-roadmap` 生成/更新 Bundle 后须同步改本索引（及 `index.json`）。
