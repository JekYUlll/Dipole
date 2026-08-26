# Web Sync 灰度与观测手册

本文档用于将 Web 客户端从旧 `/messages/offline` 增量链路渐进迁移到 `/sync`。所有阶段都保留可回切构建，真实流量观测结果需要单独归档，规则测试通过仅证明门禁表达式有效。

## 1. 适用范围

首批对照范围固定为 `incoming_direct`，只覆盖当前用户收到的私聊消息：

- 旧 Offline 与 Sync 都能表达这一语义，可按 Message UUID 精确比较。
- 当前用户发送的私聊消息只进入 Sync sender copy，不进入旧 Offline。
- 普通群使用用户 Inbox fanout，热群使用 notify + pull，群消息需要独立契约后再纳入。

## 2. 部署前提

1. Sync Projector 已追平，consumer lag 为零，retry 和 dead-letter 在观察窗口内没有增量。
2. Core Runtime 启用 metrics，并由生产 Prometheus 抓取每个实例的 `/metrics`；只加载规则但没有抓取 Core 指标无法形成观测结论。
3. Prometheus 加载 `deploy/observability/web-sync-alerts.yml`，执行 `scripts/check-web-sync-alerts.sh` 验证规则和固定时序测试。
4. 候选 Web 构建使用 `VITE_SYNC_ENGINE_MODE=shadow`，旧 Offline 继续驱动界面，Sync 只写 IndexedDB、ACK Cursor 并上报聚合计数。
5. 每次切换构建版本重新开始完整观察窗口，并记录版本、开始时间、结束时间和 Prometheus 快照。

## 3. 门禁语义

Prometheus 记录以下 24 小时滚动值：

```promql
dipole:web_sync_shadow:matches_24h
dipole:web_sync_shadow:terminal_differences_24h
dipole:web_sync_shadow:overflows_24h
dipole:web_sync_shadow:window_complete
dipole:web_sync_shadow:promotion_ready
```

`promotion_ready` 只有在同一滚动窗口满足以下条件时为 `1`：

- comparison 指标在 24 小时前已经存在；
- `match >= 100`；
- grace 到期后的 `legacy_only + sync_only == 0`；
- 有界比较器 `overflow == 0`。

`pending` 是客户端宽限期内的采样累计值，不作为终态正确性门禁。单边结果会在 60 秒 grace 到期后归入 `legacy_only` 或 `sync_only`；晋级前应保持 shadow 运行并确认两条 critical 告警均未触发。

## 4. 停止条件

以下任一条件出现时暂停晋级，保持或回切 `off`，先定位协议、Projector、设备状态或客户端容量问题：

- `DipoleWebSyncShadowDivergence` firing；
- `DipoleWebSyncShadowOverflow` firing；
- Sync Projector lag、retry 或 dead-letter 告警；
- 24 小时后 `promotion_ready != 1`；
- `/sync`、IndexedDB commit 或 Cursor ACK 的客户端错误持续增长；
- 观察期间变更了候选 Web bundle 或服务端同步语义。

禁止通过清空 Prometheus 数据、缩短窗口或提高误差阈值绕过停止条件。差异排除后发布新候选版本，重新执行完整窗口。

## 5. 晋级步骤

1. 归档候选版本在完整 24 小时窗口内的五项 recording rule 快照和两条告警状态。
2. 确认至少 100 个收到私聊的匹配样本，终态差异与溢出均为零。
3. 先对受控节点构建 `VITE_SYNC_ENGINE_MODE=primary`；`/sync` 驱动界面，旧 Offline 继续作为观测路径。
4. 验证登录恢复、断网重连、多页追平、多设备 Cursor、显式退出和账号切换。
5. 验证高低水位淘汰后本地安全 Cursor 保持完整，配额失败显示 `storage_full` 且不 ACK 未持久化页面。
6. 验证热群补拉页面先写入 IndexedDB v3，再 ACK 对应设备群 checkpoint；刷新后不得重复请求已经持久化的 Seq 范围。
7. 保留 `shadow` 或 `off` bundle 以及服务端旧接口，在 AD-025 的真实浏览器、共享设备和进程强退验收完成前不扩大默认范围。

## 6. 回切

客户端回切只改变构建变量，不迁移或删除服务端数据：

```text
VITE_SYNC_ENGINE_MODE=primary -> shadow -> off
```

`shadow` 回切后旧 Offline 重新驱动界面，并继续收集对照证据；紧急情况下直接回到 `off`。IndexedDB 中已提交的 Sync 数据按用户隔离保留；显式退出、被动 401、WS kick 和账号切换统一清理当前账号。清理失败、浏览器进程强退与真实配额行为继续由 AD-025 的外部验收跟踪。

## 7. 验证命令

```bash
./scripts/check-web-sync-alerts.sh
docker compose -f docker-compose.cluster.yml --profile observability config
cd frontend
npm ci
npm run test:e2e:install
npm run test:e2e
```

Playwright 运行 Chromium、Firefox、WebKit 的生产 IndexedDB 实现，覆盖容量淘汰、重开、账号隔离、延迟清理和页面重载中断事务。Linux WebKit 需要先执行 Playwright 官方 `install-deps webkit`，CI 可使用与 `@playwright/test` 版本一致的官方镜像。

Chromium CDP `Storage.overrideQuotaForOrigin` 当前属于实验能力；如果它报告 active 但仍允许 IndexedDB 写入，用例会明确 skip。该结果不能替代受限磁盘 profile 或真实设备产生的 `QuotaExceededError` 证据，AD-025 因此继续保持处理中。

生产 Prometheus 查询：

```promql
dipole:web_sync_shadow:promotion_ready
ALERTS{alertname=~"DipoleWebSync(ShadowDivergence|ShadowOverflow|StorageFull|ClientErrors)",alertstate="firing"}
sum by (outcome) (increase(dipole_web_sync_comparison_total{scope="incoming_direct"}[24h]))
sum by (outcome) (increase(dipole_web_sync_client_errors_total[24h]))
```
