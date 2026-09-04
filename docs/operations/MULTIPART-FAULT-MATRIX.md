# Multipart 故障矩阵

该矩阵是 A7/AD-060 的开发期验收入口，所有动作使用隔离测试资源；默认生产 Multipart 策略、relay 回退和 GPU 任务均不受影响。

## 验收命令

```bash
DIPOLE_REMOTE_GO_ROOT=/home/admin1/.local/go-1.27.0 \
  scripts/smoke-multipart-fault-matrix.sh
```

如果远端 Docker registry 不可用，可以下载并校验官方 release 后使用：

```bash
DIPOLE_PROMTOOL_BIN=/path/to/promtool \
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
| Alertmanager routing | `smoke-multipart-alertmanager-routing.sh` | Prometheus 通过临时合成 Multipart 告警投递至开发期 `discard` receiver |
| HTTP Gateway 限流 | Multipart `initiate` contract | `429` 在 Core/MinIO 前返回 |
| Presigned proxy 限流 | Gateway proxy route contract | `429` 在 MinIO 前返回 |
| Presigned proxy 超时 | upstream timeout contract | 上游挂起返回 `502` |

## 回滚边界

矩阵失败时保持 `storage.multipart_mode=relay` 和预签名代理默认关闭；禁止依据单项通过切换生产 authority。每次运行应记录 revision、Go toolchain、容器镜像、GPU 进程计数和清理结果。

Alertmanager routing smoke 只启动隔离的 Prometheus 与 Alertmanager，复用正式 Multipart 规则文件并额外挂载临时 `vector(1)` 规则验证传输。仓库 receiver 固定为 `discard`，因此该步骤不发送外部通知，也不替代生产 receiver、告警升级或 24 小时预签名切流证据。
