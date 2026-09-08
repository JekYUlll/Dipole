# Dipole 面试技术参考

## 1. Go 特性清单

### `context.Context`

用于请求取消、超时、trace/request ID、服务关闭和 Agent trusted execution context。数据库、Kafka、gRPC 和后台 worker 都应沿调用链传递 context；长任务使用明确的 durable state，不能把 context 当持久化状态。

### goroutine、channel 与生命周期

Go 服务入口通过 `signal.NotifyContext` 建立根取消树，HTTP/gRPC、Kafka consumer 和 worker 在独立 goroutine 中运行，错误经 channel 或 errgroup 汇聚，关闭时先停止接入，再停止消费、排空有界队列并释放资源。

### interface 与依赖注入

Application 层依赖 `UserStore`、`MessageStore`、`AgentCapability`、RPC client 和 delivery sink 等接口；MySQL/sqlc、embedded implementation、gRPC adapter 和测试 fake 可以互换。接口的价值在于稳定领域语义与测试边界，避免 Handler 直接依赖数据库。

### 并发控制

`sync.Once` 用于一次性初始化；`sync.Mutex` 保护连接队列、去重和本地状态；`atomic` 保存轻量生命周期标志；`singleflight` 合并热点群相同 pull。并发代码必须配合 cancellation、容量、超时和测试，否则只使用 goroutine 会制造泄漏。

### 错误处理

使用 sentinel error、`fmt.Errorf("...: %w", err)`、gRPC status code 和固定 reason code 区分可重试、拒绝、冲突、不可用和终态。消费者只有在事实、投影/evidence 和 offset 顺序满足条件后才提交进度。

### `database/sql` 与 sqlc

SQL schema、query 和 migration 是可审查输入，sqlc 生成 `Querier`、参数类型和行类型，Repository 做领域映射，事务通过 `WithTx`/transaction store 组织。缓存只作为加速层，不能改变数据所有权。

## 2. Eino 实现卡片

| 面试问题 | 当前回答 | 证据 |
| --- | --- | --- |
| Eino 用在哪里 | Go embedded/legacy Agent baseline | `internal/services/agent/legacy/eino_agent.go` |
| Agent loop 怎么跑 | `adk.NewChatModelAgent` + `adk.NewRunner`，遍历 event iterator | `eino_agent.go` |
| 模型怎么抽象 | `BaseChatModel`，Provider factory 适配 OpenAI/Ollama | `model_factory.go` |
| Tool 怎么接入 | `einoTool.BaseTool`，Tool 内部从 trusted ExecutionContext 取身份 | `tools.go`、`execution_context.go` |
| 上下文怎么构造 | ContextBuilder 校验 owner/assistant 和 conversation scope 后组装 `schema.Message` | `context_builder.go` |
| 发送副作用怎么避免重复 | Tool 已发送消息则复用；普通文本使用稳定 command ID | `service.go`、`service_test.go` |
| 如何迁移 | `go-eino-baseline.json` 固化 outcome/trajectory/permission，TS Runtime 逐项对齐 | `contracts/agent-evals/v1/` |

Eino 负责 Agent 编排原语和模型/Tool 接口，业务事实、权限、审计和消息发送由 Dipole 代码负责。

## 3. 组件实现速查

| 组件 | 当前实现 | 常见追问 |
| --- | --- | --- |
| HTTP | Gin Handler、middleware、JWT | 为什么保持无状态？登出如何立即失效？ |
| WebSocket | Gateway connection/hub、心跳、有限队列 | 如何处理慢客户端和重连？ |
| gRPC | protobuf 生成 client/server、服务边界 adapter | 如何做兼容和 deadline？ |
| Kafka | kafka-go producer/consumer、topic、group、outbox relay | 重复消费如何处理？ |
| MySQL | `database/sql` + sqlc + versioned migration | 哪些表属于 Core/Message/Agent？ |
| Redis | presence、Pub/Sub、cache、rate limit、fence | Redis 故障时哪些路径 fail closed？ |
| MinIO | S3-compatible object storage、multipart lifecycle | 大文件如何分片、校验、清理？ |
| Cassandra | Timeline/archive candidate、backfill、shadow/read rollout | 为什么按时间线建模？ |
| Elasticsearch | async projection、alias、permission-aware search | 索引落后和恢复怎么办？ |
| Temporal | Agent Task/Activity durable foundation | 哪些代码可以放 Activity？ |
| MCP | strict schema、Tool invocation audit、egress policy | 如何防止外部 Tool 越权和泄密？ |
| OpenTelemetry | low-sensitivity model/tool/task spans | 如何关联一次 Agent Run？ |

## 4. “为什么这样设计”问答索引

### 为什么要 outbox？

消息事实落库和事件发布存在双写窗口。outbox 把待发布事件放进同一事务，relay 可以重试发布；消费端仍需幂等，outbox 不等于所有下游 exactly-once。

### 为什么用 `send_requested` 和 `created` 两个事件？

前者表达意图和持久化任务，后者表达已确认事实。后续投影只消费 `created`，避免把未落库请求误当消息事实。

### 为什么 Kafka 不能替代 Sync Store？

Kafka offset 是消费组进度，Sync Store 是用户/设备需要看到的业务状态。用户多天离线、换设备或重置游标时，必须按用户权限和 Timeline 查询持久数据。

### Cassandra、Elasticsearch、Redis 是否都是事实源？

当前按职责区分：消息/元数据事实由主存储负责；Cassandra 是顺序消息 Timeline 候选/接管存储，Elasticsearch 是搜索投影，Redis 是实时状态和缓存。任何接管都需要独立 owner、对账和回滚证据。

### 如何处理大文件？

文件正文进入对象存储，数据库保存 metadata、对象 key、大小和校验信息。Multipart Upload 需要初始化、分片上传、完成、abort、过期清理、重复 part 重试和对账；当前默认路径和切流状态以 A7 运行证据为准。

### 为什么前端也需要设计稿和测试？

IM 的同步状态、审批状态和权限边界需要稳定表达。Pencil `.pen` 是 canonical 设计基线，Vue 实现使用 design tokens，Playwright 维护 Chromium canonical screenshot，Firefox/WebKit 做功能和响应式验收。

## 5. 面试中应主动说明的限制

- Eino 仍是兼容/legacy baseline，TS Runtime 是目标执行面；不能把两者说成已经完全替换。
- Cassandra、Elasticsearch、C++ Delivery、外部 MCP 和部分 Temporal 组合按 shadow/candidate/default-off 口径描述。
- `chat.sent` 表示接入确认，不能直接等价为对方已收到或已读。
- Kafka、Redis 和 WebSocket 组合仍需要应用层幂等与恢复；不要承诺无条件的 exactly-once。
- 当前 benchmark 的用户数、消息量、机器规格、是否隔离拓扑和统计口径必须随报告一起说明。

## 6. 推荐准备顺序

1. 先掌握 IM 一次发送、落库、outbox、created、投递和同步恢复。
2. 再掌握 Go 并发、context、interface、sqlc、Kafka consumer 和 WebSocket 生命周期。
3. 然后讲热点群、Cassandra Timeline、搜索投影和故障回滚。
4. 最后讲 Eino baseline 到 TS Runtime 的迁移、Capability Policy、Temporal、MCP 和 Eval。
