# Cassandra 与 Elasticsearch 隔离实验环境

本文档描述 A2 阶段的外部存储实验环境。该环境用于验证镜像、资源基线、健康检查和最小 CRUD，不接入 Core、Message、Gateway 或任何生产读写路径。

## Scope

```text
docker-compose.storage-lab.yml
        │
        ├── Cassandra 5.0.9 / single node / 512 MiB heap
        └── Elasticsearch 9.5.2 / single node / 512 MiB heap

Dipole Runtime ── no connection ──► Storage Lab
```

当前约束：

- `internal/config` 和 `internal/bootstrap` 没有 Cassandra / Elasticsearch 配置或客户端。
- `go.mod` 没有 Cassandra / Elasticsearch driver。
- MySQL 继续承担 MessageStore 和 SearchIndex 的生产职责。
- A2 不创建正式 Timeline 表、索引模板、Kafka projector 或 shadow-read。
- Cassandra 与 Elasticsearch 数据卷仅属于独立 Compose project，不与默认开发环境共享。

## Smoke

执行：

```bash
./scripts/smoke-storage-lab.sh
```

脚本执行以下门禁：

1. 扫描应用配置、Bootstrap 和 Go 依赖，拒绝提前接线。
2. 启动 Cassandra 与 Elasticsearch 并等待服务级健康检查。
3. 在 Cassandra 创建临时 keyspace 和按会话/Seq 排序的 probe 表，写入并读取一行后删除 keyspace。
4. 在 Elasticsearch 创建 strict mapping 的临时索引，写入、刷新、搜索一份文档后删除索引。
5. 删除隔离容器、网络和数据卷。

调试时可保留现场：

```bash
KEEP_STACK=1 ./scripts/smoke-storage-lab.sh
```

Elasticsearch 镜像体积较大，首次拉取耗时明显；默认 Compose、微服务 Compose 和其他 cluster profile 都不会隐式启动该环境。

## Next Gates

A3 进入 Cassandra 影子投影前，需要单独评审并验证：

- `conversation_id + bucket` 分区边界与 bucket rollover 规则。
- `conversation_seq` 聚簇排序、重复事件幂等和乱序写入语义。
- Outbox/Kafka projector checkpoint、回填、lag 与失败重试。
- MySQL/Cassandra 数量、Seq、哈希和抽样内容对账。
- shadow-read 只记录差异，客户端仍读取 MySQL。

A5 引入 Elasticsearch Search Projection 时再定义正式 analyzer、mapping、alias/version、重建和切换协议。

## References

- [Apache Cassandra data modeling](https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html)
- [Run Elasticsearch in Docker](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-basic)
- [Elasticsearch index templates](https://www.elastic.co/docs/manage-data/data-store/templates)
