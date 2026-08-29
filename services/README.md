# 多语言服务

`services/` 收纳需要独立语言工具链和独立构建上下文的长期运行服务。Go 服务入口统一位于 [`cmd/services/`](../cmd/services/README.md)，跨服务 RPC 和事件契约统一位于 [`api/proto/`](../api/proto/) 与 [`contracts/`](../contracts/)。

## 服务

| 目录 | 语言 | 职责 | 默认状态 |
| --- | --- | --- | --- |
| `agent-runtime/` | TypeScript / Node.js | Agent Task、Memory、Capability、MCP 和审计运行时 | 集成 Compose 默认启动并消费 Kafka Shadow 流；`active` authority、模型调用和写能力默认关闭 |
| `realtime-delivery/` | C++20 | Kafka 消息事件的实时投递候选数据面 | `realtime-cpp` profile 默认关闭 |

每个服务维护自己的依赖和测试入口。服务之间不得通过相对路径引用对方源码，协议变更应先更新 `api/proto/` 或 `contracts/`。

## Agent Runtime 默认语义

集成 Compose 中的 `agent` 容器默认启动，用于验证事件接收、幂等账本和 Shadow 轨迹。未设置 `DIPOLE_AGENT_RUNTIME_MODE=remote` 时，Runtime 解析为 `shadow`；这不会授予 Agent authoritative Task、模型调用或消息写入权限。

启用 `active` 前必须同时提供候选版本、release manifest、Temporal `read_active` 配置和 promotion binding，并完成架构债务台账中列出的共享环境观测与回滚门禁。验证默认配置可使用：

```bash
docker compose -f deploy/compose/docker-compose.microservices.yml config
npm --prefix services/agent-runtime test
```
