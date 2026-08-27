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

调用方必须先通过 Core `ListSearchConversationKeys` 计算当前 principal 可访问的会话集合。该 RPC 只接受受认证 `RequestContext.principal`，不接受独立 user ID 或调用方提供的 conversation keys。Core 返回私聊 Conversation 投影，以及 principal 仍具成员关系的 normal/dismissed 群；陈旧群会话投影不会授予访问权。

Elasticsearch 不承担成员关系事实；空 scope 必须 fail closed，索引结果不能扩大授权范围。后续 Search Service 只接收检索文本和分页参数，并在每次查询时向 Core 获取 scope。

## Search Indexer Runtime

`cmd/search-indexer` 是独立部署单元，启动顺序固定为 Elasticsearch 配置校验、v1 mapping/Alias readiness、Kafka 初始化、八类 Topic 注册、consumer assignment 和 metrics listener。默认 `elasticsearch.enabled=false`，不会进入 Core、Message 或 Gateway Composition Root。

订阅 Topic：

```text
message.{direct,group}.{created,edited,recalled,deleted}
```

created/edited 映射为 searchable mutation，recalled/deleted 映射为 tombstone。consumer group 固定为 `dipole-search-indexer-consumer`；处理失败沿用平台 retry/DLQ 机制，确认 offset 前必须完成 Elasticsearch Apply。运行配置支持 Basic Auth 或 API Key，两种认证不能同时启用。

## Search Query Runtime

`cmd/search-service` 是独立只读查询进程。内部 `dipole.search.v1.SearchService` 请求只包含认证上下文、查询文本和页大小；Search Application 每次向 Core 获取 principal scope，空 scope 不访问 Elasticsearch。启动通过 `ValidateReadiness` 动态发现当前双 Alias 的唯一物理 owner 并校验 strict mapping，不创建索引或修改 Alias。

Search Service 不初始化 MySQL、Redis 或 Kafka；Core/Message/Gateway 也不直接构造 Elasticsearch adapter。内部 RPC 只允许 Gateway 调用。Gateway 在 `search.enabled=true` 时注册认证 `GET /api/v1/messages/search`，从 JWT 会话取得 principal 并转发 1..256 字符的查询文本与 1..100 的 limit；依赖故障返回有界 502。

Web 搜索入口通过构建变量 `VITE_SEARCH_ENABLED=true` 启用。该变量必须与 Gateway `search.enabled=true` 同步发布；默认关闭时继续保留原会话筛选和聊天链路。前端工作区对请求执行 300ms 防抖并丢弃过期响应，Search 故障只显示局部错误态。

```bash
VITE_SEARCH_ENABLED=true scripts/docker-build.sh frontend
DIPOLE_SEARCH_ENABLED=true docker compose -f docker-compose.microservices.yml --profile search up -d
```

## Alias Migration

新 mapping 使用新物理索引构建，例如 `dipole-messages-v2`。完成回填和对账后，adapter 先验收两个生产 Alias 全局只有一个 owner，并校验目标 mapping，再通过单次 `_aliases` 请求原子移除旧 read/write Alias 并绑定新索引，write Alias 显式设置 `is_write_index=true`。remove action 使用 `must_exist=true`，并发运维或分裂 owner 会让整个请求失败。请求成功后的 owner 验收失败会触发反向原子补偿；回滚也使用同一受控路径。

`cmd/search-alias` 要求显式确认维护窗口，并在 Reconcile 前、Alias 操作前、Alias 操作后三次确认 Outbox 高水位仍等于已完成任务的固定快照。切换后的检查发现漂移时，命令立即反向切回原索引；补偿失败与原始错误会一并返回。成功输出 JSON receipt，包含 job、from/to、固定高水位、文档计数、切换时间与回滚截止时间。

安全切换流程：

1. 暂停产生 Message mutation 的业务写入；只停止 Search Indexer 无法冻结 Outbox 高水位。
2. 对目标物理索引运行一个新的 Backfill job，使固定高水位追平维护窗口起点。
3. 运行 Reconcile 并确认一致。
4. 执行 `dipole-search-alias -action switch ... -confirm-maintenance-window`。
5. 保存 receipt，恢复业务写入，并至少保留旧索引到 `rollback_until`。

