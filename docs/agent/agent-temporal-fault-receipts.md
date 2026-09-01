# Agent Temporal Fault Receipts

`agent-temporal-fault` 为可恢复 Agent Task 演练保留低敏、内容绑定的证据。它记录 Task/Run 标识、状态修订序列、注入故障计数与持久副作用基数；不包含提示词、消息正文、审批参数、模型输出或凭据。

## v1 drill

`worker_replacement_approval_resume` 绑定隔离 Temporal 测试路径：Task 进入 `running -> waiting_approval`，Worker 被替换，审批 Signal 在新 Worker 上恢复，随后收敛为 `running -> completed`。首次终态持久化写入失败一次，重试后只保留一次成功写入。

`worker_replacement_input_resume` 覆盖同样的 Worker 替换窗口下的 Elicitation 输入恢复。无效值和过期 request ID 各被拒绝一次，精确 request ID 的有效输入只恢复一次，并在一次终态写入后完成。

v1 仅在状态序列为 `1:running,2:waiting_approval,3:running,4:completed`，且 admission、审批请求、审批解析和终态持久写入均为一条时返回 `eligible`。四次 Step execution 与两次 terminal attempts 是该注入场景的预期值。任何偏差都输出 `ineligible`，不得作为恢复或零重复副作用结论。

`read_scope_confirmation_resume`、`read_scope_confirmation_declined` 和 `read_scope_confirmation_expired` 覆盖多会话读取范围的 owner 确认。这三条演练不注入故障，而是让生产 read Activity 直接跑在 Workflow 后面，因此 receipt 额外记录 `conversationReads` 与 `unconfirmedConversationReads`：确认路径要求状态序列为 `1:running,2:waiting_input,3:running,4:completed`、两次 Step execution、一次输入恢复且恰好读取一个被确认的会话；拒绝与到期路径要求 `1:running,2:waiting_input,3:cancelled`、对应的 `user_cancelled` 或 `input_expired` cancellation、一次 Step execution 且零次会话读取。任一路径出现未确认读取都直接判定 `ineligible`。

## Generate and verify

```bash
cd services/agent-runtime
receipt_dir="$(mktemp -d)"
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true \
DIPOLE_AGENT_TEMPORAL_FAULT_EVIDENCE_DIR="$receipt_dir" \
npm test -- --run src/temporal/agent-task-workflow.integration.test.ts
for receipt in "$receipt_dir"/*.json; do
  npm run receipt:temporal-fault -- --receipt="$receipt"
done
```

显式 `DIPOLE_AGENT_TEMPORAL_FAULT_EVIDENCE_DIR` 让该集成测试将两份低敏 receipt 写入预先创建的绝对目录；未设置时测试没有文件副作用。输入和输出由 [`observation.schema.json`](../../contracts/agent-temporal-fault/v1/observation.schema.json) 与 [`receipt.schema.json`](../../contracts/agent-temporal-fault/v1/receipt.schema.json) 固定。CLI 可从 observation 创建 receipt，或通过 `--receipt` 独立复核已归档 receipt；它不启动 Worker、不写数据库，也不替代共享环境的 Core restart、lease expiry 或 active authority 演练。

### Remote GPU record: 2026-09-01

候选 `6beab05d` 使用 Node `22.12.0` 执行集成套件 `7/7`，再由 CLI 独立复核
[`worker_replacement_approval_resume`](../../benchmarks/agent-temporal-fault-2026-09-01/worker-replacement-approval-resume.json)
与 [`worker_replacement_input_resume`](../../benchmarks/agent-temporal-fault-2026-09-01/worker-replacement-input-resume.json)。两份 receipt 均为 `eligible`，并固定了状态修订和精确副作用基数。该运行使用内存 Temporal Test Server；Core restart、EventLedger lease、共享 tenant 和 active authority 继续使用各自的演练与证据边界。

候选 `aec1b867` 随后在同一主机以 Node `22.12.0` 执行集成套件 `10/10`，并归档读取范围确认的三份 receipt：
[`read_scope_confirmation_resume`](../../benchmarks/agent-temporal-fault-2026-09-01/read-scope-confirmation-resume.json)、
[`read_scope_confirmation_declined`](../../benchmarks/agent-temporal-fault-2026-09-01/read-scope-confirmation-declined.json)
与 [`read_scope_confirmation_expired`](../../benchmarks/agent-temporal-fault-2026-09-01/read-scope-confirmation-expired.json)，均为 `eligible` 并经 CLI 独立复核。三条演练由生产 read Activity 驱动：确认路径在伪造 request 被拒绝后只读取被确认的会话，拒绝与到期路径的会话读取计数为零。该运行同样只使用内存 Temporal Test Server，不涉及 Kafka、Core、MySQL、共享 tenant 或 active authority，也不包含该路径的 outcome/trajectory/permission 评测。

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

## Reviewed Shadow Eval Window

[`collect-agent-shadow-eval-window.sh`](../../scripts/collect-agent-shadow-eval-window.sh)
collects an already running read-shadow Agent's reviewer-supplied manifests and
generates one low-sensitivity summary window. It does not start Compose, publish
events, create labels, or change any feature flag. The operator must first
review each manifest's outcome, trajectory, persisted authorization scope,
retrieval evidence and cost thresholds.

```bash
COMPOSE_PROJECT_NAME=dipole-agent-shadow-window \
COMPOSE_ENV_FILE=.env \
COMPOSE_OVERLAYS=deploy/microservices/agent-ai-sdk-shadow.yml:deploy/microservices/agent-temporal-read-shadow.yml \
DIPOLE_AGENT_SHADOW_EVAL_MANIFEST_DIR=/secure/reviewed-manifests \
DIPOLE_AGENT_SHADOW_EVAL_WINDOW_DIR=/secure/shadow-window \
scripts/collect-agent-shadow-eval-window.sh
```

The output directory must not exist before collection. It receives one report
per manifest, the exact summary input and the `reviewed_shadow` summary report.
Exit status `0` means every reviewed task passed; `2` preserves a valid window
with at least one failed task; all other statuses fail closed. The generated
window remains an isolated development observation until its task set, reviewer
process and environment are independently approved.

Before reading a manifest, the collector resolves the running `agent` container
and records its OCI `org.opencontainers.image.revision` label. It rejects a
missing revision or a non-clean `io.dipole.source.dirty` label. Therefore the
summary's `runtimeRevision` identifies the evaluated image, rather than the
checkout used to invoke the collector.
