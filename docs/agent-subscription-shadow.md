# Agent Subscription Shadow 观察

该模式在 `direct_target` 主路径进入 EventLedger 前，对同一消息调用 Core 的确定性 Subscription matcher，并只记录对照指标。它不会创建 Subscription binding、Task、Run、Temporal Workflow 或模型调用。

## 启用前提

- 保持 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`。
- Kafka Runtime 与 Agent Capability RPC 必须启用并通过现有 mTLS/readiness 检查。
- 显式设置 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED=true`；Compose 默认固定为 `false`。
- 先确认 Prometheus 的 `dipole-agent` target 正常，并能读取 `dipole_agent_subscription_shadow_enabled`。

## 观察指标

- `dipole_agent_subscription_shadow_comparisons_total{direct_target,subscription}`：固定 `accepted|ignored` 与 `match|miss|error` 六种组合。
- `dipole_agent_subscription_shadow_candidates_total`：匹配候选总数，不含 Subscription、Definition、principal 或 event 标识。
- matcher 错误只累计 `subscription="error"`，不阻断 direct-target 主路径；EventLedger 与下游调用次数不因此增加。

至少观察完整业务周期，并联查 `DipoleAgentSubscriptionShadowErrors` 与 `DipoleAgentSubscriptionShadowAdmissionDrift`。对照数据只能用于形成后续晋级证据；真实 reviewed corpus、precision/recall、成本和漏触发门槛仍由 AD-034 管理。

## 回滚

1. 设置 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED=false` 并滚动 Agent Runtime。
2. 确认 `dipole_agent_subscription_shadow_enabled 0`，Core matcher 调用停止。
3. 保持 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`；无需数据库迁移或 EventLedger 清理。

若 matcher error 告警触发，优先执行上述回滚。不得通过切换到 `subscription` 主模式规避告警。

## 证据合同

`contracts/agent-subscription-shadow/v1/input.schema.json` 定义 Prometheus 起止快照输入，必须绑定：

- UTC `window_start/window_end`，跨度 24 小时至 31 天；
- 当前 tracked tree 或镜像内容的 `runtime_revision` SHA-256；
- `deploy/observability/prometheus-services.yml` 与规则查询集合的 `prometheus_config_sha256`；
- 固定 `query_revision=subscription-shadow-v1`；
- 按抓取间隔计算的 `expected_scrapes`、Prometheus 实际成功样本 `successful_scrapes`；
- 窗口内对两个 counter family 执行 `resets(...[window])` 后汇总的 `counter_resets`；
- 六个 comparison series 和 candidate counter 的起止累计值。

生成证据：

```bash
cd agent-runtime
npm run eval:subscription-shadow-evidence -- --input=/secure/subscription-shadow-input.json
```

将标准输出保存为 owner-only 文件后可独立验证：

```bash
npm run eval:subscription-shadow-evidence -- --evidence=/secure/subscription-shadow-evidence.json
```

生成器要求抓取覆盖率至少 95%、观察事件至少 100、counter 全部单调、`resets=0` 且 matcher error 为零。证据最多有效 24 小时，canonical SHA-256 覆盖完整正文，并固定 `production_authority=false`、`runtime_change_authority=false`。该文件没有 Runtime admission、Trigger mode 或 Capability 授权用途。
