# Dipole 项目学习、简历与面试主文档

本文档是 Dipole 的简历、现场介绍和复盘入口。内容必须以代码、测试、基准报告和架构文档为依据；详细题库见 [面试问答](INTERVIEW-QA.md)。

## 1. 使用规则

每次新增可对外描述的能力时，更新以下五项：

1. 简历描述与现场介绍。
2. 对应的证据链接与状态。
3. 至少一个可追问的问题。
4. 已知限制与下一步学习方向。
5. 对应合并切片的验证命令或归档证据。

状态标签：

- **已验证**：有实现和可复核测试、Smoke 或性能证据。
- **默认关闭**：实现与门禁存在，生产切流仍缺共享环境或审批证据。
- **规划中**：只记录设计方向，不能写入简历成果。

### 滚动维护契约

本文件是学习、答辩和面试叙事的主入口。架构设计、运行手册、测试报告、设计稿和更新日志保留各自的事实细节；本文件只汇总可讲结论、证据链接、追问和限制，避免复制后发生漂移。

| 触发项 | 本文档必须同步的内容 | 可引用的事实源 |
| --- | --- | --- |
| 新增或改变服务/数据边界 | 简历描述、60 秒/3 分钟介绍、至少一个取舍问答 | 架构计划、服务边界、契约与 migration |
| 新增用户可见流程或 Pencil 基线 | 产品演示步骤、界面状态和视觉回归证据 | `design/`、前端设计计划、Playwright 用例 |
| 新增 Agent 能力或权限状态 | Capability 边界、审批链与默认开关 | Agent Runtime 设计、授权与部署手册 |
| 取得性能、远程或故障演练结果 | 可复现环境、指标、适用范围和限制 | 基准报告、运行记录、更新日志 |
| 切换默认路径或发现风险 | 状态标签、限制和下一步 | 架构债务台账、回滚手册 |

每个合并切片至少复核本文档是否受影响；若无变化，在切片的测试/合并记录中注明“面试叙事无变化”。涉及服务边界、默认开关、用户可见流程、性能结论、Agent 权限或存储责任的切片必须更新能力卡片。所有描述继续遵守证据优先：实现与测试齐备才标记为“已验证”，部署门禁齐备但缺共享环境证据标记为“默认关闭”，设计或待验收内容标记为“规划中”。

### 合并前复核清单

每次合并前按以下顺序复核，避免简历、演示与实际实现脱节：

1. 变更是否影响项目的一句话定位、简历句或 60 秒介绍。
2. 是否存在可复现的最短演示，以及其依赖的 fixture、开关和环境。
3. 证据是否明确区分本地、隔离远端与共享环境，且没有扩大已验证范围。
4. 是否需要在 [面试问答](INTERVIEW-QA.md) 新增或修订追问。
5. 是否已同步更新日志、架构债务台账和对应能力卡片。

### 合并时的更新记录

每个会改变项目定位、可演示能力、验证状态或技术取舍的合并切片，都在本文档对应能力卡片或新增能力卡片中追加一条简短更新记录。记录采用以下格式，日期使用切片合并日：

```md
#### YYYY-MM-DD · <能力名称>

- **状态：** 已验证 / 默认关闭 / 规划中
- **简历句：** 可放入简历的一句话。
- **对外表述：** 可用于现场介绍的一句话。
- **演示：** 受控环境中的最短复现步骤。
- **证据：** 实现、测试、基准、运行记录或设计稿链接。
- **追问：** 一个可展开的工程问题及回答入口。
- **限制：** 当前证据未覆盖的边界。
- **下一步：** 继续推进前必须完成的学习、验证或设计工作。
- **复核条件：** 下次必须重新核验的开关、环境或指标。
```

`README.md` 只保留项目定位、启动和目录入口；`CHANGELOG.md` 只记录时间线；架构文档、运行手册和测试报告保留可复核细节。本文档负责把这些事实组织为可讲、可演示、可追问的叙事，不复制实现细节。若文档之间发生冲突，以代码、契约、测试和归档运行记录为准，并在下一次合并中修正叙事。

### 能力卡片模板与索引

每个可讲的合并切片在本文档新增或更新一张能力卡片，固定保留以下字段：`状态`、`简历句`、`现场演示`、`证据`、`追问`、`限制`、`下一步` 和 `复核条件`。更新日志记录变更时间线，能力卡片只保留可复述、可核验的当前结论。自动门禁检查文档入口、核心章节和模板字段；它不判断技术结论本身，结论仍须由代码与证据复核。

