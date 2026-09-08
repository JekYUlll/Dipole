# Dipole Agent 面试分册

## 项目定位

Dipole Agent 是一个 IM-native Agent 平台，重点展示模型调用之外的工程能力：可信身份、能力授权、上下文编译、长任务恢复、人工审批、Memory、MCP、审计和评测。

## 语言边界

```text
Go / Eino baseline
  -> 当前兼容 Agent、IM 事件触发、旧 Tool loop 和迁移评测基线

TypeScript / Node.js Agent Runtime
  -> ExecutionContext、Capability、Policy、Model Router、MCP、Memory、Temporal、OTel、Eval

Go Core + gRPC
  -> owner、权限、Task/Run/Invocation 持久状态和最终业务副作用
```

Go/Eino 和 TS Runtime 不能混成一个“模型服务”来讲。前者是兼容基线与迁移参照，后者是目标执行面；每个切换都由契约、shadow 和回滚门禁约束。

## Eino 在项目里的使用方式

Eino 是 Go 侧 Agent 编排和模型抽象库，当前代码使用：

- `adk.ChatModelAgentConfig` 描述 Agent 名称、说明、system instruction 和模型。
- `adk.NewChatModelAgent` 创建 ChatModel Agent。
- `adk.NewRunner` 执行消息和 Tool loop。
- `schema.Message` 作为上下文、用户消息和助手回复的数据结构。
- `eino/components/model.BaseChatModel` 抽象模型调用。
- `eino/components/tool.BaseTool` 接入用户资料、会话查询、历史检索和系统消息能力。
- `eino-ext/components/model/openai` 与 `ollama` 提供模型适配。
- `adk.GetMessage` 从 Runner event 提取最终助手消息。

`internal/services/agent/legacy/eino_agent.go` 只把 Eino Runner 封装成 `Agent.Reply`；业务服务负责事件过滤、ExecutionContext、Policy、日志、消息命令和失败状态。这样模型输出不会直接获得数据库或身份权限。

## 旧 Eino 链路

```text
message.direct.created
  -> Go Agent Service 过滤目标 Assistant
  -> policy Start / 幂等 call log
  -> trusted ExecutionContext
  -> ContextBuilder 读取有权限的会话上下文
  -> Eino Runner / Tool calling
  -> Tool 已发送消息则复用
  -> 否则用稳定 command ID 发送助手消息
  -> policy Complete + call log
```

旧链路的 baseline 见 [go-eino-baseline.json](../../contracts/agent-evals/v1/go-eino-baseline.json)，它固定 event、reply、trajectory 和 permission 结果，迁移 Runtime 时作为回归参照。

## TypeScript Runtime 组件

| 组件 | 实现方式 | 解决的问题 |
| --- | --- | --- |
| ExecutionContext | Zod strict schema + trusted Core/RPC input | 模型参数不能伪造 principal、tenant、Task 或 scope |
| CapabilityRegistry | capability descriptor、input schema、resource resolver | 统一 Tool、MCP 和内部能力的发现与执行 |
| PolicyEngine | permission、risk、approval、resource scope 检查 | 读写和破坏性操作分级控制 |
| ModelRouter | route 列表、预算、结构化输出、审计 reservation/recovery | provider fallback、重试和模型成本边界 |
| Context Compiler | 有界窗口、任务、Memory、检索和工具上下文 | 防止无界 prompt 和 context rot |
| Event Ledger | 事件去重、claim、消费状态 | Kafka 重放不重复创建 Task |
| Temporal Workflow | Task 生命周期和 Activity 边界 | 进程重启后继续长任务 |
| Memory | working、episodic、semantic、procedural、observational 策略 | 短期上下文和长期知识分层 |
| MCP | Host/Server、严格 schema、来源披露、egress guard | 标准化外部工具连接并限制敏感字段 |
| OTel | 低敏 span、model/tool/task 属性、默认关闭 exporter | 定位延迟、失败和成本 |
| Eval | outcome、trajectory、permission、retrieval、cost | 评估 Agent 是否完成任务以及如何完成 |

## Human-in-the-loop

写入或高风险 Capability 必须产生 approval request。用户批准后，Runtime 通过稳定的 Task/Run/Invocation 绑定恢复；拒绝、过期、请求漂移和权限变化均 fail closed。审批记录表达授权事实，业务执行仍由 Core 的权威服务完成。

## Memory 口径

- `working` 只绑定当前 Task/Run，不自动升级为长期记忆。
- `episodic` 保存带证据的事件摘要。
- `semantic` 保存经过审查的稳定事实。
- `procedural` 保存可复用的工作方式或 Skill。
- `observational` 由 Observation/Reflection worker 产生候选，经过 owner review、哈希和 promotion receipt 后才允许持久化。

面试时要说明 Memory 的难点在 provenance、scope、版本、纠正、撤销和 promotion 审计，向量检索只是其中一个检索实现。

## MCP 口径

Dipole Agent 同时可以做 MCP Host 和 MCP Server。当前内置 Server 的真实入口、Tool 数量、外部连接状态和生产写入开关必须以 `services/agent-runtime/README.md` 与配置为准。外部 MCP 的 egress policy 会限制 Tool、顶层参数、载荷大小、嵌套深度和常见凭据字段；它不能替代内容级 DLP、Secret 管理和第三方服务授权。

## 高频问题

### Eino 和 LangChain/自研 loop 的关系是什么？

Eino 在 Go baseline 中承担模型、消息、Tool 和 Runner 抽象；业务身份、权限、审计和副作用仍由 Dipole 自己控制。迁移到 TypeScript 后，Runtime 继续保留这些领域边界，模型 SDK 只作为底层 primitive，不把框架内部状态当作业务事实。

### 为什么 Agent 要单独做 Runtime？

Agent 的长任务、模型重试、Tool 审批、Memory、MCP 和成本控制有独立生命周期。拆出 Runtime 后，可以独立扩缩容、升级模型适配和执行策略，同时让 Go Core 保持 owner、权限和业务事实的权威性。

### 如何防止模型越权？

principal 从认证/Run Context 注入，Tool 参数只表达资源意图；CapabilityRegistry 解析输入并计算资源，PolicyEngine 检查 permission、risk、approval 和 resource scope，最终 Core 再做服务端校验。模型传入的 user ID、tenant ID 或 Agent ID 不参与建立可信身份。

### Temporal 解决什么问题？

Temporal 负责 durable execution：Task 状态、Activity 重试、等待审批/输入、进程重启恢复和确定性 Workflow ID。它不承担权限事实和消息事实，业务状态仍由 Core/SQL 持久化。

### 为什么需要 Agent Eval？

只看最终文本会漏掉越权读取、无意义 Tool 调用、重复副作用和成本失控。评测同时检查 outcome、trajectory、permission、retrieval evidence 和 cost，并保留 adversarial case 作为回归。

## 当前状态边界

可以直接讲的能力包括 Eino baseline、TS Runtime 核心模块、Capability/Policy 测试、模型路由、Memory 评测、MCP 协议层、Temporal foundation、OTel 和安全门禁。External MCP 写入、生产模型切换、公开 Agent authority 和部分 Temporal/Remote 组合仍需按 `default-off`、`shadow` 或 `candidate` 口径描述。
