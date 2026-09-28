# app — cloud-agent Flutter

手机端：配对登录、工作流触发与进度、只读 diff、修改意见、审核通过。  
不持有 `CURSOR_API_KEY`；不直连 Slave。设计见 [doc/flutter-app.md](../doc/flutter-app.md)。

## 要求

| 项 | 版本 |
|----|------|
| Flutter | **3.35+**（stable；本仓库用 Dart **3.9** / SDK constraint `^3.9.0`） |
| 平台 | Android / iOS（当前已生成） |

```bash
flutter --version   # 确认 stable
```

## 运行

1. 启动 Gateway（会打印 pair code）：

```bash
cd gateway && go run . -pair-code ABCD-EFGH
```

2. 运行 App：

```bash
cd app
flutter pub get
flutter run
```

配对页填写：

| 字段 | 示例 |
|------|------|
| Gateway base URL | Android 模拟器 `http://10.0.2.2:8080`；iOS 模拟器 / 桌面 `http://127.0.0.1:8080`；真机用电脑局域网 IP |
| Pair code | Gateway 启动日志中的码（如 `ABCD-EFGH`） |

成功后 Bearer token 写入 **`flutter_secure_storage`**（不会明文写进源码；日志仅打印脱敏尾缀）。未登录不能进入主页壳。

## 测试

```bash
cd app && flutter test
```

## 当前里程碑

| Phase | 状态 |
|-------|------|
| M06-P01 | 工程 + 配对登录 + AuthGate |
| **M06-P02** | 工作流列表 / 详情 / Start（当前） |
| M06-P03+ | WS 日志 / diff / revise·approve |

配对后主页进入 **Workflows**：列表展示 id 与状态；详情展示节点（`awaiting_review` 高亮）；可 **Start**（`POST /v1/workflows/{id}/start`）。状态约每数秒轮询；下拉刷新。App **不**编辑 DAG JSON。

## 安全注意

- 禁止在源码或完整日志中写入 Gateway token / Cursor API Key  
- Debug 构建允许 cleartext HTTP 以便连本机 Gateway；正式包应使用 HTTPS/WSS  
