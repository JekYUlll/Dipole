# 架构债务台账

本台账只保留当前会影响产品质量、演示或维护成本的事项。已解决和历史演进记录见
[归档](../archive/ARCHITECTURE-DEBT-2026-09.md)。

## 当前事项

| ID | 事项 | 影响 | 下一步 |
| --- | --- | --- | --- |
| AD-001 | Elasticsearch 是异步投影 | 新消息在索引追赶前可能暂时不可搜索 | 通过索引延迟指标和重建任务持续校验 |
| AD-002 | 热点群阈值需要以真实负载校准 | 过早切换会增加客户端补拉，过晚切换会放大 Fan-out | 以压测结果确定阈值和队列水位 |
| AD-003 | Cassandra Timeline 保留为可选存储实验 | 默认 MySQL 路径不受影响，实验代码增加维护面 | 只在有性能证据时评估是否继续维护 |
| AD-004 | 外部 MCP 集成保留为实验代码 | 产品 Runtime 不依赖它，源码仍有额外维护成本 | 引入真实外部工具需求前再决定保留或删除 |

## 已验证的产品边界

- IM 主路径使用消息事实、Transactional Outbox、Kafka、Conversation Timeline、
  Sync Timeline、Device Cursor、Redis Presence 与 Elasticsearch Search。
- Agent 主路径使用可信 ExecutionContext、Capability、Temporal、Approval 与
  Core-owned idempotent Message Command。
- Cassandra、C++ Realtime Delivery 和外部 MCP 不属于默认产品运行路径。
