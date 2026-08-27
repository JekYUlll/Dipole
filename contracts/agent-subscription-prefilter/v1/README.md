# Agent Subscription Prefilter Eval v1

该合同用于离线比较 `rule`、`embedding` 和 `small_model` 三类 Event Subscription 预筛策略。评测器只读取已记录 evidence，不访问模型、数据库或网络。

- `corpus.schema.json` 保存受控事件正文、人工相关性标签和统一门槛。Corpus 必须同时包含相关与无关 case，case ID 唯一，最多 10000 条。
- `evidence.schema.json` 保存一个候选策略的固定 revision/configuration SHA-256、逐 case 决策、微秒耗时和微美元成本。embedding/小模型必须保存 basis-point 分数与阈值；rule 禁止保存伪分数。
- `report.schema.json` 只保存 corpus/evidence 哈希、候选身份、混淆矩阵、precision/recall、p95 延迟、平均/总成本和误判 case ID，不回显消息正文。

各语言实现除 JSON Schema 结构校验外还必须执行以下规范性语义：corpus case ID 与 evidence decision case ID 分别唯一；corpus 同时包含正、负样本；evidence 与 corpus SHA-256 一致且恰好覆盖每个 case 一次；`rule` 不携带 score/threshold；`embedding|small_model` 的每个 decision 均携带 score，且 `selected == (scoreBps >= decisionThresholdBps)`。违反任一约束均按无效证据处理。

precision/recall 使用向下取整的 basis points，平均成本使用向上取整的整数微美元，p95 使用 nearest-rank。该保守舍入避免跨语言浮点漂移。

Corpus SHA-256 先按 case ID 升序规范化 `cases`，随后递归按 ASCII 对象键字典序排列；数组其余顺序保留，字符串/数字/布尔/null 使用标准 JSON 紧凑编码，最后对 UTF-8 bytes 计算 SHA-256。合同对象键均由 strict schema 固定为 ASCII。示例 corpus 的黄金值为 `e3a0eb42bb6f40c6a81ff233fb468bb996452287d05b9b52b2908941605d3ab2`。Evidence SHA-256 使用同一 canonical JSON 算法，并在计算前按 case ID 排序 decisions。

规则基线直接复用生产 `matchEventSubscriptions`：

```bash
cd agent-runtime
npm run eval:prefilter -- \
  --corpus=../contracts/agent-subscription-prefilter/v1/corpus.example.json \
  --subscription=../contracts/agent-subscription-prefilter/v1/subscription.example.json
```

外部 embedding/小模型 adapter 先生成 evidence，再运行：

```bash
npm run eval:prefilter -- --corpus=../path/corpus.json --evidence=../path/candidate-evidence.json
```

退出码 `0` 表示全部门槛通过，`2` 表示有效证据未达门槛，`1` 表示参数、schema、哈希或逐 case 绑定无效。通过报告只可作为现有五类 Agent Eval 的 retrieval/cost 输入，不能单独启用生产 `subscription` 模式。
