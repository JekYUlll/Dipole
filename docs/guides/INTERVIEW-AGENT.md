# Dipole Agent 面试问答

## 30 秒介绍

Dipole Agent 是一个服务于 IM 场景的 TypeScript Durable Agent Runtime。用户私聊 AI、
在群里 `@AI` 或发起检索任务后，Runtime 创建稳定的 Temporal Workflow，基于可信
ExecutionContext 编译上下文，并通过受 Core 授权的 Capability 读取会话或执行操作。
写入动作必须经过人工审批，批准后由 Core 复核资源范围并以幂等 Message Command
写回 IM；Worker 重启后，Workflow 可从 Temporal 历史恢复。

## 核心流程

```text
IM event / user task
  -> Agent Runtime
  -> ExecutionContext + Context Compiler
  -> Temporal Workflow
  -> Model
  -> read Capability / approval wait / write Capability
  -> Core authorization
  -> idempotent Message Command
  -> IM message + Outbox + Kafka
```

## 高频问题

### 为什么要单独做 Agent Runtime？

IM 的消息、群成员和权限属于稳定的业务事实；Agent 的模型调用、工具循环、审批等待和
长任务恢复属于独立执行生命周期。Runtime 独立后可以管理任务执行与模型适配，同时让
Go Core 保持业务事实和最终副作用的权威性。

### ExecutionContext 解决什么问题？

ExecutionContext 保存 principal、Task、Run、会话范围和 trace 等可信信息。它由
Gateway/Core 的认证结果派生，模型和浏览器只能传递任务意图与 Tool 参数，无法构造
另一个用户、租户或更大的资源范围。

### Capability 与普通 Function Tool 有什么区别？

Capability 除了输入 schema 和执行函数，还描述风险等级、所需权限和资源解析方式。
Runtime 先根据 Tool 输入计算目标资源，再把可信主体和资源范围提交给 Core 复核。
这种模型让本地工具、MCP Tool 与 IM 命令共享同一套授权与审计边界。

### Agent 如何检索历史会话？

模型可以选择 `conversation.list`、`conversation.read` 和 `conversation.search`。这些都是
普通的只读 Capability，运行在原 Task 内：Runtime 请求 Core 做权限校验，Search Service
返回有界结果，Context Compiler 将结果作为不可信 evidence 注入下一轮模型调用。检索
不拥有独立身份、独立任务队列或独立产品入口。

### 为什么写操作必须等待 Approval？

读会话和发送消息的风险不同。写 Capability 先生成精确的 Task、Run、Capability、资源
范围和参数摘要绑定，Workflow 进入 `WAITING_APPROVAL`。用户拒绝时任务取消且没有
副作用；用户批准后，Core 再次检查权限、范围和批准绑定，随后执行一次 Message Command。

### Temporal 在这里的价值是什么？

Temporal 持久化 Workflow 历史、Activity 重试与 Signal。任务等待审批或 Worker 短暂
故障时，新的 Worker 能继续同一 Workflow ID。每个写入调用使用稳定 invocation ID，
即使命令已经提交但响应丢失，重试也会复用同一个业务副作用。

### 如何避免 Agent 自己触发自己？

Agent 写入的消息带有来源与因果信息。事件入口会识别 Agent 产出的消息并跳过同源触发；
Event Ledger 同时用事件键保护 Kafka 重放，确保一条用户消息不会创建多个同类 Task。

### Context Compiler 做了什么？

它按预算组合当前请求、当前会话窗口、可用 Capability 和检索证据，并保留每段内容的
来源与信任级别。搜索结果和 Memory 都按不可信上下文处理，模型必须通过 Capability
才能读取更多数据或执行操作。

### MCP 在项目中如何使用？

MCP 提供统一 Tool 协议。Dipole Runtime 将 MCP Tool 投影为 Capability，并保留 allowlist、
schema 校验、超时、有界输入输出、来源记录和写操作审批。核心演示使用 Dipole 自身的
会话与消息能力，外部系统凭据不进入普通任务日志。

## 可演示的五个场景

| 场景 | 证明的能力 |
| --- | --- |
| 私聊 AI | IM 事件进入 TypeScript Runtime，再由 Temporal 生成回复 |
| 群聊 `@AI` | Mention 过滤、群范围上下文与同群回复 |
| 历史讨论总结 | `conversation.search`、权限校验、有界 evidence 与二次推理 |
| 拒绝/批准写操作 | HITL、Core 二次授权、零或一次业务副作用 |
| Worker 重启后批准 | Durable Workflow、稳定 Task ID 与幂等 Command |

## 讲解重点

用“可信上下文、能力授权、持久执行、幂等副作用”四个词组织回答。模型负责选择工具和
生成文本；权限、范围、审批和消息事实由服务端领域代码决定。这样可以清楚区分 LLM
推理与系统权威状态，也能解释 Agent 在故障、重试和多用户场景下如何保持可控。
