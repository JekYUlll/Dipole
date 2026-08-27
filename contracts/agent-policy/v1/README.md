# Agent Policy Persistence v1

该契约固定 Agent Definition grant、AgentTask policy snapshot 与 Approval 的跨语言持久语义。Definition 的 permission/scope 内容按版本追加；Task 固定一个版本；Approval 固定 Task、capability、scope hash、arguments hash、nonce 和有效期。

Approval 恢复或消费时必须重新校验全部绑定字段。消费通过条件更新原子完成，已消费、撤销、过期或任一 hash 不匹配均拒绝，不能通过“先查询再执行”实现。

AgentTask UUID 使用 `task:` 加以下 UTF-8 canonical string 的 SHA-256 小写十六进制前 59 个字符，总长固定为 64。字段先 trim：

```text
dipole.agent.policy.persistence.v1\n<tenant_id>\n<agent_uuid>\n<trigger_type>\n<trigger_ref>
```

黄金向量 `dipole/UAI/message.direct.created/M100` 的结果为 `task:e47647aaf491da8a27072ed94d6b69b87a025a1e211000cbef6a9aeb458`。

`scope_sha256` 使用以下 UTF-8 canonical string 的 SHA-256 小写十六进制结果。字段先 trim，actions trim、去重并按字节升序排列：

```text
dipole.agent.scope.v1\n<resource_type>\n<resource_id>\n<action_1>\n<action_n>
```
