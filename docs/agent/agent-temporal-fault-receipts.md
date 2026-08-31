# Agent Temporal Fault Receipts

`agent-temporal-fault` 为可恢复 Agent Task 演练保留低敏、内容绑定的证据。它记录 Task/Run 标识、状态修订序列、注入故障计数与持久副作用基数；不包含提示词、消息正文、审批参数、模型输出或凭据。

## v1 drill

`worker_replacement_approval_resume` 绑定隔离 Temporal 测试路径：Task 进入 `running -> waiting_approval`，Worker 被替换，审批 Signal 在新 Worker 上恢复，随后收敛为 `running -> completed`。首次终态持久化写入失败一次，重试后只保留一次成功写入。

v1 仅在状态序列为 `1:running,2:waiting_approval,3:running,4:completed`，且 admission、审批请求、审批解析和终态持久写入均为一条时返回 `eligible`。四次 Step execution 与两次 terminal attempts 是该注入场景的预期值。任何偏差都输出 `ineligible`，不得作为恢复或零重复副作用结论。

## Generate and verify

```bash
cd services/agent-runtime
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm run test:temporal:integration
npm run receipt:temporal-fault -- --observation=temporal-fault-observation.json
```

输入和输出由 [`observation.schema.json`](../../contracts/agent-temporal-fault/v1/observation.schema.json) 与 [`receipt.schema.json`](../../contracts/agent-temporal-fault/v1/receipt.schema.json) 固定。CLI 只验证和绑定已归档观察数据；它不启动 Worker、不写数据库，也不替代共享环境的 Core restart、lease expiry、input resume 或 active authority 演练。
