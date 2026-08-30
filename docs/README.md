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
- [多语言服务目录](../services/README.md)
- [参考项目与演进原则](architecture/architecture-reference.md)
- [开发路线图](architecture/DEVELOPMENT-ROADMAP.md)
- [架构问答与审查](architecture/ARCHITECTURE-QA.md)
- [消息存储与同步模型](architecture/MESSAGE-STORAGE-AND-SYNC.md)
- [消息同步策略](architecture/MESSAGE-SYNC-STRATEGY.md)

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
- [缓存策略](data/CACHE-STRATEGY.md)

## 部署与运行

- [远程开发部署与压测](operations/REMOTE-DEV-DEPLOYMENT.md)
- [轻量微服务 Smoke](../scripts/smoke-microservices-lite.sh)
- [Gateway 部署](operations/GATEWAY-DEPLOYMENT.md)
- [Message Service 部署](operations/MESSAGE-SERVICE-DEPLOYMENT.md)
- [Search Service 部署](operations/SEARCH-SERVICE-DEPLOYMENT.md)
- [Web Sync 灰度与回滚](operations/WEB-SYNC-ROLLOUT.md)
- [重复消息 hydration 灰度](operations/DUPLICATE-HYDRATION-ROLLOUT.md)
- [TLS 配置](operations/TLS-SETUP.md)
- [待办事项](operations/TODO.md)

## Agent Runtime

Agent 的协议、记忆、MCP、观测和事件触发材料集中在 `docs/agent/` 与 `contracts/`：

- [Agent Artifact 对账](agent/agent-artifact-reconcile.md)
- [Agent 外部 MCP](agent/agent-external-mcp.md)
- [Agent MCP 授权](agent/agent-mcp-authorization.md)
- [Agent Memory Observation](agent/agent-memory-observation.md)
- [Agent OpenTelemetry 运维](agent/agent-otel-operations.md)
- [Agent Subscription Shadow](agent/agent-subscription-shadow.md)
- [Agent Timeline Repair 运维](agent/AGENT-TIMELINE-REPAIR-OPERATIONS.md)
- [Agent 前置能力清单](agent/ai-readiness-checklist.md)

## 前端设计

- [前端设计计划](frontend/FRONTEND-DESIGN-PLAN.md)
- [Pencil 设计资产](../design/README.md)

## 性能记录

- [性能基线](performance/PERFORMANCE-BASELINE.md)
- [Benchmark 目录](../benchmarks/)
- [负载测试报告](performance/LOAD-TEST-REPORT.md)

## 指南与参考

- [面试问答](guides/INTERVIEW-QA.md)
- [参考项目目录说明](references/README.md)
