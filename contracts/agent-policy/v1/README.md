# Agent Policy Persistence v1

该契约固定 Agent Definition grant、AgentTask policy snapshot 与 Approval 的跨语言持久语义。Definition 的 permission/scope 内容按版本追加；Task 固定一个版本；Approval 固定 Task、capability、scope hash、arguments hash、nonce 和有效期。

Approval 恢复或消费时必须重新校验全部绑定字段。消费通过条件更新原子完成，已消费、撤销、过期或任一 hash 不匹配均拒绝，不能通过“先查询再执行”实现。

`scope_sha256` 使用以下 UTF-8 canonical string 的 SHA-256 小写十六进制结果。字段先 trim，actions trim、去重并按字节升序排列：

```text
dipole.agent.scope.v1\n<resource_type>\n<resource_id>\n<action_1>\n<action_n>
```
