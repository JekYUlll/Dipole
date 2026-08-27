# Agent Artifact 孤儿对象对账

`dipole-agent-artifact-reconcile` 只生成 `dipole.agent.artifact.reconcile.v1` dry-run 报告，用于发现 MinIO 中缺少 MySQL 元数据引用的 Artifact 对象。命令没有删除入口，报告也固定包含 `delete_authorized=false`。

## 准备

先运行 Compose 的 `minio-init`，或等价创建 `configs/minio/agent-artifact-audit-policy.json` 中的只读身份。只向离线命令注入以下配置：

```bash
export DIPOLE_STORAGE_ARTIFACT_ENDPOINT=minio:9000
export DIPOLE_STORAGE_ARTIFACT_BUCKET=dipole-agent-artifacts
export DIPOLE_STORAGE_ARTIFACT_AUDIT_ACCESS_KEY=dipoleartifactaudit
export DIPOLE_STORAGE_ARTIFACT_AUDIT_SECRET_KEY='replace-me'
```

同时设置正常的 `DIPOLE_MYSQL_*` 只读连接配置。audit access key 必须与 Runtime 的 `storage.artifact_access_key` 不同。

## 运行

```bash
/app/dipole-agent-artifact-reconcile \
  -minimum-age=24h \
  -max-examples=100 \
  > artifact-reconcile.json
```

命令只列举 `agent-artifacts/v1/`。对象达到最短年龄后才查询 MySQL；合法内容寻址键且元数据缺失时记为 `orphan_candidate`，异常键记为 `invalid_object_key` 且不具备清理资格。

退出码 `0` 表示没有候选或异常键，`2` 表示需要人工审查，`1` 表示配置、存储或报告生成失败。使用方应调用报告校验逻辑或复算 `evidence_sha256`，并完整保留原始 JSON。

## 安全边界

- audit policy 不包含 Get、Put 或 DeleteObject。
- Runtime/Core、TS Agent 和 Gateway 不持有 audit 凭据。
- 当前命令和报告不授权清理；删除执行器由 AD-032 后续里程碑单独设计。
- 未来执行删除前必须重新查询对象键元数据，防止 dry-run 后新增引用造成竞态。
