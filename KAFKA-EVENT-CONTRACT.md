# Kafka 事件契约

本文档定义 Dipole 内部领域事件的版本、兼容、弃用、重试与死信规则。Kafka 用于服务间事实传播，用户设备同步继续使用持久化 Sync Timeline。

## Envelope

所有领域事件使用统一 JSON Envelope：

```json
{
  "event_id": "E...",
  "event_type": "message.direct.created",
  "version": "v1",
  "source": "dipole",
  "occurred_at": "2026-08-26T00:00:00Z",
  "payload": {}
}
```

Publisher 同时写入 `event_type`、`version`、`schema_version`、`source` 和 `event_id` headers。`version` 保留滚动兼容，`schema_version` 是后续跨语言消费者的明确入口；Envelope 是冲突时的数据事实源。

## 兼容规则

| 输入版本 | 行为 |
| --- | --- |
| 空版本 | 按 legacy v1 读取，并在内存中归一化为 `v1` |
| `v1`、`v1.x` | 正常消费；新增字段必须可选，消费者忽略未知字段 |
| `v2` 及其他主版本 | 视为永久 schema 错误，不调用业务 Handler，直接发布到 `.dead` |
| 非法版本文本 | 按永久 schema 错误处理 |

同一主版本允许追加可选字段、增加枚举值的 unknown fallback 和放宽读取规则。字段删除、类型变化、语义复用、幂等键变化或路由键变化必须提升主版本。

## Breaking Migration

主版本升级按以下顺序执行：

1. 新消费者先支持 v1 与 v2，并部署到全部消费组。
2. Producer 开启 v2 影子事件或受控双发，使用独立 event ID，影子消费者禁止业务写入。
3. 对比 payload、路由键、消费结果、lag 和 DLQ，完成重放验证。
4. Producer 切换 v2，旧消费者继续保留至少一个完整滚动部署窗口。
5. 确认无 v1 lag、无活跃旧实例且回滚版本支持当前数据后，停止 v1。
6. 在更新日志记录弃用时间、最后兼容版本、重放 checkpoint 和恢复步骤。

禁止在同一发布中同时修改 Topic、partition key 和 schema 主版本。

## Retry And Dead Letter

- 业务 Handler 的暂时性错误按配置进行进程内退避，并转发到 `<topic>.retry`。
- 达到跨 Topic 重试上限后转发到 `<topic>.dead`。
- JSON 损坏、缺少 `event_type`、非法版本和未知主版本属于永久 envelope 错误，跳过业务 Handler 并直接进入 `.dead`。
- 原消息 value 与 key 原样保留，便于修复后重放。
- DLQ headers 至少包含 `dead_reason`、`last_error`、`original_topic`、`retry_attempt`、`failed_at` 和原始 schema headers。
- DLQ 发布失败时不提交原 offset，避免静默丢失。

应用启动时显式创建主 Topic、`.retry` 和 `.dead`，不依赖 broker 自动建 Topic。

## 当前 Topic

Topic 清单位于 `internal/bootstrap/kafka_topics.go`。Message、Group、Conversation、Contact 与 Session 事件共享本规则；`message.*.created` 由 Transactional Outbox 发布，其 value 和 headers 在同一消息事务中固化。