| 能力卡片 | 状态 | 简历句与现场演示入口 | 证据与追问 |
| --- | --- | --- | --- |
| 实时 IM 与 Timeline | **已验证** | 第 3 节后端描述；第 4 节 60 秒/3 分钟介绍 | [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md)；“三个 Seq 为什么分开？” |
| 渐进式微服务与 SQLC | **已验证** | 第 3 节后端描述；第 5 节渐进微服务故事 | [服务边界](../architecture/SERVICE-BOUNDARIES.md)；“为什么不一次性拆分？” |
| Agent Runtime 与权限 | **已验证** | 第 3 节 Agent 描述；第 5 节 Agent 安全与可恢复执行 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)；“模型为何不能决定权限？” |
| Owner-reviewed Memory 类型晋级 | **已验证（本地）** | 受控 candidate/review 选择 semantic 等持久类型；Temporal 只在显式 commit 请求下调用可注入 Activity | [Memory promotion 契约](../../contracts/agent-memory-promotion/v1/README.md)、[active executor 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)；“为何 working 不能晋级？” |
| Receipt Commit 最小权限接线 | **默认关闭** | Core 以 mTLS 配置门禁按需注册仅含 receipt commit 的 Agent RPC；其余 Agent 能力不在独立 Core 暴露 | [active executor 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)、[Agent Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)；“为何不用完整 Agent RPC adapter？” |
| Receipt Commit Active Worker | **默认关闭** | 独立 `promotion_active` Worker 需同时通过 Runtime、mTLS、operator authority 与 Core/Runtime 双开关门禁 | [Agent Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)、[active executor 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)；“为什么仍需要 Core grant？” |
| Receipt Commit Drill Evidence | **已验证（本地）** | 将共享环境 commit、重试、grant 撤销与回滚演练压缩为低敏、可判定的 evidence record | [Worker drill 契约](../../contracts/agent-memory-promotion/v2/worker-drill-evidence.schema.json)、[Agent Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)；“CLI 能否代替真实演练？” |
| Receipt Commit Promotion Compose Gate | **已验证（本地）** | 受控渲染 active + promotion overlay，确认写入前的多重静态开关与 authority | `scripts/check-compose.sh`；“Compose 通过能证明写权限安全吗？” |
| Receipt Commit mTLS RPC Drill | **已验证（隔离 Remote GPU）** | 用 Go fixture 与 TS generated client 验证跨语言 prepared receipt 的 mTLS 提交 | `scripts/drill-agent-memory-promotion-rpc.sh`；“fixture 能证明真实写入吗？” |
| Receipt Commit MySQL mTLS Contract | **已验证（隔离 Remote GPU）** | 经实际 Core receipt adapter、TCP+mTLS 和 Agent 证书身份验证 receipt 到持久 candidate/review 和 Memory 事务的完整约束 | `scripts/test-agent-memory-promotion-mysql-contract.sh`；“为何还需要 Temporal 联合演练？” |
| Temporal/Core/MySQL Receipt Retry | **已验证（隔离 Remote GPU）** | 在首次持久提交后故意失败 Activity，验证重试经 mTLS 返回同一条 MySQL Memory，并验证 admission 后 grant 撤销拒绝 | `scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh`；“撤销后为何预 admission Run 不能继续写入？” |
| MinIO Multipart 与可恢复上传 | **已验证（隔离 Remote GPU）** | 上传超过阈值的文件，展示分片、暂停、恢复与完成；预签名直传维持候选状态 | [大文件上传计划](../architecture/PLATFORM-EVOLUTION-PLAN.md)；“为什么还不默认切到预签名直传？” |
| Agent Definition Catalog | **已验证（本地）** | 只读目录演示：版本、scope 和 runtime 关闭边界 | `frontend/src/components/AgentDefinitionCatalog.vue`、`frontend/e2e/agent-definitions.spec.ts`、`frontend/e2e/agent-definitions.visual.spec.ts`；认证流程已通过 Chromium/Firefox/WebKit，视觉基线仅固定 Chromium；“为何 Definition 目录不提供激活或编辑？” |
| Artifact 与 Task Timeline 关联 | **已验证（本地）** | Timeline `artifact` 事件以内容寻址 ID 打开 owner-scoped metadata 页面，并固定正文与下载关闭边界 | [Timeline 契约](../../contracts/agent-task-timeline/v1/README.md)、`frontend/src/components/AgentArtifactMetadata.vue`、`frontend/e2e/agent-artifact.spec.ts`；认证读取已通过 Chromium/Firefox/WebKit，视觉基线仅固定 Chromium；“为什么 Timeline 只返回 Artifact ID？” |
| Active Agent、外部 MCP 与 C++ 数据面 | **默认关闭 / 规划中** | 仅展示门禁、Shadow 与回滚设计，不作为上线能力演示 | [架构债务台账](../architecture/ARCHITECTURE-DEBT.md)；“何时允许切流？” |

#### 2026-08-30 · Owner-reviewed Memory 类型晋级

- **状态：** 已验证（本地）
- **对外表述：** 将 Agent Memory 的类型策略从 Runtime 校验延伸至持久化事务：owner 在已接受 review 后可将 observational candidate 晋级为 semantic、episodic、procedural 或 observational Memory，并由 Gateway、gRPC、Core 和 MySQL 共同校验。
- **演示：** 使用受控 candidate/review 调用 promotion RPC，指定 `semantic` 后读取返回 Memory 类型；再提交 `working`，确认 Gateway 返回 400 且未触发写入。
- **证据：** [Memory promotion 契约](../../contracts/agent-memory-promotion/v1/README.md)、[active executor 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)、`internal/services/agent/application/agent_memory_candidate_promotion_test.go`、`internal/transport/grpc/agent/server_test.go`、`services/agent-runtime/src/temporal/agent-memory-promotion-commit-activity.test.ts`。
- **追问：** “为什么 working 不能晋级？” working 只服务当前 Task 的短期推理状态，持久化会扩大生命周期和检索范围；长期 Memory 必须经过 owner review，并在事务内绑定 candidate 与 review。
- **限制：** 当前路径使用 owner 控制 RPC；TS receipt v2、Agent-only internal RPC、低敏回包校验、默认关闭的 Core mTLS bootstrap 以及可注入 Temporal Activity 已固定，但受控 Worker 组合、授权演练和共享环境证据尚未接入，不能宣称 active Agent 已自动写入长期 Memory。
- **复核条件：** 接入 receipt、Temporal Activity、active authority 或增加新的 Memory 类型时。

#### 2026-08-30 · Receipt Commit 最小权限接线

- **状态：** 默认关闭
- **对外表述：** 为 Agent Memory receipt 增加独立 Core 接线：开关开启时要求内部 RPC mTLS，并注册仅实现 `CommitMemoryPromotionReceipt` 的专用 Adapter；独立 Core 不会为单个提交接口装配完整 Agent Runtime 面。
- **演示：** 以 `dipole-agent` 服务身份连接受控 Core gRPC，提交低敏 receipt binding 并确认返回已晋级 Memory 的低敏标识；使用 Gateway 身份或关闭开关时确认接口不可用或被拒绝。
- **证据：** `internal/services/core/bootstrap/runtime.go`、`internal/transport/grpc/agent/memory_promotion_receipt_server.go`、`internal/bootstrap/internal_rpc_test.go`、[active executor 契约](../../contracts/agent-memory-promotion/v2/ACTIVE-EXECUTOR-DESIGN.md)。
- **追问：** “为何不用完整 Agent RPC adapter？” 独立 Core 当前只拥有 receipt commit 所需的持久化 resolver 与 promotion 事务。收窄 RPC surface 可以避免无关 Tool、Task、Artifact 或 owner-control 入口在该部署单元被意外暴露。
- **限制：** 默认开关关闭；Temporal Worker 仍未完成 active authority 组合，且没有共享环境 mTLS、重放与授权演练证据，因此不能表述为自动长期 Memory 写入已启用。
- **复核条件：** 改变 RPC caller allowlist、mTLS 配置、Temporal Worker 组合或 receipt schema 时。

#### 2026-08-30 · Receipt Commit Active Worker Authority

