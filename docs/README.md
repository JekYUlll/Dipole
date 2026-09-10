# Dipole 文档

## 从这里开始

- [项目介绍与演示脚本](guides/PROJECT-LEARNING-AND-INTERVIEW.md)
- [Dipole IM 面试问答](guides/INTERVIEW-IM.md)
- [Dipole Agent 面试问答](guides/INTERVIEW-AGENT.md)
- [服务边界](architecture/SERVICE-BOUNDARIES.md)
- [微服务部署](architecture/MICROSERVICES-DEPLOYMENT.md)

## 核心设计

| 主题 | 文档 | 关注点 |
| --- | --- | --- |
| 消息与同步 | [MESSAGE-STORAGE-AND-SYNC.md](architecture/MESSAGE-STORAGE-AND-SYNC.md) | 会话序列、Inbox Timeline、Read Seq、Device Cursor |
| 服务协作 | [SERVICE-BOUNDARIES.md](architecture/SERVICE-BOUNDARIES.md) | Gateway、Core、Message、Sync、Search、Agent 的数据所有权 |
| Agent Runtime | [AGENT-RUNTIME-DESIGN.md](architecture/AGENT-RUNTIME-DESIGN.md) | ExecutionContext、Capability、Temporal、Approval、Artifact |
| 数据访问 | [DATA-ACCESS-MIGRATION.md](data/DATA-ACCESS-MIGRATION.md) | `database/sql`、sqlc、迁移与事务 |
| 事件与搜索 | [KAFKA-EVENT-CONTRACT.md](data/KAFKA-EVENT-CONTRACT.md) | Outbox、Kafka 事件和 Search projection |
| 文件 | [对象存储与文件模块](operations/MESSAGE-SERVICE-DEPLOYMENT.md) | MinIO、分片上传和访问授权 |

## 运行与验证

- [部署拓扑](architecture/MICROSERVICES-DEPLOYMENT.md)
- [Gateway 运行说明](operations/GATEWAY-DEPLOYMENT.md)
- [Message Service 运行说明](operations/MESSAGE-SERVICE-DEPLOYMENT.md)
- [Search Service 运行说明](operations/SEARCH-SERVICE-DEPLOYMENT.md)
- [性能基线](performance/PERFORMANCE-BASELINE.md)

## 参考材料

`architecture/`、`data/`、`operations/`、`agent/` 和 `performance/` 保留设计细节、
运行手册、实验记录与历史证据。它们服务于实现和维护；项目介绍、演示与面试请优先
从本页的“从这里开始”和“核心设计”进入。

- [历史文档归档](archive/README.md)

架构文档清单由 `architecture-docs.manifest` 管理：

```bash
scripts/check-architecture-docs.sh
```
