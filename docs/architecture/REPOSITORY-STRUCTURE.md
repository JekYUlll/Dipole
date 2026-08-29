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

一次性迁移、回填、对账、证据采集和本地诊断工具暂留在 `cmd/` 顶层。它们不属于长期服务部署单元，后续可在工具生命周期稳定后单独归档到 `cmd/tools/`。

## 共享代码与契约

- `internal/` 存放 Go 服务共享的领域、应用、数据访问和传输实现。
- `api/proto/` 存放跨服务 RPC 契约及生成代码。
- `contracts/` 存放事件、Agent 和运行时边界契约。
- `db/` 存放迁移、sqlc 查询和数据库结构。
- `frontend/` 存放客户端；`agent-runtime/` 存放 TypeScript Agent 服务。
- `docs/` 存放架构、数据、运行、前端和性能文档；根目录只保留项目入口和滚动更新日志。

## 结构门禁

新增长期运行服务时，应同时完成入口目录、构建脚本、Compose 配置、运行手册和测试门禁更新：

```bash
scripts/check-service-layout.sh
```
