# Agent Memory Prefilter v1

该合同用于离线评估 embedding 或 small model 对 reviewed Memory corpus 的晋级预筛。Evidence 只记录固定候选配置、逐 case 分数/阈值、延迟和成本，评测器不访问模型、数据库、Kafka 或网络。

Corpus 标签来自 `agent-memory-reviewed-corpus/v1` 的 `goldPromotable`。候选必须完整覆盖每个 case，且 `selected` 必须与 `scoreBps >= decisionThresholdBps` 一致。报告只输出哈希、候选摘要、聚合指标和门禁原因，不回显 case、消息正文或 reviewer 身份。

```bash
cd agent-runtime
npm run eval:memory-prefilter -- --corpus=../path/corpus.json --evidence=../path/evidence.json --policy=../path/policy.json
```

退出码 `0` 表示达标，`2` 表示有效证据未达门槛，`1` 表示输入无效。该证据仅用于后续候选比较，不能单独开启生产 Memory 自动写入或 Event Subscription 灰度。
