# Agent Memory Observation/Reflection

## 目标

`ObservationWorker` 和 `ReflectionWorker` 为 Agent Memory 提供一个框架中立、默认只读的压缩边界：

```text
message event
    -> ObservationWorker
    -> observational candidate
    -> reviewed/persisted sink
    -> ReflectionWorker
    -> reflection candidate
```

本切片只生成候选，不自动写入 `agent_memories`，也不调用模型、Kafka、数据库或外部工具。后续接入时必须通过显式 sink、owner/policy 校验和独立 rollout 开关。

## 不变量

- Observation 以 `eventId` 幂等；同一 worker 重复收到事件不会生成第二个候选。
- Candidate ID 由租户、主体、资源、事件/窗口和证据 ID 的规范字节计算，重放不会产生漂移 ID。
- 候选固定为 `observational` 和 `untrusted` 语义，provenance 只保存消息或窗口引用，不复制凭据。
- 输入超限、凭据模式和无法识别的观察内容 fail closed；上游坏数据不会中断消费循环。
- Reflection 需要最少两个唯一观察，拒绝跨租户、跨 Agent、跨资源和重复证据。
- Reflection 窗口只能成功一次；重复窗口返回空结果，避免重复长期记忆。

## 后续接线顺序

1. 以受审阅 corpus 校准规则、小模型或 embedding 预筛选，并补充 retrieval/semantic Eval。
2. 接入持久 candidate ledger，保存 candidate hash、evidence IDs、policy version 和 reviewer decision。
3. 在 Temporal durable task 中运行 Reflection，完成取消、重试和输出 receipt 后再允许显式 Memory sink。
4. 通过 owner scope、TTL、correction/supersession 和派生数据 retention 门禁后，按 tenant shadow 灰度。
