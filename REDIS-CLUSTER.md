# Redis Sentinel 开发与故障验收

本文档描述 A2 阶段的 Redis 7.4 Sentinel 基线。现有 Compose 继续提供单节点快速开发；`docker-compose.redis-cluster.yml` 提供隔离的故障转移演练环境。

## Topology

```text
Dipole go-redis Failover Client
              │
     sentinel-1/2/3 (quorum=2)
              │
              ▼
       dipole-master alias
              │
       redis-1/2/3
      one writer + two replicas
```

应用继续共享一个 `*redis.Client`，业务组件无需感知当前 writer。连接模式由配置选择：

```yaml
redis:
  mode: sentinel
  password: ""
  db: 0
  sentinel_master_name: dipole-master
  sentinel_addresses:
    - sentinel-1:26379
    - sentinel-2:26379
    - sentinel-3:26379
  sentinel_password: ""
```

`mode` 留空或设为 `single` 时继续读取 `host + port`，现有本地部署无需修改。Sentinel 环境下应用只向当前 master 写入，不从 replica 分担读请求。

## Data Semantics

Redis 只保存可重建的实时状态：

| 能力 | 故障期间行为 | 恢复依据 |
| --- | --- | --- |
| Presence / 连接路由 | 操作失败时降级本节点；心跳继续刷新 | Gateway 活连接和后续心跳 |
| Pub/Sub | 订阅连接由 go-redis 重连；断线窗口消息可能缺失 | Kafka 重投递、Sync Timeline 和客户端增量同步 |
| Hot Group | 计数缺失时回到冷群策略 | 新消息重新积累计数，持久群 checkpoint 保存在 MySQL |
| Rate Limit | Redis 错误时 fail-open | 新 writer 可用后重新计数 |
| 元数据缓存 | miss 或错误时回源 | MySQL 事实数据 |

Sentinel 使用异步复制。演练在切主前执行 `WAIT 2`，确认测试状态已到达两个副本；该命令不提供跨故障的强一致承诺。业务设计不得把消息正文、Inbox、设备 Cursor 或其他持久事实放入 Redis。

Redis Pub/Sub 提供 at-most-once 投递，Sentinel 只负责发现新 master。切换窗口内的在线通知允许缺失，客户端通过持久同步协议恢复已确认消息；该边界记录为 `AD-017`。

## Failure Smoke

执行：

```bash
./scripts/smoke-redis-failover.sh
```

脚本使用独立 Compose project 和 volumes，并验证：

1. 启动一个 master、两个 replica 和三个 Sentinel，quorum 为 2。
2. 通过 go-redis Failover Client 写入确认值，并建立长期 Pub/Sub 订阅。
3. 写入 Presence、Hot Group 和 Rate Limit 状态，等待两个副本确认。
4. 停止 Sentinel 报告的当前 master，等待新 master 产生。
5. 复用同一客户端验证读写、原 Pub/Sub 订阅和四类实时状态。
6. 重启旧 master，并确认 Sentinel 将它重新配置为 replica。

调试时可保留现场：

```bash
KEEP_STACK=1 ./scripts/smoke-redis-failover.sh
```

该 Compose 关闭了 Redis protected mode 且没有配置密码，只能用于隔离开发网络。生产环境必须配置 Redis ACL、Sentinel 认证和私有网络访问控制。

## References

- [Redis Sentinel](https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/)
- [Redis Pub/Sub delivery semantics](https://redis.io/docs/latest/develop/pubsub/)
