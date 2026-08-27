# Agent Message Command API v1

该契约固定 Embedded Go/Eino 与未来 TypeScript Runtime 共用的消息写入边界。Runtime 只能提交命令种类、稳定 `command_id` 和正文；sender、target、权限与 correlation 均取自可信 `AgentInvocationV1`。

本地 adapter 在写入前执行 `AgentPolicyV1`，并将以下 UTF-8 canonical string 的 SHA-256 小写十六进制结果作为 Message `client_message_id`：

```text
dipole.agent.command.v1\n<kind>\n<command_id>
```

相同 Agent Command 重试会命中 Message Service 的 `(sender_uuid, client_message_id)` 幂等约束。`assistant_reply` 和 `system_message` 均经过 Message Service、Kafka send-requested 与 Outbox 链路，Agent Runtime 不持有消息数据库写权限。