- **状态：** 默认关闭
- **对外表述：** 为 reviewed Memory receipt 设计独立 `promotion_active` Temporal Worker profile：Worker 仅在 active Runtime、Temporal、Capability RPC mTLS、显式 operator authority 与 Core/Runtime 双开关同时满足时装配 commit Activity。
- **演示：** 以 `agent-active.yml` 加 `agent-memory-promotion.yml` 执行受控 Compose 渲染，确认缺失 authority 或 TLS 时启动拒绝；用合法 profile 运行 fixture Workflow，确认基础 Worker 仍拒绝 commit，而 promotion profile 才尝试低敏 receipt RPC。
- **证据：** `services/agent-runtime/src/runtime/active-memory-promotion-profile.ts`、`services/agent-runtime/src/temporal/agent-memory-promotion-commit-activity.ts`、`deploy/microservices/agent-memory-promotion.yml`、[Agent Active 部署手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)。
- **追问：** “为什么 Worker 已有 operator authority 还需要 Core grant？” Worker authority 只控制是否能尝试调用，Core 仍从持久 Task/Run 恢复主体并复核 active admission、有效 grant、receipt 与 candidate/review，避免环境配置成为数据写入授权。
- **限制：** 当前仅有本地配置与 Activity 组合测试；共享环境的有效提交、Temporal 重试、失效 grant、观测窗口和回滚演练尚未归档，默认不加载该 overlay。
- **复核条件：** 修改 release manifest、Runtime profile、Core grant、mTLS、Temporal queue 或 receipt schema 时。

#### 2026-08-30 · Receipt Commit Drill Evidence

- **状态：** 已验证（本地）
- **对外表述：** 为 Agent Memory receipt 的受控写入演练建立独立 evidence contract，将首个 commit、重试幂等、失效 grant 拒绝与 overlay 回滚绑定到同一候选版本和摘要，缺少任一结果即保持 blocked。
- **演示：** 使用脱敏演练 JSON 运行 `npm run promotion:memory-worker-drill -- --evidence=<path>`，确认完整记录返回 `eligible`；将 retry 的 Memory ID 改为不同值，确认返回 `blocked`。
- **证据：** [Worker drill 契约](../../contracts/agent-memory-promotion/v2/worker-drill-evidence.schema.json)、`services/agent-runtime/src/promotion/memory-promotion-worker-drill-evidence.test.ts`、`services/agent-runtime/src/promotion/memory-promotion-worker-drill-cli.test.ts`。
- **追问：** “CLI 能否代替真实演练？” CLI 只检查人工归档结果的绑定和完整性，不访问实际系统；原始服务日志、Temporal 记录、指标快照、审批和回滚工单仍是共享环境证据的一部分。
- **限制：** 当前通过的是本地契约与 CLI；没有任何共享环境提交、grant 撤销或回滚运行记录，因此默认写路径继续关闭。
- **复核条件：** 修改 receipt、grant、Worker profile、Core 入口或演练标准时。

#### 2026-08-30 · Receipt Commit Promotion Compose Gate

- **状态：** 已验证（本地）
- **简历句：** 为 Agent Memory 的受控写入建立多层 Compose 配置门禁，要求 Core receipt commit、Temporal promotion Worker 与显式 operator authority 同时成立。
- **对外表述：** 将 active 与 promotion overlay 组合渲染，并断言默认关闭的 Control、MCP、外部 MCP 和自动 Memory 不会随写入能力一并开放。
- **演示：** 运行 `bash scripts/check-compose.sh`；移除 `DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY` 后，promotion overlay 的 Compose 渲染应失败。
- **证据：** `scripts/check-compose.sh`、`deploy/microservices/agent-active.yml`、`deploy/microservices/agent-memory-promotion.yml`。
- **追问：** “Compose 通过能证明写权限安全吗？” 静态渲染只能阻止开关组合漂移；Core 仍需复核 mTLS caller、持久 Task/Run、grant、receipt 与 candidate/review，真实环境还需重放、撤销和回滚证据。
- **限制：** 当前没有启动 Temporal、Core 或 Kafka，也未实际提交任何 Memory。
- **下一步：** 在维护窗口内归档真实 Core/Temporal 的首个提交、幂等重试、grant 撤销和 overlay 回滚证据。
- **复核条件：** 修改 Worker mode、Core 开关、operator authority、active overlay 或 Compose 基础环境时。

#### 2026-08-30 · Receipt Commit Remote Development Baseline

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 将 Agent Memory promotion 的开发期验证迁移到隔离 Remote GPU worktree，在不占用 GPU 或启动业务容器的前提下复核 durable Workflow 与 TypeScript Runtime 边界。
- **对外表述：** 固定 Node 22、候选 revision 和隔离 in-memory Temporal，验证 prepared receipt、临时失败重试与严格 promotion profile 配置，避免本机资源压力影响开发门禁。
- **演示：** 在独立 worktree 运行 `DIPOLE_AGENT_TEMPORAL_INTEGRATION=true npm test -- --run src/temporal/agent-memory-promotion-workflow.integration.test.ts src/runtime/active-memory-promotion-profile.test.ts`，再运行 `npm run typecheck`。
- **证据：** `services/agent-runtime/src/temporal/agent-memory-promotion-workflow.integration.test.ts`、`services/agent-runtime/src/runtime/active-memory-promotion-profile.test.ts`、[开发工作流](../operations/DEVELOPMENT-WORKFLOW.md)。
- **追问：** “为什么远端通过仍不能宣称 active Memory 写入已上线？” 该测试使用 in-memory Temporal 和 commit stub，只验证 Worker 的 durable retry 语义及配置边界；真正提交仍依赖 Core mTLS、持久 grant、candidate/review 事务、Kafka 事件与可执行 rollback。
- **限制：** 未启动 Compose，未连接共享 Core/Kafka，未进行真实 grant 撤销、跨进程 mTLS 或回滚演练。
- **下一步：** 在维护窗口部署隔离 Core 与 Temporal 后，以受控 candidate/review 验证首次提交、相同 receipt 重试、失效 grant 拒绝与 overlay 回滚。
- **复核条件：** Node/Temporal SDK、receipt schema、Worker retry、Core RPC 或远程验证工作流变化时。

