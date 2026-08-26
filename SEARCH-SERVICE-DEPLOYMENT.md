# Search Service 渐进部署手册

`cmd/search-service` 是只读消息检索服务，拥有 Elasticsearch read Alias 查询，并通过 Core Capability 获取认证用户的会话范围。Search Indexer 继续独立消费 Kafka mutation 并拥有 write Alias。

```text
Gateway -> Search RPC -> Core Capability -> MySQL metadata
                      -> Elasticsearch read Alias

Kafka -> Search Indexer -> Elasticsearch write Alias
```

## 边界

- Search Service 不初始化 MySQL、Redis 或 Kafka，也不接受调用方提供的 user ID、conversation keys。
- Core 根据 `RequestContext.principal_user_id` 返回私聊与当前群成员范围；空范围直接返回空结果。
- Core 方法级策略只允许 `dipole-search` 调用 scope 方法，其他 User/Group/File 能力返回 `PermissionDenied`。
- 启动只读校验双 Alias 的唯一 owner 与 strict mapping，不创建索引、不切换 Alias。
- Search 不可用只影响检索，消息持久化、同步、实时投递和历史查询继续运行。
- 当前 Search RPC 只允许 `dipole-gateway`；公开 Gateway API 在下一独立里程碑接线。

## 开发部署

先构建镜像并生成含 `dipole-search` 身份的开发证书：

```bash
scripts/generate-internal-certs.sh
scripts/docker-build.sh build
DIPOLE_INTERNAL_RPC_SHARED_SECRET=<secret> \
  docker compose --profile search -f docker-compose.microservices.yml up -d --wait
```

`search` profile 增加 Elasticsearch、Search Indexer 和 Search Service。默认 Compose 不启用该 profile，现有 Core/Message/Gateway 冷启动路径保持不变。

真实存储契约：

```bash
scripts/smoke-search-service.sh
```

脚本在隔离 Elasticsearch 9.5.2 中写入一个可见文档和一个越权文档，经 Core scope、Search RPC 与 read Alias 查询后只返回可见文档，并自动清理容器与 volume。

## 验收与回滚

上线前确认 Search Indexer lag 为零、read/write Alias 只有一个共同 owner、Search 与 Core mTLS 身份匹配。Search Service 启动失败时保持 Gateway 搜索入口关闭；停止 `search` profile 即可回滚，索引与消息主链路无需逆向迁移。
