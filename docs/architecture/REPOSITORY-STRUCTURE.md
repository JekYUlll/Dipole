# 仓库结构

Dipole 采用面向服务边界的 Monorepo。目录结构先表达部署边界，再表达共享实现，方便从模块化单体渐进切换到独立服务。

## 服务入口

长期运行的 Go 服务统一放在 `cmd/services/`：

| 目录 | 服务职责 | 当前制品 |
| --- | --- | --- |
| `cmd/services/core` | 用户、群组、联系人、文件和会话核心 | `dipole-server` |
| `cmd/services/gateway` | HTTP、WebSocket、认证上下文和实时投递 | `dipole-gateway` |
| `cmd/services/message` | 消息命令、历史、幂等、Outbox 和 Message Store | `dipole-message` |
| `cmd/services/sync` | 用户 Inbox Timeline 和设备同步 | `dipole-sync` |
| `cmd/services/search` | Elasticsearch 只读查询 | `dipole-search` |
| `cmd/services/search-indexer` | Kafka 消费和 Elasticsearch 写入 | `dipole-search-indexer` |

一次性迁移、回填、对账、证据采集和本地诊断工具统一放在 `cmd/tools/`。它们不属于长期服务部署单元，构建时仍生成原有二进制名称。

## 共享代码与契约

- `internal/` 存放 Go 服务共享的领域、应用、数据访问和传输实现。
- `api/proto/` 存放跨服务 RPC 契约及生成代码。
- `contracts/` 存放事件、Agent 和运行时边界契约。
- `db/` 存放迁移、sqlc 查询和数据库结构。
- `frontend/` 存放客户端；`agent-runtime/` 存放 TypeScript Agent 服务。
- `docs/` 存放架构、Agent、数据、运行、前端、性能、指南和参考材料；`docs/` 顶层只保留索引、清单和架构图。

## 非运行时目录

- `acc/` 是本地参考项目区，不属于 Dipole 的编译图和部署拓扑。
- `benchmarks/` 只保存可复核的性能、故障和迁移证据，每个证据目录应包含 README、报告和校验清单。
- `design/` 保存 Pencil 源文件、设计变更记录和导出图；前端实现位于 `frontend/`。
- `scripts/` 保存测试、Smoke、迁移和运维门禁脚本，不承载长期服务入口。
- `tmp/` 只用于本地临时数据，不应提交业务源文件。

## Compose 配置层级

- `docker-compose.yml`：最小本地开发拓扑。
- `docker-compose.microservices.yml`：服务边界、Kafka、同步、搜索和 Agent 的集成拓扑。
- `docker-compose.dist.yml`：分发/部署镜像拓扑。
- `docker-compose.cluster.yml`、`docker-compose.mysql-cluster.yml`、`docker-compose.redis-cluster.yml`：集群和故障演练拓扑。
- `docker-compose.storage-lab.yml`：隔离存储实验，不作为默认运行入口。

Compose 文件暂时保留在根目录，以保持 Docker Compose 的直接调用习惯；职责和优先级由本节固定，后续如迁移到 `deploy/compose/` 必须同步更新所有运行手册和脚本。

## 结构门禁

新增长期运行服务时，应同时完成入口目录、构建脚本、Compose 配置、运行手册和测试门禁更新：

```bash
scripts/check-service-layout.sh
```
