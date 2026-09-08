# Dipole 学习与面试主文档

> 这份文档是简历、项目介绍、现场演示和持续学习的入口。技术事实以当前代码、契约、测试和归档证据为准；目标架构与候选能力必须显式标注状态。

## 1. 项目拆分

Dipole 对外讲解时拆成两个相互协作的项目：

| 项目 | 面试定位 | 主文档 |
| --- | --- | --- |
| Dipole IM | Go 实时通信后端、分布式消息链路和多端同步 | [INTERVIEW-IM.md](INTERVIEW-IM.md) |
| Dipole Agent | TypeScript Agent Runtime、可靠任务、能力授权和 MCP | [INTERVIEW-AGENT.md](INTERVIEW-AGENT.md) |

公共实现和技术细节见 [INTERVIEW-TECHNICAL-REFERENCE.md](INTERVIEW-TECHNICAL-REFERENCE.md)。旧问答入口 [INTERVIEW-QA.md](INTERVIEW-QA.md) 保留兼容链接，并逐步收敛到两份分册。

## 2. 简历口径

### Dipole IM

面向多端实时通信与智能协作场景构建的分布式 IM 后端。使用 Go、WebSocket、gRPC、Kafka、Redis、MySQL、sqlc、Cassandra、Elasticsearch 和 MinIO，围绕消息幂等、Transactional Outbox、会话序列、用户同步游标、热点群 notify + pull 和服务渐进拆分建立可测试、可回滚的消息链路。

### Dipole Agent

面向 IM 场景构建的 Agent Runtime。保留 Go/Eino 作为兼容基线，并以 TypeScript/Node.js 承载独立 Runtime；通过可信 ExecutionContext、Capability Registry、资源范围 Policy、模型路由、Memory、MCP、Temporal 和 OpenTelemetry 组织可审计的 Agent Task。高风险能力默认需要人工审批，外部连接和生产写入按独立证据门禁推进。

## 3. 当前事实分层

| 标签 | 含义 | 面试表达 |
| --- | --- | --- |
| `verified` | 代码和测试/运行证据已覆盖 | 可以直接陈述，并给出文件或测试入口 |
| `shadow` | 已有观察或对照路径，不承接默认副作用 | 说明观察目标、退出条件和回滚边界 |
| `candidate` | 候选实现或隔离演练 | 说明它还没有成为默认权威 |
| `default-off` | 代码存在但默认关闭 | 说明启用条件，不把它写成线上默认能力 |
| `planned` | 架构计划或后续工作 | 只能作为演进方向 |

## 4. 90 秒介绍

Dipole 是我持续演进的一套实时协作平台，核心包含两个项目。Dipole IM 用 Go 处理用户、群组、消息、会话和连接接入，通过 Kafka 与 outbox 解耦持久化、会话投影和实时投递，Redis 管理在线状态与热点群策略，MySQL/sqlc 负责事务元数据，Cassandra 和 Elasticsearch 作为独立 Timeline 与搜索投影逐步接管。Dipole Agent 以 Go/Eino 兼容链路为基线，逐步迁移到独立 TypeScript Runtime，Runtime 通过 ExecutionContext、Capability Policy、MCP、Memory 和 Temporal 处理长任务、审批、恢复和审计。整个项目采用先契约、再边界、后独立部署的方式，每个存储或投递切换都保留 shadow、证据和回滚路径。

## 5. 现场讲解顺序

1. 先讲 Dipole IM 的消息事实、异步事件、实时投递和同步游标。
2. 展示一次单聊或群聊链路，解释 `send_requested`、落库、outbox、`created` 和投递的区别。
3. 解释热点群为什么从完整 fan-out 变成 notify + pull，并给出测试/benchmark 位置。
4. 再切到 Dipole Agent，区分 Go/Eino 兼容路径和 TypeScript Runtime。
5. 展示 Agent 的 ExecutionContext、Capability 授权、审批、Task Timeline 和 Memory 证据。
6. 最后讲一个尚未切流的能力，并说明为什么保持默认关闭以及如何回滚。

## 6. 统一回答模板

回答实现类问题时按四句话组织：

1. 先定义组件的职责和数据所有权。
2. 再描述调用或事件顺序。
3. 然后说明失败、幂等、超时和回滚处理。
4. 最后给出代码、契约、测试或 benchmark 证据。

回答取舍类问题时补充：规模假设、替代方案、当前限制和下一步验证。

## 7. 证据入口

- 架构路线：[PLATFORM-EVOLUTION-PLAN.md](../architecture/PLATFORM-EVOLUTION-PLAN.md)
- 服务边界：[SERVICE-BOUNDARIES.md](../architecture/SERVICE-BOUNDARIES.md)
- 消息存储与同步：[MESSAGE-STORAGE-AND-SYNC.md](../architecture/MESSAGE-STORAGE-AND-SYNC.md)
- Agent Runtime：[AGENT-RUNTIME-DESIGN.md](../architecture/AGENT-RUNTIME-DESIGN.md)
- Go/Eino baseline：[contracts/agent-evals/v1/README.md](../../contracts/agent-evals/v1/README.md)
- 性能证据：[benchmarks/](../../benchmarks/)
- 更新日志：[CHANGELOG.md](../../CHANGELOG.md)

## 8. 持续维护规则

- 每次架构切片同时更新本目录相关分册、`CHANGELOG.md` 和架构债务台账。
- 新增简历数字必须绑定 benchmark、测试输出或归档报告；没有证据就使用 `[待测]` 或删去数字。
- API、事件、迁移和配置名称以代码/契约为准，文档中的旧名称必须标记兼容期。
- Go/Eino 与 TypeScript Runtime 的职责变化要同时更新迁移状态，避免把 baseline 误写为主路径。
- 面试材料只记录可公开的低敏信息，不放凭据、真实用户标识、消息正文或内部地址。
