# Search Service 渐进部署手册

`cmd/services/search` 是只读消息检索服务，拥有 Elasticsearch read Alias 查询，并通过 Core Capability 获取认证用户的会话范围。Search Indexer 继续独立消费 Kafka mutation 并拥有 write Alias。

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
- Search RPC 只允许 `dipole-gateway`；Gateway 仅在 `search.enabled=true` 时注册公开搜索路由并建立 RPC 连接。

## 开发部署

先构建镜像并生成含 `dipole-search` 身份的开发证书：

```bash
scripts/generate-internal-certs.sh
scripts/docker-build.sh build
DIPOLE_INTERNAL_RPC_SHARED_SECRET=<secret> \
  docker compose --profile search -f deploy/compose/docker-compose.microservices.yml up -d --wait
```

`search` profile 增加 Elasticsearch、Search Indexer 和 Search Service。默认 Compose 不启用该 profile，现有 Core/Message/Gateway 冷启动路径保持不变。先启动存储与内部服务，再灰度重建 Gateway：

```bash
DIPOLE_INTERNAL_RPC_SHARED_SECRET=<secret> \
  docker compose --profile search -f deploy/compose/docker-compose.microservices.yml \
  up -d --wait elasticsearch search-indexer search

DIPOLE_INTERNAL_RPC_SHARED_SECRET=<secret> DIPOLE_SEARCH_ENABLED=true \
  docker compose --profile search -f deploy/compose/docker-compose.microservices.yml \
  up -d --wait --force-recreate gateway
```

公开接口：

```http
GET /api/v1/messages/search?q=migration&limit=20
Authorization: Bearer <token>
```

`q` 为 1..256 个 Unicode 字符，`limit` 范围为 1..100。Gateway 从 JWT 会话取得 principal，不接受 user ID 或 conversation scope 参数。

真实存储契约：

```bash
scripts/smoke-search-service.sh
```

脚本在隔离 Elasticsearch 9.5.2 中写入一个可见文档和一个越权文档，经 Core scope、Search RPC 与 read Alias 查询后只返回可见文档，并自动清理容器与 volume。

### Remote GPU 体验栈

长驻 `dipole-experience` 先启动 `search` 服务，再使用版本化
`search-experience.yml` 只重建 Gateway。release snapshot、体验 `.env`、mTLS
证书和 Agent overlay 都必须显式传入：

```bash
release=/home/admin1/agent/releases/dipole-<revision>
work=/home/admin1/workspaces/Dipole

DIPOLE_INTERNAL_CERT_DIR="$work/certs/internal" \
docker compose --env-file "$work/.env" -p dipole-experience \
  -f "$release/deploy/compose/docker-compose.microservices.yml" \
  -f "$work/deploy/microservices/agent-experience.yml" \
  --profile search up -d --no-deps search

DIPOLE_INTERNAL_CERT_DIR="$work/certs/internal" \
docker compose --env-file "$work/.env" -p dipole-experience \
  -f "$release/deploy/compose/docker-compose.microservices.yml" \
  -f "$work/deploy/microservices/agent-experience.yml" \
  -f "$release/deploy/microservices/search-experience.yml" \
  --profile search up -d --no-deps gateway
```

验收先确认未认证请求得到 `401`，再以两个不同用户验证一方能检索自己的会话消息、无法检索另一方的私聊。回滚时使用相同的基础文件但移除
`search-experience.yml` 重建 Gateway；确认路由关闭后，再停止 `search` 服务。共享 project 不使用 `--remove-orphans`。

## 验收与回滚

上线前确认 Search Indexer lag 为零、read/write Alias 只有一个共同 owner、Search 与 Core mTLS 身份匹配。Search Service 启动失败时保持 Gateway 搜索入口关闭。回滚时先以 `DIPOLE_SEARCH_ENABLED=false` 重建 Gateway，再停止 `search` profile；索引与消息主链路无需逆向迁移。