#### 2026-08-30 · Receipt Commit mTLS RPC Drill

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 为 Agent Memory receipt 建立 Go/TypeScript 跨语言 mTLS 演练，验证 Agent 身份、prepared receipt 的 protobuf 编码与低敏回包绑定。
- **对外表述：** 通过临时 CA 和 loopback fixture，让 TypeScript generated client 实际调用 Go gRPC 服务；错误 secret 与错误证书会在服务端拒绝。
- **演示：** 使用 Node 22 与显式 `DIPOLE_GO_BIN` 运行 `scripts/drill-agent-memory-promotion-rpc.sh`，确认 fixture 记录一次 authenticated receipt commit。
- **证据：** `scripts/drill-agent-memory-promotion-rpc.sh`、`internal/bootstrap/agent_mcp_rpc_drill_fixture_test.go`、`services/agent-runtime/src/capabilities/agent-memory-promotion-rpc-drill.integration.test.ts`。
- **追问：** “fixture 能证明真实 Memory 写入吗？” 它覆盖 TLS、服务身份、RPC 序列化和 client/server response binding；真实 Core 仍需从持久 Task/Run 恢复主体、复核 grant 与 candidate/review，再执行 MySQL 事务。
- **限制：** 未启动 Docker、Temporal、Kafka 或 MySQL，且 fixture 不包含真实 Core application service。
- **下一步：** 在维护窗口将相同 receipt 场景接入隔离 Core、Temporal 和持久 candidate/review，再归档撤销与回滚证据。
- **复核条件：** 修改 protobuf、mTLS caller policy、receipt schema、TS client 或远端 Go toolchain 时。

#### 2026-08-30 · Receipt Commit MySQL mTLS Contract

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 为 Agent Memory receipt 建立真实 MySQL、Core adapter 与 mTLS 联合契约，验证证书身份、方法白名单、持久授权、candidate/review 复核和幂等晋级事务。
- **对外表述：** 临时 MySQL 8.4 完整执行 migration 后，`dipole-agent` 使用客户端证书经 TCP 调用实际 Core receipt adapter；receipt 只能在 active grant、持久 Task/Run 与 owner-reviewed candidate 同时有效时创建 Memory，重复提交返回同一条 Memory，撤销 grant 后拒绝。
- **演示：** 在独立 worktree 运行 `DIPOLE_GO_BIN=/home/admin1/.local/go-1.27.0/bin/go GOTOOLCHAIN=local scripts/test-agent-memory-promotion-mysql-contract.sh`。
- **证据：** `internal/services/agent/infrastructure/mysql/agent_memory_promotion_receipt_contract_test.go`、`scripts/test-agent-memory-promotion-mysql-contract.sh`、`deploy/agent/memory-promotion-mysql-contract.compose.yml`。
- **追问：** “为何 MySQL mTLS 通过后还需要 Temporal 演练？” 该测试验证持久事务、adapter 映射、TCP+mTLS 和服务身份；Temporal durable retry、Worker 生命周期与 rollback 仍由独立隔离演练覆盖，最后仍需组合证据。
- **限制：** 测试使用临时 CA、loopback listener 和临时 MySQL，不启动 Temporal 或 Kafka；共享环境 authority、撤销和 rollback 运行记录仍缺失。
- **下一步：** 将同一低敏 receipt 场景接入隔离 Core、Temporal 与 MySQL，归档跨进程撤销和回滚证据。
- **复核条件：** 修改 Memory lineage、candidate/review schema、receipt binding、grant 解析或 MySQL migration 时。

#### 2026-08-30 · Receipt Commit Active Grant Composition

- **状态：** 已验证（本地）
- **简历句：** 收敛 Core 与 embedded Agent receipt commit 的授权组合，使持久 active grant 成为真实写入前的必经复核条件。
- **对外表述：** Receipt 不信任 Runtime 自报身份；Core 从 Task/Run 恢复 invocation，并以持久 promotion grant authorizer 复核 active 权限。
- **演示：** 运行 receipt/Core bootstrap 定向测试，确认缺失或失效 grant 返回 policy denied。
- **证据：** `internal/services/core/bootstrap/runtime.go`、`internal/bootstrap/embedded/runtime/runtime.go`、`internal/services/agent/application/agent_execution_policy.go`。
- **追问：** “为什么只在 Worker 做开关检查还不够？” Worker 配置无法替代 Core 对持久 Task/Run 与有效 grant 的权威复核。
- **限制：** 当前未完成 Core、Temporal 和 MySQL 的单次跨进程联合演练，默认 active Worker 继续关闭。
- **下一步：** 以隔离全栈 fixture 归档首次提交、重试、grant 撤销与回滚证据。
- **复核条件：** 修改 Core composition、Runtime grant schema、receipt RPC 或 Worker profile 时。

#### 2026-08-30 · Embedded Active Run Admission

- **状态：** 已验证（本地）
- **简历句：** 统一 embedded Agent 的 active admission 与 receipt commit 授权边界，避免 Task/Run 创建和后续提交使用不同 grant 判断。
- **对外表述：** 同一持久 promotion grant 同时约束 active Run admission 与 Core receipt commit，保证可执行任务与写入授权语义一致。
- **演示：** 运行 Agent execution policy 定向测试，核验有效 grant 允许 admission，撤销后拒绝。
- **证据：** `internal/bootstrap/embedded/runtime/runtime.go`、`internal/services/agent/application/agent_execution_policy_test.go`。
- **追问：** “为什么 admission 与 commit 都要复核 grant？” 两个阶段相隔时间较长；双重复核可阻止已撤销 grant 的既有任务继续写入。
- **限制：** 当前为本地组合与领域测试，尚无跨进程 Core/Temporal/MySQL 联合回放证据。
- **下一步：** 构建隔离联合演练并验证撤销发生在 admission 之后的提交拒绝。
- **复核条件：** 改动 embedded runtime、Run lifecycle、grant 有效期或 receipt 提交路径时。

#### 2026-08-30 · Temporal Receipt Commit Retry

- **状态：** 已验证（隔离 Temporal）
- **对外表述：** 为 reviewed Memory receipt 的 durable Workflow 补充临时提交失败重试：Temporal 会重用同一 prepared receipt，最终仅记录与 receipt hash 对应的低敏 Memory binding。
- **演示：** 启用 `DIPOLE_AGENT_TEMPORAL_INTEGRATION=true` 运行 promotion workflow integration test，观察首个 commit Activity 故意失败一次后由重试收敛为 completed。
- **证据：** `services/agent-runtime/src/temporal/agent-memory-promotion-workflow.integration.test.ts`、`services/agent-runtime/src/temporal/agent-task-workflow.ts`。
- **追问：** “为什么重试不会重复写入？” Worker 重试复用同一 receipt；Core 的 candidate/review promotion 事务对同一 binding 幂等，重复提交应返回同一 Memory。集成测试固定 Worker 侧 receipt 一致性，Core 事务幂等由 Go 合约测试覆盖。
- **限制：** 此测试使用 stubbed commit Activity，不连接真实 Core、grant 或共享 Kafka/Temporal，因此不能代替共享环境重放、撤销与回滚演练。
- **复核条件：** 改动 Workflow retry、receipt schema、Core 回包绑定或 Activity mode 时。

