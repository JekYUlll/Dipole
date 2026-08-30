# C1 Remote GPU Baseline

本目录归档 Remote GPU 上 `master` 提交 `160d2cc620ac62de33b099a1690a2d84e6a8bb18` 的低风险 C1 direct message 基线。

## Scope

- 拓扑：`deploy/compose/docker-compose.dist.yml`，三节点 `dipole-server`，独立 Compose project `dipole-c1`
- 负载：20 用户、10 个直聊发送者、每个发送者 5 条消息、最大 20 VU
- 工具：远端 `k6 v1.3.0`
- 结果：50/50 接受、持久化和投递；HTTP 失败率 `0%`
- 消息端到端延迟：平均 `70.04ms`，P50 `49ms`，P95 `162.10ms`，P99 `165ms`
- Kafka：峰值和稳定采样 lag 均为 `0`

补充群广播场景：20 个成员、10 条群消息，`190/190` 预期回执收到，投递率 `100%`；消息端到端延迟 P50/P95/P99 为 `83/89.54/107ms`，Kafka lag 采样为 `0`。

补充并发在线场景：20 个在线用户、每用户 4 条消息，`80/80` 消息接受、持久化和投递，投递率 `100%`；消息端到端延迟 P50/P95/P99 为 `91.5/103.05/104.41ms`，Kafka lag 采样为 `0`。

容量观察场景：100 个在线用户、每用户 4 条消息，`400/400` 消息接受、持久化和投递，投递率 `100%`；消息端到端延迟 P50/P95/P99 为 `149/178.04/243.01ms`，Kafka lag 采样为 `0`。相比 20 用户并发，延迟上升，后续需要继续测试更高并发和故障场景。

节点恢复场景：对 `dipole-node2` 执行 stop/start；约 `505ms` 观察到不可用，约 `16.0s` 恢复健康，consumer group 恢复并稳定为 `72` 个成员。恢复后 40/40 消息接受、持久化和投递，Kafka lag 为 `0`，PID 已更换且镜像 revision 保持一致。

组件故障场景：独立三 broker Kafka consumer rebalance 通过，单 member 退出后 6 个 partition 完成接管且 lag 为 `0`；独立 Redis Sentinel 三节点 failover 通过，主节点停止后约 4 秒完成切换，客户端读写、Pub/Sub、Presence、热群和限流状态恢复，旧 master 重新加入为 replica。详细边界见 `c1-component-fault-evidence.md`。

## Limits

该结果用于确认候选拓扑和消息链路在低负载下可运行，不代表最大吞吐、长连接容量、热群 fan-out 或故障恢复能力。后续 C1 基线应在相同提交绑定、资源隔离和指标采样条件下扩大矩阵。

## Files

- `c1-remote-20.baseline.md`：可读报告
- `c1-remote-20.baseline.json`：原始 k6 汇总数据
- `c1-remote-20.log`：运行日志
- `c1-remote-group20.baseline.md`：群广播可读报告
- `c1-remote-group20.baseline.json`：群广播原始汇总数据
- `c1-remote-group20.log`：群广播运行日志
- `c1-remote-concurrent20.baseline.md`：并发在线可读报告
- `c1-remote-concurrent20.baseline.json`：并发在线原始汇总数据
- `c1-remote-concurrent20.log`：并发在线运行日志
- `c1-remote-concurrent100.baseline.md`：100 用户容量观察报告
- `c1-remote-concurrent100.baseline.json`：100 用户原始汇总数据
- `c1-remote-concurrent100.log`：100 用户运行日志
- `c1-node2-recovery-dd46e35b.recovery-evidence.json`：节点故障原始证据
- `c1-node2-recovery-dd46e35b.recovery-report.json`：节点恢复校验报告
- `c1-node2-recovery-dd46e35b-post.baseline.json`：恢复后消息基线
- `c1-node2-recovery-dd46e35b-post.baseline.md`：恢复后可读基线
- `c1-node2-recovery-dd46e35b-post.log`：恢复后运行日志
- `c1-component-fault-evidence.md`：Kafka/Redis 组件故障演练证据及边界
