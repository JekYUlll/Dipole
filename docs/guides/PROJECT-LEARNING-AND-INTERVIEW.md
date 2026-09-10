# Dipole 项目介绍与讲解指南

## 项目概述

Dipole 是一个面向实时协作的即时通信与 Agent 平台。项目由两部分组成：

| 项目 | 定位 | 核心问题 |
| --- | --- | --- |
| Dipole IM | Go 实时通信后端 | 如何可靠地发送、投递、同步和检索消息 |
| Dipole Agent | TypeScript Durable Agent Runtime | 如何让 Agent 在受控权限下完成长任务并从中断恢复 |

两者通过 Kafka 事件和 Core Capability RPC 协作：IM 负责消息、会话和权限事实；
Agent 负责任务编排、上下文、工具调用与审批等待。

## 完整产品链路

```text
Client
  | HTTP / WebSocket
  v
Gateway
  | authentication, connection, realtime route
  v
Core / Message -------------------> MySQL
  |                                  | message + outbox transaction
  v                                  v
Kafka --------------------------> Sync / Search / Realtime Delivery / Agent
  |                                  |                |
  |                                  v                v
  |                           user inbox         Redis presence
  |                           device cursor
  v
Agent Runtime -> Temporal -> Core Capability -> Message Command
```

### 消息发送与实时投递

客户端通过 HTTP 或 WebSocket 发送消息。Gateway 完成认证、限流和连接路由；
Message Service 用 Client Message ID 保证重试幂等，并在一个数据库事务内写入消息
事实和 Outbox。Outbox Relay 将 `message.created` 发送到 Kafka，后续的会话更新、
同步投影、搜索索引、在线投递和 Agent 触发都从该事实事件派生。

Redis 保存在线节点与连接状态。普通群聊采用接收者投递；热点群使用轻量通知和
按序补拉，减少单条消息的写扩散与连接扇出。

### 历史、同步与搜索

消息历史按会话内单调 `seq` 读取。同步路径维护用户 Inbox Timeline、Read Seq 和
Device Cursor：历史顺序、已读位置和设备同步位置分别表达，客户端可以稳定地补拉
离线消息并在多端恢复。

搜索由 Kafka 驱动异步索引。Agent 与普通客户端都必须通过 Core 提供的权限边界
查询会话，搜索结果只返回当前主体有权读取的消息。

### Durable Agent

私聊 AI、群聊 `@AI` 或显式任务会创建稳定的 Agent Task。Runtime 从可信服务端
状态构建 ExecutionContext，再编译当前消息、会话窗口与有界检索证据。模型只能选择
已经注册的 Capability：

- `conversation.list`、`conversation.read`、`conversation.search` 用于只读会话检索。
- 写操作先进入 `WAITING_APPROVAL`，由用户批准或拒绝。
- 批准后的写入通过 Core 再次校验权限与资源范围，最终以幂等 Message Command 写回 IM。

Temporal 持久化 Workflow 状态、Activity 重试和 Approval 等待点。Worker 重启后，
同一 Task 会从历史继续执行；稳定的 invocation ID 使已经提交的写入不会重复产生副作用。

## 演示脚本

1. 登录两个账户，建立单聊或群聊，发送一条普通消息并观察实时到达。
2. 断开一个客户端，继续发送消息；重新连接后用同步游标补拉。
3. 在群中发送 `@AI 总结刚才讨论的内容`，展示同一群内的 Agent 回复与 Task 状态。
4. 私聊 AI：`帮我找之前关于 Cassandra 的讨论并总结结论`，展示受权限约束的会话搜索与回答。
5. 私聊 AI：`/system 提醒我明天检查发布`，展示任务进入 Approval；拒绝后没有消息写入，批准后只写入一条消息。
6. 在 Approval 等待期间重启 Agent Worker，再批准，展示同一 Task 恢复并完成。

## 简历描述

### Dipole IM

面向多端实时协作构建 Go 即时通信后端，基于 WebSocket、Kafka、Redis、MySQL/sqlc、
Elasticsearch 与 MinIO 实现可靠消息发送、实时投递、双 Timeline 多端同步、热点群
`notify + pull`、权限感知搜索与分片文件上传。

### Dipole Agent

基于 TypeScript、Temporal、gRPC 与 MCP 构建 IM-native Agent Runtime；通过可信
ExecutionContext、Capability 授权、上下文编译、Human-in-the-loop 和幂等 Message
Command 支持会话检索、受控写操作与 Worker 重启后的 Durable Task 恢复。

## 面试入口

- [Dipole IM 问答](INTERVIEW-IM.md)
- [Dipole Agent 问答](INTERVIEW-AGENT.md)
- [技术参考](INTERVIEW-TECHNICAL-REFERENCE.md)
- [架构设计](../architecture/AGENT-RUNTIME-DESIGN.md)
