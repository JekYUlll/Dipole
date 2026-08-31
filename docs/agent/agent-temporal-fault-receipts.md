# Agent Temporal Fault Receipts

`agent-temporal-fault` 为可恢复 Agent Task 演练保留低敏、内容绑定的证据。它记录 Task/Run 标识、状态修订序列、注入故障计数与持久副作用基数；不包含提示词、消息正文、审批参数、模型输出或凭据。

## v1 drill

`worker_replacement_approval_resume` 绑定隔离 Temporal 测试路径：Task 进入 `running -> waiting_approval`，Worker 被替换，审批 Signal 在新 Worker 上恢复，随后收敛为 `running -> completed`。首次终态持久化写入失败一次，重试后只保留一次成功写入。

`worker_replacement_input_resume` 覆盖同样的 Worker 替换窗口下的 Elicitation 输入恢复。无效值和过期 request ID 各被拒绝一次，精确 request ID 的有效输入只恢复一次，并在一次终态写入后完成。

v1 仅在状态序列为 `1:running,2:waiting_approval,3:running,4:completed`，且 admission、审批请求、审批解析和终态持久写入均为一条时返回 `eligible`。四次 Step execution 与两次 terminal attempts 是该注入场景的预期值。任何偏差都输出 `ineligible`，不得作为恢复或零重复副作用结论。

## Generate and verify

```bash
cd services/agent-runtime
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm run test:temporal:integration
npm run receipt:temporal-fault -- --observation=temporal-fault-observation.json
```

输入和输出由 [`observation.schema.json`](../../contracts/agent-temporal-fault/v1/observation.schema.json) 与 [`receipt.schema.json`](../../contracts/agent-temporal-fault/v1/receipt.schema.json) 固定。CLI 只验证和绑定已归档观察数据；它不启动 Worker、不写数据库，也不替代共享环境的 Core restart、lease expiry、input resume 或 active authority 演练。

## Core restart read-shadow evidence

`core-restart-read-shadow-evidence.schema.json` 记录独立 Compose read-shadow 演练中的 Core 重启窗口。只有在 Core 重启后通过 readiness、Gateway 代理恢复，并且同一事件的 completed EventLedger、Task、Run、模型调用和 `conversation_digest` Artifact 均精确为一条时，smoke 脚本才会输出证据。

```bash
COMPOSE_PROJECT_NAME=dipole-read-shadow-restart \
DIPOLE_GATEWAY_PORT=28084 \
GATEWAY_URL=http://127.0.0.1:28084 \
COMPOSE_ENV_FILE=.env \
COMPOSE_OVERLAYS=deploy/microservices/agent-ai-sdk-shadow.yml:deploy/microservices/agent-temporal-read-shadow.yml \
EXPECT_READ_SHADOW=1 \
RESTART_CORE_AFTER_EVENT=1 \
DIPOLE_AGENT_CORE_RESTART_EVIDENCE=services/agent-runtime/.artifacts/core-restart-read-shadow.json \
scripts/smoke-microservices.sh
```

artifact 固定为 24 小时有效、SHA-256 绑定且 `production_authority=false` 的低敏开发期证据。它证明受控只读 Shadow 中的一次 Core restart 收敛；Worker replacement、lease expiry、写 Capability、共享 tenant 与 active authority 仍需独立演练。

### Remote GPU record: 2026-08-31

Remote GPU 在候选 revision `a7bc03ef2f4b17782ddc97b148aaf0a969bd4f35` 的隔离
Compose 项目 `dipole-agent-core-restart-f1a0cc2f` 完成一次受控演练。Core 在事件
发布后重启，Gateway 代理恢复；同一 event 的 completed EventLedger、Task、Run、模型
调用和 `conversation_digest` Artifact 计数均为 `1`。运行后用新构建的
`dipole-agent:latest` 镜像内 `core-restart-read-shadow-evidence-cli.js` 独立复核通过。

回执的 `collected_at` 为 `2026-08-31T15:37:21.810Z`，canonical
`content_sha256` 为 `09796dcedde2f95b7197183e9c584dd8d8c470ce67db9e9eccd8fa936b52b727`。
raw artifact 的文件 SHA-256 为
`482142d3754cdefc0452f746f50efd51edd8d7d3020fdf67062da8de2c336f16`，并遵循其
24 小时有效期，因此不纳入版本库。该记录只证明该 revision 的 disposable Compose
read-shadow 恢复路径；不构成共享环境、active authority、写 Capability、lease expiry
或 Worker replacement 联合故障的证据。

## EventLedger lease reclaim evidence

`event-lease-reclaim-evidence.schema.json` 记录 Agent Kafka 消费侧 MySQL
`agent_event_ledger` 的租约恢复。它要求 expired claim 被一次新 claim 回收、原 claim
token 的完成请求被拒绝、同一 event 的最终 completed 行精确为一条，且 receipt 固定为
24 小时、SHA-256 绑定、`production_authority=false`。该 lease 仅保护 EventLedger
消费 ownership；Workflow 的 Worker replacement、approval/input resume 仍使用上方
Temporal fault receipt，二者不得互相替代。

Remote GPU 在候选 `5e8a213e` 的 loopback-only `mysql:8.4` 临时容器中完成真实
集成测试 `3/3`，并通过 receipt 单测 `4/4` 与 Runtime typecheck。测试结束后容器已
移除。该结果不表示共享 Kafka/Temporal、跨进程业务副作用或 active authority 已验收。

### Remote GPU Temporal regression record: 2026-09-01

候选 `cc5806fb` 使用 Node `22.12.0` 完成
`DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm run test:temporal:integration`：两个
测试文件、十个用例均通过。运行覆盖 approval/input resume、Worker replacement、终态
重试和模型/Step post-effect 重放；后者同时验证 Model call、Capability 和 Step
authorization audit 不因 Activity acknowledgement 丢失而重复。该记录来自内存
Temporal Test Server，不包含 Core restart、Kafka EventLedger lease、共享环境或 active
authority 结论。
