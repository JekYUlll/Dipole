<p align="center">
  <img src="docs/images/dipole-wordmark.svg" width="560" alt="Dipole: an event-driven collaboration platform" />
</p>

<p align="center">
  Dipole 是一个面向实时协作与 Agent 能力演进的现代 IM 平台。
</p>

项目以 Go 承担 IM 领域服务，Kafka 连接异步事件与投影，MySQL 提供元数据和事务一致性，并逐步引入 Cassandra Timeline、Elasticsearch Search、Redis Realtime State 与独立的 TypeScript Agent Runtime。

## 项目定位

Dipole 采用渐进式微服务化路线：先以模块化单体保持开发效率，再沿 Gateway、Message、Sync、Search 和 Agent 边界逐步独立部署。核心设计关注消息幂等、会话序列、用户同步游标、事件投影、故障回切和可观测性。

当前语言职责如下：

| 区域 | 技术 | 职责 |
| --- | --- | --- |
| IM Core | Go | 用户、群组、消息、会话和一致性边界 |
| Agent Runtime | TypeScript / Node.js | Agent Task、工具调用、记忆、审批和工作流 |
| Frontend | TypeScript / Vue | IM 客户端和 Agent 交互界面 |
| Event Bus | Kafka | 领域事件、异步投影和服务解耦 |
| Data Layer | MySQL、Cassandra、Elasticsearch、Redis | 元数据、消息 Timeline、搜索和实时状态 |

## 架构概览

```text
Client -- WS/HTTP --> IM Gateway --> Message Service --> MySQL / Kafka
                                             |
                         +-------------------+-------------------+
                         |                   |                   |
                    Sync Service      Search Service       Agent Runtime
                         |                   |                   |
                    Timeline Store          ES             TS + MCP
```

服务会以兼容的本地实现开始，通过配置切换到独立服务。详细架构决策、迁移边界和当前状态见 [文档目录](docs/README.md)。

## 快速开始

启动基础服务并执行数据库迁移：

```bash
docker compose up -d mysql redis kafka
go run ./cmd/tools/migrate -direction up
go run ./cmd/services/core
```

启动前端：

```bash
cd frontend
nvm use
npm ci
npm run dev
```

生产数据访问使用 `database/sql + sqlc`。生成代码前安装固定版本的 sqlc：

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
scripts/sqlc.sh generate
```

## 验证

常用门禁：

```bash
scripts/check-go.sh
scripts/check-sqlc.sh
scripts/check-proto.sh
scripts/check-compose.sh
scripts/check-architecture-docs.sh
scripts/check-service-layout.sh
```

前端验证：

```bash
cd frontend
npm run test:toolchain
npm test
npm run build
```

## 文档

文档按主题归档在 [`docs/`](docs/README.md)，根目录只保留项目入口、滚动更新日志和仓库协作规则。

- [架构与演进](docs/README.md#架构与演进)
- [多语言服务目录](services/README.md)
- [数据与存储](docs/README.md#数据与存储)
- [部署与运行](docs/README.md#部署与运行)
- [Agent Runtime](docs/README.md#agent-runtime)
- [前端设计](docs/README.md#前端设计)
- [性能记录](docs/README.md#性能记录)
- [更新日志](CHANGELOG.md)

## 开发约定

长期架构约束需要同步实现、测试、文档清单和更新日志。新增服务或数据边界时，优先增加接口与测试，再逐步切换运行拓扑；所有切换都应保留回滚路径。