#### 2026-08-30 · Temporal/Core/MySQL Receipt Retry

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 设计 Agent Memory promotion 的跨语言 durable retry 合约：Temporal Activity 在 Core 已持久化后故障重试，经 mTLS 复用同一 receipt 并收敛到同一条 MySQL Memory。
- **对外表述：** receipt 绑定 candidate/review、目标类型和时效；Core 每次提交重新解析 Task/Run 与有效 grant，MySQL 用候选的确定性 Memory ID 保证幂等。首次提交时间由持久记录保存，重试墙钟不会改变其语义；已 admission 的 Run 在 grant 撤销后也无法继续写入。
- **演示：** 在 Remote GPU 一次性 worktree 中使用显式 Node 22 和 Go 1.27 运行 `scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh`，观察第一次 commit 后的受控 Activity 失败、第二次调用成功，以及撤销 grant 后的 `PERMISSION_DENIED`。
- **证据：** `services/agent-runtime/src/temporal/agent-memory-promotion-mtls-mysql.integration.test.ts`、`internal/services/agent/infrastructure/mysql/agent_memory_promotion_temporal_fixture_test.go`、`internal/services/agent/infrastructure/mysql/agent_memory_candidate.go`、`scripts/drill-agent-memory-promotion-temporal-mysql-mtls.sh`。
- **追问：** “撤销后为何预 admission Run 不能继续写入？” admission 与 receipt commit 都查询有效 grant；夹具在两个 Run admission 后撤销同一 grant，第二个 receipt 仍由 Core 拒绝，避免旧 Task/Run 因缓存授权继续写入。
- **限制：** Temporal 为内存 test server，MySQL/证书/监听器均为临时资源；没有接入 Kafka，也未验证 overlay 回滚或业务级 Memory rollback。
- **下一步：** 在受控共享环境归档 Kafka trigger、overlay 回滚、业务级 Memory rollback 和观测窗口证据，再评估 `promotion_active` 的灰度。
- **复核条件：** 修改 receipt canonicalization、candidate promotion 事务、Temporal retry、Core caller policy、mTLS 或 Memory schema 时。

#### 2026-08-30 · MinIO Multipart 与可恢复上传

- **状态：** 已验证（隔离 Remote GPU）
- **简历句：** 基于 MinIO S3 Multipart Upload 实现大文件分片、校验、暂停恢复、会话续传、Redis/对象存储对账与生命周期清理，并以版本化策略保持预签名直传默认关闭且可即时回切。
- **对外表述：** 文件数据面交给对象存储，Core 保留上传会话、文件所有权、ETag/大小登记和完成事务。浏览器按文件指纹恢复已确认分片，网络异常与 `408`、`429`、`5xx` 执行有界退避，确定的预签名 `4xx` 立即失败。
- **演示：** 在隔离环境运行 `scripts/smoke-minio-multipart.sh` 与 `scripts/smoke-minio-multipart-restart.sh`，展示乱序分片、同编号替换、服务重启后续传、完成内容校验和重复 Abort；前端执行 `npm test -- --run src/upload/multipartUpload.test.ts`。
- **证据：** [平台演进计划 A7](../architecture/PLATFORM-EVOLUTION-PLAN.md)、[架构债务 AD-055](../architecture/ARCHITECTURE-DEBT.md)、`internal/services/core/domain/file/file_service.go`、`frontend/src/upload/multipartUpload.ts`、`scripts/smoke-minio-multipart.sh`。
- **追问：** “为什么预签名直传仍然默认关闭？” 对象存储跨域/同源代理、URL 失效、断网恢复、限流、超时和回切必须完成同版本故障矩阵；当前 relay 仍是权威兼容路径。
- **限制：** 浏览器调度的断连、限流、服务端 `5xx` 与永久 `4xx` 已有单元证据，真实代理断网、跨网络恢复、共享环境告警路由和默认直传切流仍未验收。
- **下一步：** 在隔离环境补齐 presigned proxy 的浏览器级网络故障矩阵，并归档切流前的 shadow、回切和负责人批准证据。
- **复核条件：** 修改文件阈值、分片大小、重试策略、URL TTL、对象存储、Gateway proxy 或默认上传策略时。

#### 2026-08-30 · Artifact 与 Task Timeline 关联

