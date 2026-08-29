# 仓库结构

Dipole 采用面向服务边界的 Monorepo。目录结构先表达部署边界，再表达共享实现，方便从模块化单体渐进切换到独立服务。

## 服务入口

长期运行的 Go 服务统一放在 `cmd/services/`：

入口索引和服务职责见 [`cmd/services/README.md`](../../cmd/services/README.md)，数据所有权和过渡例外见[服务边界清单](SERVICE-BOUNDARIES.md)。

| 目录 | 服务职责 | 当前制品 |
| --- | --- | --- |
| `cmd/services/core` | 用户、群组、联系人、文件和会话核心 | `dipole-server` |
| `cmd/services/gateway` | HTTP、WebSocket、认证上下文、消息/同步查询和实时投递 | `dipole-gateway` |
| `cmd/services/message` | 消息命令、历史、幂等、Outbox 和 Message Store | `dipole-message` |
| `cmd/services/sync` | 用户 Inbox Timeline 和设备同步 | `dipole-sync` |
| `cmd/services/search` | Elasticsearch 只读查询 | `dipole-search` |
| `cmd/services/search-indexer` | Kafka 消费和 Elasticsearch 写入 | `dipole-search-indexer` |

一次性迁移、回填、对账、证据采集和本地诊断工具统一放在 `cmd/tools/`。它们不属于长期服务部署单元，构建时仍生成原有二进制名称。

## 共享代码与契约

- `internal/` 当前存放 Go 服务共享的领域、应用、数据访问和传输实现，属于渐进迁移中的共享实现区；它不代表所有服务可以任意依赖彼此的业务实现。
- `internal/application`、`internal/model`、`internal/platform` 是优先允许共享的基础层；`internal/app`、`internal/service`、`internal/handler`、`internal/store` 的服务归属以[服务边界清单](SERVICE-BOUNDARIES.md)为准。
- 已完成物理收敛的服务实现应放在 `internal/services/<service>/`；当前 Core capability/User/Contact/Group/Conversation application、Search、Message、Sync application 分别位于 `internal/services/core/application/`、`internal/services/search/application/`、`internal/services/message/application/`、`internal/services/sync/application/`，通用 `internal/app` 只保留兼容装配入口。
- Gateway 专属 HTTP 边缘适配器位于 `internal/gateway/`；Search HTTP handler 已从通用 `internal/handler/http` 收敛到 Gateway 包。
- `api/proto/` 存放跨服务 RPC 契约及生成代码。
- `contracts/` 存放事件、Agent 和运行时边界契约。
- `db/` 存放迁移、sqlc 查询和数据库结构。
- `frontend/` 存放客户端；`services/agent-runtime/` 和 `services/realtime-delivery/` 存放非 Go 服务源码及其独立构建入口。
- `deploy/images/` 存放按服务边界构建的镜像模板；`deploy/microservices/` 存放可组合的微服务部署 override。
- `docs/` 存放架构、Agent、数据、运行、前端、性能、指南和参考材料；`docs/` 顶层只保留索引、清单和架构图。

## 非运行时目录

- `acc/` 是本地参考项目区，不属于 Dipole 的编译图和部署拓扑。
- `benchmarks/` 只保存可复核的性能、故障和迁移证据，每个证据目录应包含 README、报告和校验清单。
- `design/` 保存 Pencil 源文件、设计变更记录和导出图；前端实现位于 `frontend/`。
- `scripts/` 保存测试、Smoke、迁移和运维门禁脚本，不承载长期服务入口；微服务镜像脚本为每个部署单元生成独立制品。
- `tmp/` 只用于本地临时数据，不应提交业务源文件。

## 多语言服务目录

- `cmd/services/` 只放 Go 长期运行服务的 `main.go` 入口。
- `services/` 只放需要独立 module/toolchain 的服务源码：TypeScript Agent Runtime 和 C++ Realtime Delivery。
- 服务目录内允许保留语言生态所需的 `package.json`、`go.mod`、`CMakeLists.txt` 和本地测试配置；跨服务协议仍只能来自 `api/proto/` 或 `contracts/`。
- 根目录禁止重新出现 `services/agent-runtime/`、`services/realtime-delivery/` 两个服务源码目录。

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

Go 单服务镜像使用 `scripts/docker-build-microservice-images.sh` 构建；`docker-compose.dist.yml` 和 `DIPOLE_*_IMAGE` 可作为 legacy 回滚路径。