回滚前需要在新的维护窗口内对旧索引运行新的 Backfill/Reconcile job，再执行 `-action rollback` 将新索引作为 `from`、旧索引作为 `to`。命令不会直接删除旧索引，也不会允许用陈旧 checkpoint 回切。ES 故障不得阻断消息持久化，Kafka lag 与 DLQ 负责暴露投影滞后。

## Backfill And Reconciliation

`cmd/search-backfill` 从 Transactional Outbox 捕获一次固定的 Message mutation ID 高水位，并在该快照内按 Message UUID 只选择最终事件。created/edited 恢复完整 searchable 文档，recalled/deleted 恢复持久 tombstone；同一消息的旧 revision 不进入目标构建批次。

任务状态保存在 `search_backfill_jobs`，记录目标物理索引、owner lease、固定高水位、单调 checkpoint、attempt 和失败原因。任务名与目标索引绑定，不能将已完成 checkpoint 复用于另一个构建索引。构建目标名称必须位于当前 `index_prefix-messages-*` 命名空间，创建时不绑定生产 read/write Alias。

```bash
dipole-search-backfill \
  -job message-search-v1-build-20260827 \
  -target-index dipole-messages-v1-build-20260827

dipole-search-reconcile \
  -job message-search-v1-build-20260827 \
  -target-index dipole-messages-v1-build-20260827
```

Reconcile 只接受已完成任务的原始高水位，刷新目标索引后逐条比较 revision、searchable 与 canonical payload hash，并同时比较源状态数、目标文档数、缺失数和额外文档数。报告一致时退出 0，执行错误退出 1，确认差异退出 2。

当前恢复源要求已发布 Outbox 事件持续保留；归档与清理契约记录在 `AD-021`。在线 Indexer 只写当前 write Alias，因此生产 Alias 切换采用显式业务写维护窗口。未来若需要无停写切换，可在不改变 Backfill/Reconcile 语义的前提下增加双写 build target 与 source event watermark。

## Verified Contract

单元 contract 覆盖：

- v1 strict mapping 与 read/write Alias 创建。
- 已有 mapping/alias readiness 校验和 drift 拒绝。
- 四动作原子 Alias 切换请求。
- 首次写入、相同 revision 重放、旧 revision no-op、同 revision payload 冲突和 tombstone 防复活。
- conversation scope 去重、排序、fail closed 与 100 条上限。
- ES 错误响应有界读取，避免诊断内容无限进入内存。
- Search Projector 的八类 Topic 映射、legacy created 默认、channel/target 冲突与 adapter 失败传播。
- 物理构建索引无生产 Alias、显式 direct write、refresh、lookup 与 count。
- 固定 Outbox 最终状态快照、owner lease、失败恢复、目标绑定和缺失/hash/额外文档对账。
- 维护窗口确认、新鲜快照三重检查、Alias owner CAS、切换后验收、自动反向补偿与 rollback receipt。
- Core Search scope 只由认证 principal 派生，并排除无成员关系的陈旧群会话投影。

真实 Elasticsearch 9.5.2 contract 覆盖重复 Bootstrap、external revision 更新与重放、旧事件 no-op、tombstone 防复活、刷新后检索和隐藏会话无结果。三节点 Kafka 端到端 smoke 进一步验证 created r1、recalled r3 与迟到 edited r2 最终保持 revision 3 tombstone。测试入口：

```bash
DIPOLE_TEST_ELASTICSEARCH_URL=http://127.0.0.1:9200 \
  go test -count=1 -run TestIndexContract -v ./internal/data/elasticsearch

scripts/smoke-search-indexer.sh
scripts/smoke-search-backfill.sh
```

## Next Milestones

1. 增加围绕指定 `conversation_seq` 拉取上下文的历史接口，使搜索结果可以精确定位消息。
2. 建立 Playwright 键盘、响应式和视觉回归门禁，对比 `design/exports/search-v1`。
3. 在有零停写需求时增加双写 build target 与可证明的 source event watermark。

## References

- [Elasticsearch Index API external versioning](https://www.elastic.co/docs/api/doc/elasticsearch/operation/operation-index)
- [Elasticsearch aliases](https://www.elastic.co/guide/en/elasticsearch/reference/current/aliases.html)
