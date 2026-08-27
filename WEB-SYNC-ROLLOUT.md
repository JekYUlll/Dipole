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
7. 保留 `shadow` 或 `off` bundle 以及服务端旧接口；扩大默认范围前复跑 AD-025 的真实容量、共享设备和进程强退验收。

## 6. 回切

客户端回切只改变构建变量，不迁移或删除服务端数据：

```text
VITE_SYNC_ENGINE_MODE=primary -> shadow -> off
```

`shadow` 回切后旧 Offline 重新驱动界面，并继续收集对照证据；紧急情况下直接回到 `off`。IndexedDB 中已提交的 Sync 数据按用户隔离保留；显式退出、被动 401、WS kick 和账号切换统一清理当前账号。公共设备应使用显式退出；若浏览器在清理完成前被强制终止，从浏览器设置中清除 Dipole 站点数据。

## 7. 验证命令

```bash
./scripts/check-web-sync-alerts.sh
docker compose -f docker-compose.cluster.yml --profile observability config
cd frontend
npm ci
npm run test:e2e:install
npm run test:e2e
cd ..
./scripts/check-web-sync-real-quota.sh
```

Playwright 运行 Chromium、Firefox、WebKit 的生产 IndexedDB 实现，覆盖容量淘汰、重开、账号隔离、延迟清理和页面重载中断事务。Chromium 额外启动独立 persistent profile，在生产 `commitPage` 仍 pending 时通过 CDP `Browser.crash` 终止浏览器主进程，再以同一 profile 重启并验证 Message、manifest 与安全 Cursor 的整页原子性。Linux WebKit 需要先执行 Playwright 官方 `install-deps webkit`，CI 可使用与 `@playwright/test` 版本一致的官方镜像。

Chromium CDP `Storage.overrideQuotaForOrigin` 当前属于实验能力；如果它报告 active 但仍允许 IndexedDB 写入，用例会明确 skip。`check-web-sync-real-quota.sh` 使用无特权 user/mount namespace 挂载 128 MiB tmpfs，并预留 24 MiB reserve file；独立 Chromium profile 持续提交随机不可压缩正文，真实拒绝后释放 reserve，再读取数据库验证失败页原子性。普通 E2E 默认跳过该外部门禁，Linux CI 需要允许 user namespace 和 tmpfs mount。完整进程强退、受限容量和共享设备 401/kick 已纳入自动验收，AD-025 已关闭。

生产 Prometheus 查询：

```promql
dipole:web_sync_shadow:promotion_ready
ALERTS{alertname=~"DipoleWebSync(ShadowDivergence|ShadowOverflow|StorageFull|ClientErrors)",alertstate="firing"}
sum by (outcome) (increase(dipole_web_sync_comparison_total{scope="incoming_direct"}[24h]))
sum by (outcome) (increase(dipole_web_sync_client_errors_total[24h]))
```

### 7.1 可恢复观察会话与证据归档

发布 shadow 候选前，将实际部署的 `frontend/dist` 制作为不可变发布归档。观察会话必须使用完整 40 位 Git commit 和该归档文件，不能用源码目录或任意占位文件代替：

```bash
python3 scripts/web_sync_observation.py start \
  --candidate-version web-sync-shadow-20260828.1 \
  --git-commit "$(git rev-parse HEAD)" \
  --bundle /secure/releases/web-sync-shadow-20260828.1.tar \
  --prometheus-url https://prometheus.example.internal \
  --output /secure/evidence/web-sync-shadow-20260828.1.session.json
```

`start` 会先确认 incoming-direct comparison series 已存在、Sync Projector lag 为零且相关告警没有 firing。Session ID 覆盖候选版本、commit、bundle SHA-256、Prometheus 地址、开始时间和初始原始响应；输出文件使用不可覆盖写入。

观察期间可读取状态，命令不会修改 Session：

```bash
python3 scripts/web_sync_observation.py status \
  --session /secure/evidence/web-sync-shadow-20260828.1.session.json
```

满 24 小时后使用同一 commit 和发布归档完成证据。工具会重新计算 bundle SHA-256，候选漂移、窗口不足或 Session 篡改都会拒绝归档：

```bash
python3 scripts/web_sync_observation.py finalize \
  --session /secure/evidence/web-sync-shadow-20260828.1.session.json \
  --git-commit "$(git rev-parse HEAD)" \
  --bundle /secure/releases/web-sync-shadow-20260828.1.tar \
  --output /secure/evidence/web-sync-shadow-20260828.1.evidence.json
```

退出码 `0` 表示 `eligible`，退出码 `2` 表示已归档但门禁判定为 `blocked`，输入、网络、时间或完整性错误返回 `1`。Evidence 保存原始 Prometheus API 响应、Session/快照/完整证据 SHA-256 和具体阻塞原因；不得包含凭据、Message UUID 或正文。将 Session、Evidence 与候选发布归档一并保存到启用版本控制和保留策略的受控对象存储，并在晋级记录中固定 object version、ETag 和责任人。

本地验证工具与契约：

```bash
./scripts/check-web-sync-observation.sh
```

## 8. Timeline Notification Shadow

该链路验证轻量在线通知能否可靠定位 Direct/普通群 Timeline，同时继续由现有 `chat.message` 正文驱动界面。热群不发送 `sync.item.notify.v1`，继续使用聚合 `group.message.notify` 后按 Seq 补拉。

部署时需要同时启用服务端与候选 Web 构建；任一侧未启用都不会改变现有投递：

```bash
export DIPOLE_MESSAGE_TIMELINE_NOTIFY_MODE=shadow
VITE_TIMELINE_NOTIFY_MODE=shadow npm run build
```

通知仅包含 `schema_version=1`、`event_id`、`message_uuid`、`conversation_key`、`message_seq` 和目标 locator。Web 按会话串行执行 `after_seq` 拉取；重复或过期 event 由有界 event-ID 集合过滤，通知间隔中的 Seq 缺口由下一次补拉恢复。shadow 校验不得替换、延迟或阻断完整消息展示。

Prometheus 使用独立有界指标与 recording rules：

```promql
sum by (outcome) (increase(dipole_web_timeline_notify_shadow_total[24h]))
dipole:web_timeline_notify_shadow:window_complete
dipole:web_timeline_notify_shadow:promotion_ready
ALERTS{alertname=~"DipoleWebTimelineNotify(Divergence|VerifierErrors)",alertstate="firing"}
```

晋级评审要求同一候选版本连续运行满 24 小时，`match >= 100`，且 `missing + mismatch + error + invalid == 0`。规则测试通过只确认表达式语义；真实 Prometheus 快照、构建版本、窗口时间和告警状态需要另行归档。达到门槛后仍需单独评审 primary 路由，当前运行时只接受 `off|shadow`。

回滚同时关闭两个开关并重新发布候选 Web bundle：

```text
DIPOLE_MESSAGE_TIMELINE_NOTIFY_MODE=off
VITE_TIMELINE_NOTIFY_MODE=off
```

回滚不涉及数据迁移；完整 WS 消息、旧 Offline、IndexedDB Sync 和热群 notify + pull 均保持原路径。
