# 契约目录

`contracts/` 保存跨服务、跨语言和证据工具共享的版本化契约。每个目录内的 `README.md` 解释语义、兼容范围和晋级边界；JSON Schema、示例和测试共同构成可执行接口。

## Agent 契约

| 领域 | 契约 |
| --- | --- |
| Capability 与权限 | [Capabilities](agent-capabilities/v1/README.md)、[Policy](agent-policy/v1/README.md) |
| Task、Artifact 与输入 | [Artifact](agent-artifact/v1/README.md)、[Elicitation](agent-elicitation/v1/README.md)、[Task Timeline](agent-task-timeline/v1/README.md) |
| Memory | [Memory Promotion v1](agent-memory-promotion/v1/README.md)、[Memory Promotion v2](agent-memory-promotion/v2/README.md)、[Memory Prefilter](agent-memory-prefilter/v1/README.md)、[Reviewed Corpus](agent-memory-reviewed-corpus/v1/README.md)、[Retention](agent-memory-retention/v1/README.md) |
| 事件与消息动作 | [Commands](agent-commands/v1/README.md) |
| MCP | 运行时边界见 [Agent MCP 文档](../docs/agent/README.md)，协议输入由服务内 MCP adapter 校验 |
| 发布与修复 | [Release](agent-release/v1/README.md)、[Promotion v1](agent-promotion/v1/README.md)、[Promotion v2](agent-promotion/v2/README.md)、[Workflow Repair](agent-workflow-repair/v1/README.md)、[Repair Rollout](agent-timeline-repair-rollout/v1/README.md) |
| 评估 | [Agent Evals](agent-evals/v1/README.md) |

## 其他平台契约

- [领域事件](events/domain/v1/)
- [Sync Cassandra Hydration](sync-cassandra-hydration/v1/)
- [Web Sync Observation](web-sync-observation/v1/)
- [Cassandra Read Rollout](cassandra-read-rollout/v1/)
- [Realtime Delivery](realtime-delivery-fence/v1/)

## 维护规则

1. 版本化目录只做向后兼容的追加修改；破坏性语义发布新版本目录。
2. 修改 Schema 时同步更新该目录的 README、示例和对应测试。
3. `eligible`、`approved` 或 `recorded` 只表示契约门禁通过，不能单独授予生产 authority。
4. 真实环境证据必须绑定 revision、配置/契约哈希和有效窗口；synthetic fixture 不得代替共享环境验收。
5. 新增跨服务契约时，同时更新 [文档目录](../docs/README.md) 和必要的架构债务条目。
