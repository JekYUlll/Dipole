# Cassandra 与 Elasticsearch 隔离实验环境

本文档描述外部存储实验环境。该环境用于验证镜像、资源基线、健康检查和最小 CRUD，不接入默认 Core、Message、Gateway 生产拓扑。

## Scope

```text
docker-compose.storage-lab.yml
        │
        ├── Cassandra 5.0.9 / single node / 512 MiB heap
        └── Elasticsearch 9.5.2 / single node / 512 MiB heap

Core / Gateway ── no connection ──► Storage Lab
Message ── optional query-only shadow-read ──► Cassandra
```

当前约束：

- A3 已引入 Apache Cassandra GoCQL driver、Timeline schema primitive、独立 Projector Runtime 和一次性 Backfill 作业。
- Projector 默认关闭；Backfill 仅由运维显式执行；Core 与 Gateway 不创建 Cassandra session。
- MySQL 继续承担 MessageStore 和 SearchIndex 的生产职责。
- Elasticsearch 仍没有正式索引模板或 Runtime；Cassandra 已具备固定快照对账和默认关闭的 Message query-only shadow-read。
- Cassandra 与 Elasticsearch 数据卷仅属于独立 Compose project，不与默认开发环境共享。

## Smoke

执行：

```bash
./scripts/smoke-storage-lab.sh
```

脚本执行以下门禁：

1. 扫描应用配置和 Bootstrap，确认 Cassandra 与 Message shadow-read 默认关闭，并拒绝 Core/Gateway 直接接入。
2. 启动 Cassandra 与 Elasticsearch 并等待服务级健康检查。
3. 在 Cassandra 创建临时 keyspace 和按会话/Seq 排序的 probe 表，写入并读取一行后删除 keyspace。
4. 在 Elasticsearch 创建 strict mapping 的临时索引，写入、刷新、搜索一份文档后删除索引。
5. 删除隔离容器、网络和数据卷。

调试时可保留现场：

```bash
KEEP_STACK=1 ./scripts/smoke-storage-lab.sh
```

Elasticsearch 镜像体积较大，首次拉取耗时明显；默认 Compose、微服务 Compose 和其他 cluster profile 都不会隐式启动该环境。

## Completed A3 Gates

A3 已完成 bucket rollover、Seq 聚簇排序、事件幂等、Projector、历史 Backfill、固定快照对账和 query-only shadow-read。客户端仍读取 MySQL；具体启用与回滚顺序见 [Cassandra Timeline 文档](../architecture/CASSANDRA-TIMELINE.md)。

A5 引入 Elasticsearch Search Projection 时再定义正式 analyzer、mapping、alias/version、重建和切换协议。

## References

- [Apache Cassandra data modeling](https://cassandra.apache.org/doc/latest/cassandra/developing/data-modeling/intro.html)
- [Run Elasticsearch in Docker](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-basic)
- [Elasticsearch index templates](https://www.elastic.co/docs/manage-data/data-store/templates)