- **状态：** 已验证（本地）
- **对外表述：** 为 Agent Task Timeline 增加内容寻址 Artifact 关联，并让主投影与失败修复队列共享同一持久化契约，保证重试后仍可回到相同 Artifact metadata 边界。
- **演示：** 使用受控 Artifact 创建事件读取 Timeline，确认 `kind=artifact` 返回 64 位 `artifact_id`；再通过 owner-scoped metadata API 读取低敏元数据。
- **证据：** [Timeline 契约](../../contracts/agent-task-timeline/v1/README.md)、[Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、`internal/transport/grpc/agent/server_test.go`、`services/agent-runtime/src/capabilities/agent-capability-rpc.test.ts`。
- **追问：** “为什么不在 Timeline 直接返回正文或对象键？” Timeline 是低敏执行索引，正文读取需要独立披露策略、对象访问授权和前端设计，避免时间线接口扩大数据暴露范围。
- **限制：** Artifact metadata 页面仅在默认关闭的 flag 下通过本地组件、三浏览器认证读取、Chromium fixture 和设计导出验证；正文、下载、跨浏览器视觉证据和共享环境运行记录尚未完成。
- **复核条件：** 启用 Artifact Web 页面、正文读取或下载、修改对象生命周期，或改变 Timeline 事件 schema 时。

#### 2026-08-30 · Artifact 只读 Metadata 页面

- **状态：** 已验证（本地）
- **对外表述：** 为内容寻址 Artifact 增加 owner-scoped 的只读 metadata 展示：Timeline 只在 Artifact event 上跳转，页面复核 SHA-256、Task/Run 与低敏元数据，并将正文和下载明确保持关闭。
- **演示：** 使用受控 Timeline Artifact event 打开 metadata 页面，确认只显示类型、版本、大小、Task/Run、创建时间与摘要；模拟读取失败，确认旧 metadata 被清空并只提供重试。
- **证据：** [Pencil 设计说明](../../design/README.md)、`frontend/src/api/agentArtifacts.test.ts`、`frontend/src/components/AgentArtifactMetadata.test.ts`、`frontend/e2e/agent-artifact.spec.ts`。
- **追问：** “为何不直接给 Artifact 加下载链接？” 下载需要独立的对象访问授权、审计和披露策略；当前读取页只承担低敏发现，避免 Timeline 或 metadata API 扩大对象访问面。
- **限制：** 三浏览器功能验证与 Chromium 截图均不能代表共享环境、跨浏览器像素级视觉或任何正文读取能力。
- **复核条件：** 改变 metadata schema、Feature Flag、Timeline 关联，或评审正文/下载授权时。

#### 2026-08-30 · 学习、简历与面试叙事维护

- **状态：** 已验证
- **对外表述：** 为持续演进的分布式 IM 项目维护证据驱动的简历、演示与追问主文档，使能力陈述和验证边界可以随每次架构切片复核。
- **演示：** 从根 README 进入本文档，选择一张能力卡片，沿证据链接复核实现、测试或运行手册，再使用 60 秒与 3 分钟版本介绍该能力。
- **证据：** [根 README](../../README.md)、[文档目录](../README.md)、[更新日志](../../CHANGELOG.md)、[架构债务台账](../architecture/ARCHITECTURE-DEBT.md)。
- **追问：** “如何避免简历描述超过真实验证范围？” 通过状态标签、证据链接、限制项和合并复核，将默认关闭与规划能力从已验证成果中分离。
- **限制：** 文档维护无法替代共享环境运行、压测或生产授权证据；状态需随默认开关和运行结果重新核验。
- **复核条件：** 每个改变服务边界、默认路径、用户可见流程、性能结论或 Agent 权限的合并切片。

#### 2026-08-30 · 学习与面试文档门禁

- **状态：** 已验证（本地）
- **简历句：** 为长期演进的分布式 IM 项目建立证据驱动的学习与面试文档门禁，确保每个可讲能力都有可追溯边界。
- **对外表述：** 将简历句、现场演示、证据、追问、限制、下一步和复核条件固定为能力卡片字段，并在架构文档检查中验证入口与模板。
- **演示：** 运行 `bash scripts/check-learning-interview-doc.sh`；移除 README 入口或任一核心模板字段后，检查应失败。
- **证据：** `scripts/check-learning-interview-doc.sh`、`scripts/check-architecture-docs.sh`、本文档的滚动维护契约。
- **追问：** “脚本文档检查能保证简历描述真实吗？” 它只阻止入口和模板退化；真实性仍依赖代码、测试、基准与运行记录，并在每个合并切片人工复核。
- **限制：** 现有历史能力卡片的字段格式并未批量改写，后续触及对应能力时按新模板补齐，避免制造只改文案的大规模噪声。
- **下一步：** 为 Agent promotion Compose overlay 增加独立渲染与缺失 authority 门禁，并同步相应能力卡片。
- **复核条件：** 修改文档目录、主入口、能力卡片模板或合并流程时。

#### 2026-08-30 · 简历与面试主文档维护契约

- **状态：** 已验证（本地）
- **简历句：** 为持续演进的分布式 IM 项目维护证据驱动的学习与面试主文档，统一简历描述、现场介绍、演示路径与追问边界。
- **对外表述：** 每个可讲能力均以能力卡片串联实现、测试、限制与下一步；项目介绍保留在 README，时间线保留在 CHANGELOG，面试叙事集中在本主文档。
- **演示：** 从根 README 打开本文档，选择能力卡片并沿证据链接复核；合并前运行 `bash scripts/check-learning-interview-doc.sh`。
- **证据：** [根 README](../../README.md)、[面试问答](INTERVIEW-QA.md)、`scripts/check-learning-interview-doc.sh`、本节的合并前复核清单。
- **追问：** “如何避免项目规模增长后文档叙事漂移？” 以文档职责分离、能力卡片、状态标签和自动入口检查约束更新，并在每个影响叙事的合并切片中复核。
- **限制：** 脚本只能检查入口与栏目完整性；技术结论仍需回到实现、测试、基准和运行记录验证。
- **下一步：** 每个后续架构、Agent、前端或性能切片合并时同步复核相关能力卡片与问答。
- **复核条件：** 改变 README 职责、文档目录、简历表述、演示流程或能力卡片字段时。

能力卡片的现场演示必须使用受控 fixture 或隔离环境；涉及真实消息、外部 MCP、生产凭据和写 Capability 时，先按对应运行手册完成授权与脱敏检查。

## 2. 一句话定位

Dipole 是一个面向实时协作与 Agent 能力演进的现代 IM 平台：Go 承担 IM 领域与一致性边界，Kafka 解耦事件与投影，TypeScript Runtime 承担可恢复 Agent Task，并以渐进式微服务化和可回滚切换替代一次性重写。

## 3. 简历描述

### 后端 / 分布式系统版本

```text
Dipole 现代 IM 平台 | Go, sqlc, MySQL, Kafka, Redis, Cassandra, Elasticsearch, MinIO, WebSocket
- 设计消息幂等、Transactional Outbox 与 Kafka 事件链路，按 Message、Conversation 和 Sync Timeline 分离事实存储、用户会话状态与多端增量同步。
- 将 Core、Gateway、Message、Sync、Search 抽象为可独立部署的服务边界，保留 embedded 兼容路径，并以 gRPC、版本化契约、Shadow、回滚和故障演练推进迁移。
- 基于 MinIO S3 Multipart Upload 实现大文件分片、校验、暂停恢复、会话续传、生命周期清理与 Redis/对象存储对账；预签名直传通过版本化策略保持默认关闭和可回切。
- 面向热点群采用 notify + pull 与增量 Timeline；基准记录 100 成员场景的投递、Kafka lag、Inbox 写放大和端到端延迟，性能结论均以归档报告为准。
```

### Agent / AI 工程版本

```text
Dipole Agent Runtime | TypeScript, Node.js, Temporal, Kafka, MCP, OpenTelemetry
- 构建事件驱动的 Agent Task Runtime，包含可信 ExecutionContext、Capability Registry、Context Compiler、Memory 策略、Temporal Workflow 与人工审批状态。
- 以 Provider 注入、模型调用审计、结构化输出、预算限制和五类 Eval 管理模型路径；MCP、写 Capability 和 active authority 按默认关闭与显式 promotion 证据控制。
- 设计 OpenAI-compatible Provider、独立 consumer group、user-gray Active Compose profile 与可执行回滚，避免模型、Tool 和部署配置绕过权限边界。
```

### 使用边界

简历中可使用“已设计并实现”“已通过本地/隔离环境验证”等准确表述。不要把 Cassandra 主读、Elasticsearch 默认搜索、Agent active authority、外部 MCP 写入或 C++ 数据面性能收益写成已上线成果，详见 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md)。

