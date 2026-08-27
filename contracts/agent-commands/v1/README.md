# Agent Message Command API v1

该契约固定 Embedded Go/Eino 与 TypeScript Runtime 共用的消息写入边界。Embedded 调用提交命令种类、稳定 `command_id` 和正文；MCP Runtime 引用已审批 Tool Invocation，由 Core 根据 Invocation 和 kind 派生 Command ID。sender、target、权限与 correlation 均取自可信 `AgentInvocationV1`。

本地 adapter 在写入前执行 `AgentPolicyV1`，并将以下 UTF-8 canonical string 的 SHA-256 小写十六进制结果作为 Message `client_message_id`：

```text
dipole.agent.command.v1\n<kind>\n<command_id>
```

相同 Agent Command 重试会命中 Message Service 的 `(sender_uuid, client_message_id)` 幂等约束。`assistant_reply` 和 `system_message` 均经过 Message Service、Kafka send-requested 与 Outbox 链路，Agent Runtime 不持有消息数据库写权限。

MCP 消息 Tool 的审批参数使用递归排序键的 canonical JSON。当前 direct message 形态固定为：

```json
{"content":"notice","conversationId":"direct:U100:UAI"}
```

该字节串的 SHA-256 为 `5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9`。Go 与 TypeScript 测试共同固定此向量，避免对象键顺序造成 Approval、Tool audit 与 Command RPC 摘要漂移。
