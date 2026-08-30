# Multipart 故障矩阵

该矩阵是 A7/AD-060 的开发期验收入口，所有动作使用隔离测试资源；默认生产 Multipart 策略、relay 回退和 GPU 任务均不受影响。

## 验收命令

```bash
DIPOLE_REMOTE_GO_ROOT=/home/admin1/.local/go-1.27.0 \
  scripts/smoke-multipart-fault-matrix.sh
```

Remote GPU 执行时允许与既有 GPU 任务并行。脚本只启动临时 MinIO/Redis 容器，退出时自动清理；失败时保留命令输出用于诊断，不执行业务数据删除。

## 覆盖项

| 场景 | 证据 | 判定 |
| --- | --- | --- |
| Redis metadata/parts TTL | Redis deterministic tests | metadata、parts 和 completion receipt 生命周期符合契约 |
| MinIO/Redis 对账 | real reconciliation smoke | matched、missing Redis、Redis orphan 均可识别 |
| Redis restart | reconciliation restart smoke | metadata 丢失 fail-closed，MinIO incomplete upload 可清理 |
| cleanup race | `NoSuchUpload` unit contract | `already_gone` 计入收敛，其他错误仍失败 |
| 指标发布失败 | atomic textfile unit contract | 原目标保留，临时文件清理 |
| 指标告警 | `check-multipart-alerts.sh` | error、checksum、latency、drift、incomplete、stale 可触发 |
| HTTP Gateway 限流 | Multipart `initiate` contract | `429` 在 Core/MinIO 前返回 |
| Presigned proxy 限流 | Gateway proxy route contract | `429` 在 MinIO 前返回 |
| Presigned proxy 超时 | upstream timeout contract | 上游挂起返回 `502` |

## 回滚边界

矩阵失败时保持 `storage.multipart_mode=relay` 和预签名代理默认关闭；禁止依据单项通过切换生产 authority。每次运行应记录 revision、Go toolchain、容器镜像、GPU 进程计数和清理结果。
