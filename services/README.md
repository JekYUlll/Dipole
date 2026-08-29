# 多语言服务

`services/` 收纳需要独立语言工具链和独立构建上下文的长期运行服务。Go 服务入口统一位于 [`cmd/services/`](../cmd/services/README.md)，跨服务 RPC 和事件契约统一位于 [`api/proto/`](../api/proto/) 与 [`contracts/`](../contracts/)。

## 服务

| 目录 | 语言 | 职责 | 默认状态 |
| --- | --- | --- | --- |
| `agent-runtime/` | TypeScript / Node.js | Agent Task、Memory、Capability、MCP 和审计运行时 | 集成 Compose 默认启用，模型和写能力按门禁控制 |
| `realtime-delivery/` | C++20 | Kafka 消息事件的实时投递候选数据面 | `realtime-cpp` profile 默认关闭 |

每个服务维护自己的依赖和测试入口。服务之间不得通过相对路径引用对方源码，协议变更应先更新 `api/proto/` 或 `contracts/`。
