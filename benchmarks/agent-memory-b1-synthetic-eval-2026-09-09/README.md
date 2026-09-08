# B1 Synthetic Memory Eval

该基准记录 2026-09-09（Asia/Shanghai）在 Remote GPU 隔离 Compose 项目中以真实 DeepSeek V4 Flash Provider 运行的 B1 synthetic Memory Eval。suite 只保存 canary、Task 和回复的 SHA-256，以及完成状态、模型调用计数、Memory lineage 计数和 canary 命中布尔值。

同一个 owner-reviewed synthetic canary 形成两个案例：`recall-b1` 要求一条 pre-model lineage 和回复命中；`revoke-b1` 要求 owner revoke 后的新 Task 没有该 lineage。后者的 `responseContainsCanary` 仅作观察，不参与否定断言，因为同一直接会话的短期历史可能包含此前的回复。Core lineage 是撤销读取边界的权威证据。

复跑命令：

```bash
cd services/agent-runtime
npm run eval:memory-b1-synthetic -- --suite=../../benchmarks/agent-memory-b1-synthetic-eval-2026-09-09/suite.json --report=../../benchmarks/agent-memory-b1-synthetic-eval-2026-09-09/report.json
```

本次结果应为 2/2 通过、`10000` bps。该样本仅证明合成 canary 的端到端召回和撤销边界，不能外推为真实用户语义成功率、长期 Memory 默认启用或多模型质量。
