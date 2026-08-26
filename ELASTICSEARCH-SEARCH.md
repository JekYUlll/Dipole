# Elasticsearch Message Search

本文档记录 A5 Message Search Projection 的版本化索引、Alias、外部版本、作用域查询和渐进接线边界。独立 Search Indexer 已具备真实 Kafka/Elasticsearch 链路；查询 API 和生产流量开关保持关闭。

## Index Contract

物理索引和 Alias 使用稳定命名：

```text
dipole-messages-v1
  ├─ dipole-messages-read
  └─ dipole-messages-write  (is_write_index=true)
```

v1 mapping 位于 `internal/data/elasticsearch/schema/message_search_v1.json`，启用 `dynamic: strict` 并固定以下字段：

| 字段 | 类型 | 用途 |
| --- | --- | --- |
| `message_uuid` | keyword | 文档业务身份，同时作为 Elasticsearch `_id` |
| `conversation_key` | keyword | 权限过滤后的会话作用域 |
| `message_seq` | long | 会话内消息位置 |
| `revision` | long | mutation 外部版本 |
| `sender_uuid` | keyword | 发送者过滤与展示 |
| `message_type` | byte | 消息类型 |
| `content` | text | 全文检索正文 |
| `sent_at` | date | 结果排序 |
| `searchable` | boolean | 正文与 tombstone 过滤 |
| `payload_hash` | keyword, index=false | 重放与冲突分类 |

`Bootstrap` 首次创建物理索引并同时绑定 read/write Alias；重复启动会校验 strict mapping、全部字段类型、`payload_hash` 的非索引属性和 write Alias 所有权。Schema 漂移会使 readiness 失败，应用不会自动修改已有 mapping。

## Revision And Replay

写入固定使用：

```text
PUT /dipole-messages-write/_doc/{message_id}
  ?require_alias=true
  &version={revision}
  &version_type=external
```

`require_alias=true` 防止误写物理索引。外部 revision 阻止较旧 mutation 覆盖较新文档；收到 409 后，adapter 读取当前 revision 与稳定 payload hash 并分类：

| 状态 | 处理 |
| --- | --- |
| 当前 revision 更高 | 旧事件安全 no-op |
| revision 与 hash 均相同 | 幂等重放成功 |
| revision 相同但 hash 不同 | 返回 `ErrProjectionConflict` |
| 当前 revision 更低 | 返回异常状态错误，不确认事件 |

`SearchIndex.Apply(MessageSearchMutation)` 是统一写契约。created/edited 生成 `searchable=true` 的完整状态；recalled/deleted 生成 `searchable=false` 的最小 tombstone，并通过相同 external revision 规则长期阻挡旧正文事件复活。MySQL 逻辑索引使用 `000007_versioned_search_mutations` 提供相同状态机。

## Scoped Search

Search adapter 要求非空 `conversation_keys` 与检索文本。会话列表经去空、去重和排序后进入 Elasticsearch `terms` filter，文本进入 `match` query，结果按 `sent_at DESC, message_uuid DESC` 排序，单页限制为 100。

调用方仍需先通过 Core 计算当前 principal 可访问的会话集合。Elasticsearch 不承担成员关系事实；空 scope 必须 fail closed，索引结果不能扩大授权范围。

## Search Indexer Runtime

`cmd/search-indexer` 是独立部署单元，启动顺序固定为 Elasticsearch 配置校验、v1 mapping/Alias readiness、Kafka 初始化、八类 Topic 注册、consumer assignment 和 metrics listener。默认 `elasticsearch.enabled=false`，不会进入 Core、Message 或 Gateway Composition Root。

订阅 Topic：

```text
message.{direct,group}.{created,edited,recalled,deleted}
```

created/edited 映射为 searchable mutation，recalled/deleted 映射为 tombstone。consumer group 固定为 `dipole-search-indexer-consumer`；处理失败沿用平台 retry/DLQ 机制，确认 offset 前必须完成 Elasticsearch Apply。运行配置支持 Basic Auth 或 API Key，两种认证不能同时启用。

## Alias Migration

新 mapping 使用新物理索引构建，例如 `dipole-messages-v2`。完成回填和对账后，adapter 先验收目标 mapping，再通过单次 `_aliases` 请求原子移除旧 read/write Alias 并绑定新索引，write Alias 显式设置 `is_write_index=true`。回滚使用同一原子操作反向切换。

索引切换前需要保留：源 checkpoint、目标文档数量/hash、构建开始时间、owner、回滚截止时间和旧索引保留期。ES 故障不得阻断消息持久化，Kafka lag 与 DLQ 负责暴露投影滞后。

## Verified Contract

单元 contract 覆盖：

- v1 strict mapping 与 read/write Alias 创建。
- 已有 mapping/alias readiness 校验和 drift 拒绝。
- 四动作原子 Alias 切换请求。
- 首次写入、相同 revision 重放、旧 revision no-op、同 revision payload 冲突和 tombstone 防复活。
- conversation scope 去重、排序、fail closed 与 100 条上限。
- ES 错误响应有界读取，避免诊断内容无限进入内存。
- Search Projector 的八类 Topic 映射、legacy created 默认、channel/target 冲突与 adapter 失败传播。

真实 Elasticsearch 9.5.2 contract 覆盖重复 Bootstrap、external revision 更新与重放、旧事件 no-op、tombstone 防复活、刷新后检索和隐藏会话无结果。三节点 Kafka 端到端 smoke 进一步验证 created r1、recalled r3 与迟到 edited r2 最终保持 revision 3 tombstone。测试入口：

```bash
DIPOLE_TEST_ELASTICSEARCH_URL=http://127.0.0.1:9200 \
  go test -count=1 -run TestIndexContract -v ./internal/data/elasticsearch

scripts/smoke-search-indexer.sh
```

## Next Milestones

1. 增加固定快照 Backfill、数量/hash Reconcile 和 Alias 切换命令。
2. 通过 Core Capability 计算搜索 scope，再开放内部 Search RPC 与 Gateway API。

## References

- [Elasticsearch Index API external versioning](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-index)
- [Elasticsearch aliases](https://www.elastic.co/guide/en/elasticsearch/reference/current/aliases.html)
