# Agent Subscription Shadow 观察

该模式在 `direct_target` 主路径进入 EventLedger 前，对同一消息调用 Core 的确定性 Subscription matcher，并只记录对照指标。它不会创建 Subscription binding、Task、Run、Temporal Workflow 或模型调用。

## Enforced Runtime Binding

当后续在受控环境启用 `subscription` trigger mode 时，每个确定性命中的 Subscription 都独立派发。Runtime 将 Subscription 的 `created_by_id` 绑定为执行 principal，Core 会再次要求该 principal 同时拥有 pinned Definition 和 Subscription。Task ID 与 EventLedger key 都包含 Subscription identity，因此同一事件可服务多个 owner，同时每条 Subscription 的重复消费继续收敛为一次 Task/Run 副作用。

Definition 选择同样以 owner 为边界：Core 按 `(tenant, created_by_id, agent_id)` 查询最新版本，数据库唯一键为 `(tenant, owner, agent, version)`。同一 Agent 的两位用户可以持有相同版本号而不发生覆盖；嵌入式 direct-target Definition 继续由 Agent 自身作为 owner 管理。

该契约仅覆盖 Runtime 与 Core 的一致性门禁。默认 `direct_target` 模式、默认关闭的 subscription trigger、rollout gate 和 reviewed 观察窗口均保持不变。

观察窗口完成后，受控环境可采用 [Subscription Active Read](AGENT-ACTIVE-DEPLOYMENT.md#subscription-active-read) overlay。它使用专用 consumer group 与 Temporal queue 将 owner-scoped Subscription 固化为独立 durable Task，保持只读 Capability，且不复用 Shadow 的 direct-target 主路径。

## 启用前提

- 保持 `DIPOLE_AGENT_TRIGGER_MODE=direct_target`。
- Kafka Runtime 与 Agent Capability RPC 必须启用并通过现有 mTLS/readiness 检查。
- 显式设置 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED=true`；Compose 默认固定为 `false`。
- 先确认 Prometheus 的 `dipole-agent` target 正常，并能读取 `dipole_agent_subscription_shadow_enabled`。

## 受控 Compose Profile

`deploy/microservices/agent-subscription-shadow.yml` 提供开发期的显式观察
profile。它保持 `shadow + direct_target`，开启 matcher 对照指标，并只暴露
owner-scoped Definition/Subscription 管理 API；Task Control、Memory、MCP 和
External MCP 均固定关闭。

```bash
docker compose \
  -f deploy/compose/docker-compose.microservices.yml \
  -f deploy/microservices/agent-subscription-shadow.yml up -d
```

移除该 overlay 后，基础 Compose 会恢复 `DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED=false`
与两项 Gateway 管理入口关闭的默认状态。

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

## 自动采集

`contracts/agent-subscription-shadow/v1/collection.schema.json` 定义只读采集请求。Collector 固定执行 19 次历史 instant query，要求窗口内只有一个 `dipole-agent` series、Shadow 全程启用、所有结果均为非负安全整数，并自动计算抓取覆盖、counter reset 与起止快照。Prometheus URL 仅允许无凭据、无 path/query/fragment 的 HTTP(S) origin。

```json
{
  "schema_version": "dipole.agent.subscription-shadow-collection.v1",
  "prometheus_url": "http://prometheus:9090",
  "window_start": "2026-08-28T00:00:00.000Z",
  "window_end": "2026-08-29T00:00:00.000Z",
  "runtime_revision": "<64-char-sha256>",
  "prometheus_config_sha256": "<64-char-sha256>",
  "scrape_interval_seconds": 5
}
```

采集并保存 v1 input：

```bash
cd services/agent-runtime
npm run eval:subscription-shadow-collect -- --request=/secure/subscription-shadow-collection.json \
  > /secure/subscription-shadow-input.json
```

Collector 不修改 Runtime、Prometheus 或 Trigger mode，也不在输出中保留 Prometheus URL。`runtime_revision` 与 `prometheus_config_sha256` 必须由发布系统根据实际部署 artifact/config 提供；当前指标面无法独立证明远端 revision，审核时仍需核对部署记录。

生成证据：

```bash
cd services/agent-runtime
npm run eval:subscription-shadow-evidence -- --input=/secure/subscription-shadow-input.json
```

将标准输出保存为 owner-only 文件后可独立验证：

```bash
npm run eval:subscription-shadow-evidence -- --evidence=/secure/subscription-shadow-evidence.json
```

生成器要求抓取覆盖率至少 95%、观察事件至少 100、counter 全部单调、`resets=0` 且 matcher error 为零。证据最多有效 24 小时，canonical SHA-256 覆盖完整正文，并固定 `production_authority=false`、`runtime_change_authority=false`。该文件没有 Runtime admission、Trigger mode 或 Capability 授权用途。
