# Dipole 服务入口

本目录只存放长期运行的服务入口。服务的业务实现、Composition Root 和部署配置分别位于 `internal/`、`internal/bootstrap/` 与 `deploy/`；入口目录不直接持有数据库访问代码。

服务边界、数据所有权和当前过渡例外以 [服务边界清单](../../docs/architecture/SERVICE-BOUNDARIES.md) 为准。

| 服务 | 入口 | 主要职责 |
| --- | --- | --- |
| Core | `core/main.go` | 用户、群组、联系人、文件、认证和会话核心 |
| Gateway | `gateway/main.go` | HTTP、WebSocket、认证上下文、限流和实时连接 |
| Message | `message/main.go` | 消息命令、历史、幂等、Outbox 和 Message Store |
| Sync | `sync/main.go` | User Inbox Timeline、设备 Cursor 和同步查询 |
| Search | `search/main.go` | Elasticsearch 查询适配 |
| Search Indexer | `search-indexer/main.go` | Kafka 消费和 Elasticsearch 索引投影 |

一次性迁移、回填、对账和证据工具统一位于 `cmd/tools/`，不应放回本目录。
