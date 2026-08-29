# 文档目录

Dipole 的文档按生命周期和职责分类维护。架构契约由 `docs/architecture-docs.manifest` 管理，检查入口为：

```bash
scripts/check-architecture-docs.sh
```

## 架构与演进

- [平台演进计划](architecture/PLATFORM-EVOLUTION-PLAN.md)
- [架构债务台账](architecture/ARCHITECTURE-DEBT.md)
- [微服务部署拓扑](architecture/MICROSERVICES-DEPLOYMENT.md)
- [Sync Service](architecture/SYNC-SERVICE.md)
- [Cassandra Timeline](architecture/CASSANDRA-TIMELINE.md)
- [Realtime Delivery](architecture/REALTIME-DELIVERY.md)
- [Agent Runtime 设计](architecture/AGENT-RUNTIME-DESIGN.md)
- [仓库结构](architecture/REPOSITORY-STRUCTURE.md)

## 数据与存储

- [GORM 到 sqlc 迁移计划](data/DATA-ACCESS-MIGRATION.md)
- [Kafka 事件契约](data/KAFKA-EVENT-CONTRACT.md)
- [Kafka Cluster](data/KAFKA-CLUSTER.md)
- [MySQL Cluster](data/MYSQL-CLUSTER.md)
- [Redis Cluster](data/REDIS-CLUSTER.md)
- [Cassandra 消息归档运行手册](data/CASSANDRA-MESSAGE-ARCHIVE-RUNBOOK.md)
- [Elasticsearch Search](data/ELASTICSEARCH-SEARCH.md)
- [Search Archive 运行手册](data/SEARCH-ARCHIVE-RUNBOOK.md)
- [Storage Lab](data/STORAGE-LAB.md)

## 部署与运行

- [Gateway 部署](operations/GATEWAY-DEPLOYMENT.md)
- [Message Service 部署](operations/MESSAGE-SERVICE-DEPLOYMENT.md)
- [Search Service 部署](operations/SEARCH-SERVICE-DEPLOYMENT.md)
- [Web Sync 灰度与回滚](operations/WEB-SYNC-ROLLOUT.md)
- [重复消息 hydration 灰度](operations/DUPLICATE-HYDRATION-ROLLOUT.md)

## Agent Runtime

Agent 的协议、记忆、MCP、观测和事件触发材料集中在 `docs/` 与 `contracts/`：

- [Agent Artifact 对账](agent-artifact-reconcile.md)
- [Agent 外部 MCP](agent-external-mcp.md)
- [Agent MCP 授权](agent-mcp-authorization.md)
- [Agent Memory Observation](agent-memory-observation.md)
- [Agent OpenTelemetry 运维](agent-otel-operations.md)
- [Agent Subscription Shadow](agent-subscription-shadow.md)
- [Agent Timeline Repair 运维](AGENT-TIMELINE-REPAIR-OPERATIONS.md)

## 前端设计

- [前端设计计划](frontend/FRONTEND-DESIGN-PLAN.md)
- [Pencil 设计资产](../design/README.md)

## 性能记录

- [性能基线](performance/PERFORMANCE-BASELINE.md)
- [Benchmark 目录](../benchmarks/)

## 本地参考与历史材料

部分早期方案、问答和本地参考文件通过 `.gitignore` 单文件规则保留，不承担当前实现契约。它们转为正式文档前，需要先完成代码、配置、运行手册和测试对齐，再加入架构清单。
