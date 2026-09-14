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
| AD-005 | 历史 benchmark 提交包含 JWT | 当前产物和导出已修复，Git 历史仍保留旧内容 | 按面试审核 IR-05 评估历史暴露和凭据失效，未经授权不重写历史 |
| AD-006 | 体验栈运行代码与重建镜像需对齐 | Agent 完整镜像已构建且禁网模块加载通过；现有体验容器尚未切换到该镜像 | 用新镜像重建 Agent 后运行现有真实 smoke；干净环境启动单独验收 |
| AD-007 | 定时总结交互待完善 | 群定时发布已完成真实 DeepSeek/Temporal/Core/MySQL/WS/Sync 验证；自然语言时间澄清未实现，等待期间仍显示审批状态 | 改善时间澄清和定时等待展示 |

## 已验证的产品边界

- 构建统一由 Makefile 管理，日常操作使用 justfile；旧多程序 Dockerfile 仅保留给仍有引用的 benchmark/legacy 配置，主路径使用单服务镜像。

- IM 主路径使用消息事实、Transactional Outbox、Kafka、Conversation Timeline、
  Sync Timeline、Device Cursor、Redis Presence 与 Elasticsearch Search。
- Agent 主路径使用可信 ExecutionContext、Capability、Temporal、Approval 与
  Core-owned idempotent Message Command。
- AD-008 已修复：迁移 `000055` 将消费过的审批唯一绑定到调用；重试复用原调用并返回持久化消息回执。真实 smoke 在消息提交后阻断审计并强制停止 Worker，原 Task 经 Activity 第 2 次执行完成，消息与调用各一条。已终止的历史失败任务不自动恢复，外部 MCP 实验路径不在此验收范围。
- 天气工具使用公开 Open-Meteo 端点和显式 weather.read 权限；真实 HTTP 查询已验证，聊天使用需部署工具与迁移 000053。
- Cassandra、C++ Realtime Delivery 和外部 MCP 不属于默认产品运行路径。