### 面试证据速查

| 叙事主题 | 当前状态 | 面试前应复核的证据 |
| --- | --- | --- |
| 服务边界与回滚 | **已验证** | [服务边界](../architecture/SERVICE-BOUNDARIES.md)、[微服务部署](../architecture/MICROSERVICES-DEPLOYMENT.md) 与对应 Smoke 记录 |
| SQLC 数据访问 | **已验证** | [数据访问迁移说明](../data/DATA-ACCESS-MIGRATION.md) 与版本化 migration/sqlc 查询 |
| Temporal 审批恢复 | **已验证** | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[MCP 授权](../agent/agent-mcp-authorization.md) 与 Workflow 回归测试 |
| Active Agent | **默认关闭** | [Active 部署运行手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md)、release manifest、五类 Eval 与共享环境记录 |
| 外部 MCP Shadow | **默认关闭** | [外部 MCP 连接边界](../agent/agent-external-mcp.md)、`agent-external-mcp-shadow.yml`、Compose 门禁与隔离全栈演练；真实公网/凭据/共享环境证据仍需复核 |
| Cassandra/Elasticsearch 切流 | **默认关闭** | [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 中的回填、对账、Shadow 和回滚门禁 |
| C++ 实时数据面 | **规划中** | [平台演进计划](../architecture/PLATFORM-EVOLUTION-PLAN.md) 与基准报告；在可复现收益前不作性能承诺 |

## 4. 现场介绍

### 60 秒版本

Dipole 是我持续迭代的现代 IM 项目。核心消息链路由 Go 服务负责，通过 Kafka 和 Transactional Outbox 将消息持久化、会话投影、用户同步和实时投递解耦。数据模型把消息历史、用户会话状态和多端 Sync Timeline 分开，使用会话序列和设备 cursor 支持增量同步。项目从模块化单体出发，逐步形成 Core、Gateway、Message、Sync、Search 与 TypeScript Agent Runtime 的服务边界，并为每次切换保留 Shadow、回滚和验证门禁。Agent 部分强调可信上下文、Capability 权限、Temporal 可恢复任务、Memory 和 MCP 安全边界。

### 3 分钟版本

先从 IM 数据模型讲：消息事实按会话 Timeline 存储，Conversation 提供用户视角的摘要和已读状态，Sync Inbox 为每个用户提供可单调推进的同步流。这样历史查询、首页状态和多端增量同步各自有清晰责任。

消息发送经过鉴权、幂等校验、持久化和 Outbox；Kafka 事件再驱动会话、Sync、搜索、实时投递和 Agent 投影。群聊区分普通群和热点群，热点群使用 notify + pull 降低扇出压力。微服务化采用渐进迁移，先稳定接口、数据所有权和 gRPC 契约，再抽离部署单元，embedded 路径用于回滚。

最后是 Agent：Runtime 独立为 TypeScript 服务，通过 Capability RPC 使用 IM 能力，模型无法自行指定用户身份或资源范围。Temporal 管理长任务和审批等待，MCP 与写操作保持默认关闭，active 需要可复核的评测、release manifest、权限和共享环境证据。我的重点是把可靠性、权限和可观测性放在 Agent loop 外层，而非只实现一次模型调用。

## 5. 可展开的工程故事

| 主题 | 可讲的取舍 | 证据与深入材料 |
| --- | --- | --- |
| Sync Timeline | 将历史、会话状态和用户同步流拆开，避免用 `messages.id` 同时承担所有语义 | [消息存储与同步模型](../architecture/MESSAGE-STORAGE-AND-SYNC.md)、[Sync Service](../architecture/SYNC-SERVICE.md) |
| 可靠消息 | 通过 Outbox 缩小“已落库但未发布”缺口，consumer 依赖幂等和重试边界 | [Kafka 事件契约](../data/KAFKA-EVENT-CONTRACT.md)、[面试问答](INTERVIEW-QA.md) |
| 热点群 | 在完整 push 与 notify + pull 之间按负载切换，接受客户端补拉复杂度以控制扇出 | [Realtime Delivery](../architecture/REALTIME-DELIVERY.md)、[性能基线](../performance/PERFORMANCE-BASELINE.md) |
| 大文件上传 | 将对象存储的 multipart 数据面与 Core 的会话、授权和完成事务分开，先以 relay 保持兼容，再用策略、重试分类和 smoke 逐步验证直传候选 | [平台演进计划](../architecture/PLATFORM-EVOLUTION-PLAN.md)、[架构债务台账](../architecture/ARCHITECTURE-DEBT.md) |
| 渐进微服务 | 接口、契约、Shadow、独立入口和 embedded 回滚并存，降低一次性拆分风险 | [服务边界](../architecture/SERVICE-BOUNDARIES.md)、[微服务部署](../architecture/MICROSERVICES-DEPLOYMENT.md) |
| Agent 安全 | ExecutionContext 和 Capability policy 位于模型外层，Tool 与权限分离 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[MCP 授权](../agent/agent-mcp-authorization.md) |
| Agent 可恢复执行 | Temporal 保存任务状态，人工输入与审批可恢复；模型输出仍受审计和预算约束 | [Agent Runtime 设计](../architecture/AGENT-RUNTIME-DESIGN.md)、[Active 部署运行手册](../agent/AGENT-ACTIVE-DEPLOYMENT.md) |
| 设计到实现闭环 | Pencil canonical frame 定义只读 Timeline 的信息边界，Chromium snapshot 固定当前 Vue 页面，后续页面与跨浏览器基线独立推进 | [前端设计计划](../frontend/FRONTEND-DESIGN-PLAN.md)、[设计说明](../../design/README.md)、`frontend/e2e/agent-task-timeline.visual.spec.ts` |

## 6. 高频追问

### 为什么没有直接把每个模块拆成独立服务？

服务数量不会自动改善消息可靠性或数据所有权。Dipole 先用模块边界和接口隔离稳定语义，再将热点和独立运行需求明确的模块抽离。这样可以保留可测试的本地组合和 embedded 回滚路径，降低迁移期间的故障定位成本。

### Kafka 能否直接作为用户离线消息同步队列？

Kafka 用于内部事件传播和投影。用户同步需要面向用户与设备的长期域状态、稳定 cursor、权限重算和重连语义，因此由 Sync Timeline 承担。两者通过事件连接，消费组 offset 不承担客户端同步协议。

### 为什么 Message ID、Conversation Seq 和 Sync Seq 要分开？

全局 Message ID 用于唯一性和幂等；Conversation Seq 表达会话内顺序；Sync Seq 表达某个用户的增量消费顺序。三者的分区、排序和重试范围不同，混用会增加分页、已读、多端和迁移的复杂度。

### Agent 为什么选择 TypeScript？

Agent 主要是模型、工具、工作流与协议集成，TypeScript 对 Zod、JSON Schema、MCP、Node I/O 和 Temporal SDK 的支持较完整。Go 继续负责 IM 领域约束与一致性，语言职责按数据面、控制面和智能执行面拆分，避免模型 Runtime 直接接触业务存储。

### Agent active 为什么仍然默认关闭？

模型调用、工具权限和长期任务会引入成本、数据访问和副作用风险。当前 active profile 只允许只读 Temporal Activity，并要求 user-gray manifest、五类 Eval、Operator grant、共享 Kafka/Temporal/RPC/Provider 证据与维护窗口。缺少任一证据时保持 Shadow。

### Temporal 等待审批时，如何避免错误用户批准了错误任务？

Workflow 先通过 Core 创建持久 Approval，再进入 `waiting_approval`。Signal 必须同时匹配当前 request 和 approval ID；Activity 会把 Task、Run、Runtime、审批 ID、决策和经 Gateway 认证的 actor 交给 Core。Core 重读持久 Task/Run，要求 actor 等于 Task principal，并用条件更新收敛首个 approved/denied 决策。重放只接受完全相同的已决结果，参数漂移、过期、撤销或跨 Task 引用都会拒绝。写 Tool 仍需后续的 grant 解析与原子 consume，因此“收到 Signal”本身不授予副作用权限。

### 为什么从 GORM 迁移到 SQLC？

消息、同步和投影的关键路径需要明确 SQL、索引、锁语义和跨服务可复用的数据契约。SQLC 让查询、参数和结果类型在编译期绑定，便于审查 MySQL 事务边界、迁移版本和最小权限授权；Go 服务继续把领域规则放在 application 层。这个选择也让后续多语言服务能够共享 protobuf、SQL schema 与数据库所有权约束，而不依赖某个语言的 ORM 行为。

### 为什么预签名 Multipart 直传仍然默认关闭？

直传可以降低 Core 的大文件带宽与连接占用，但它还依赖对象存储 CORS 或同源代理、短期 URL 刷新、客户端断网恢复、限流、超时、生命周期清理与跨存储对账。当前 relay 路径已具备 MinIO Multipart、暂停恢复、session 续传和可观测性，预签名路径作为受版本化策略保护的候选实现保留。切换前需要同版本故障矩阵、隔离环境 smoke、责任人批准和可执行回切证据。

### 远程部署和压力测试怎样避免“本地能跑”的伪证据？

开发环境区分 Remote GPU 的完整拓扑和 TencentCloud 的低资源 Smoke。每次运行应绑定 Git revision、镜像或源码摘要、配置摘要和资源快照，先通过 migration、readiness、mTLS、Kafka lag 与健康检查，再记录 P50/P95/P99、错误率和资源水位。活动会话受到保护时只进行只读审计或本地契约测试，不能用静态渲染、单元测试或未获批准的共享环境操作替代运行时证据。

### C++ 实时数据面为什么留到后期？

当前 Go Delivery 仍是权威路径。C++ 候选聚焦连接、批量投递、背压和节点级 fanout 等数据面工作；它需要在稳定协议之上用相同流量、同一指标和自动回切策略证明收益。这样语言边界有明确性能动机，也避免为了技术栈展示而把 CRUD 领域拆到 C++。

### 如何证明前端设计稿没有停留在静态展示？

设计以 canonical Pencil frame、共享 `--dp-*` token 和受控 Playwright fixture 形成三层闭环。以 Agent Task Timeline 为例，Pencil 定义 desktop/mobile/state matrix 和低敏 metadata 边界；Vue 路由与组件测试固定状态机映射；Chromium visual test 再固定当前只读页面的 revision、Capability、分页入口及 raw event kind 不直接呈现。该基线不代表所有页面、所有浏览器或 active Agent 已完成验收，剩余范围继续由前端计划和 AD-044 管理。

更多网络、存储、性能、SQLC、MCP、C++ 与故障恢复问题见 [详细面试问答](INTERVIEW-QA.md)。

## 7. 学习路线

| 阶段 | 目标 | 建议练习 | 完成标志 |
| --- | --- | --- | --- |
| IM 基础 | 讲清消息、会话、未读、已读和 Sync Timeline | 手画数据流与 cursor 推进过程 | 能解释三个序列的边界 |
| 可靠性 | 掌握幂等、Outbox、重试、DLQ 和投影一致性 | 演示重复事件和发布失败如何收敛 | 能描述失败模式与回滚 |
| 微服务 | 理解服务边界、RPC、配置和数据所有权 | 用一条消息追踪 Gateway 到投影 | 能说明为何渐进拆分 |
| 性能 | 基于报告解释 bottleneck，避免孤立指标结论 | 比较普通群与热点群证据 | 能区分吞吐、延迟、写放大和 lag |
| Agent | 掌握 ExecutionContext、Tool policy、Memory、Temporal 和 Eval | 演示等待审批后恢复的状态机 | 能解释模型不能决定权限 |
| 复盘 | 诚实说明已验证、默认关闭与规划能力 | 每次提交更新本节与题库 | 对局限有明确下一步 |

## 8. 面试前检查

1. 从 [README](../../README.md) 复核当前架构与服务列表。
2. 从 [更新日志](../../CHANGELOG.md) 选择最近两个有测试证据的改动。
3. 从 [性能基线](../performance/PERFORMANCE-BASELINE.md) 选择一组结果，并说明其环境与局限。
4. 从 [架构债务台账](../architecture/ARCHITECTURE-DEBT.md) 选择一个未完成项，准备解释风险和计划。
5. 使用 60 秒和 3 分钟版本各练习一次，再从详细题库抽取 5 个追问。
6. 对照最近一次合并切片的测试记录，确认本文件的状态标签、证据链接、限制和复核条件未过期。
