# 更新日志

本文档记录当前版本线中面向用户和开发者的重要变化。详细历史保存在
[文档归档](docs/archive/CHANGELOG-2026-09.md)。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### 修复

- 修复 AD-008：已审批消息在写入后遇到审计异常或 Worker 中断时，Temporal 可复用原调用继续完成；迁移 `000055` 限制一份审批绑定一个调用，保持权限及参数复核。Core 返回持久化回执中的原消息 ID，避免 Kafka 重试的临时 ID 导致审计冲突。真实 DeepSeek/Temporal/MySQL 故障注入验证原 Task 完成、消息与调用各一条、模型未重新规划。

- 本地真实体验验证通过：群摘要使用 DeepSeek 实际检索和二次回答，审批后由 Temporal 按真实时间发布；等待期间重启 Agent 后仍完成原 Task。批准一条、拒绝零条、取消零条，重复批准不增发，WebSocket/Sync 可见。使用现有 smoke，未使用模型、Core 或存储替身。

- 真实体验验证发现天气工具加入后工具描述超过 Context 分区预算，现将 Capability 预算调整为 1200、相应减少 Evidence 分区，保持总上限 4096。新增迁移 `000054` 允许已审批群回复记录完成审计；不修改历史迁移。审批消费后的异常恢复缺口记录为 AD-008。

- Agent 完整镜像依赖安装与编译通过，解除先前下载阻塞；以非 root 禁网容器验证 Temporal/Runtime 模块加载。面试材料明确默认体验的 Memory、MCP 与单节点可用性边界。

- 真实 Agent Experience 全链路通过：私聊、群 @AI、Elasticsearch 检索后二次回答、自然语言审批拒绝/批准、Worker 重启与重复批准、历史和 Sync。统一使用现有体验栈和 `scripts/smoke-agent-experience.mjs`。
- 上下文优先保留最新历史，区分当前请求与历史指令；回答阶段移除重复计划策略，避免实际模型预算超限。Kafka 无 offset 分区从保留历史起读，复用 EventLedger 幂等。
- Elasticsearch 健康检查要求主分片可用；单节点 experience 使用 20/10/5GB 空闲水位。Agent 锁文件下载地址统一官方 npm registry，版本和 integrity 不变。

- Agent experience 明确依赖 Search 健康并开启 Gateway 搜索；修复仅设置新模型 Key 时仍要求旧 Key 的 Compose 插值错误，补齐镜像构建、证书和前端代理启动说明。

- Agent 在只读工具执行后将有界证据送入回答阶段；同一 Task 的计划与回答分别持久化恢复，已完成工具步骤复用保存结果。需应用数据库迁移 `000052`。

- 压测 summary 仅导出统计指标与阈值，排除登录 setup 数据；历史 benchmark JWT 已脱敏并更新对应校验和。

### 新增

- 原生只读 `get_weather` 工具：按城市查询 Open-Meteo 当前天气，返回地点、时间、单位及来源，复用 Agent 工具证据回答链路。固定请求地址、8 秒超时与 64KiB 响应上限；迁移 `000053` 为内置 Agent 添加天气读取权限，自定义策略保持不变。

- `/digest <带时区的 ISO 时间> <检索请求>` 复用现有工具/回答生成审批草稿，通过 Temporal 持久化等待定时发送，支持等待期间取消；群内 `@AI /digest ...` 经审批后回写原群。审批身份绑定正文、会话与时间，Core 发送前复核请求者当前群访问权限。局部测试及真实 Temporal Worker 更换测试通过；自然语言时间澄清和新路径完整体验验收仍待完成。

- 模型可根据当前私聊的自然语言请求提出系统消息，复用现有审批与 Message Command；批准恢复使用持久化提案，不再次调用模型。实际 Temporal Worker 更换测试覆盖拒绝、重复批准以及提交后响应丢失重试，Core 命令端使用测试替身。

- TypeScript Agent Runtime 已接入 Temporal 主路径，支持私聊 AI、群聊 `@AI`、
  会话搜索、人工审批和 Worker 重启恢复。
- 消息写入通过 Core 授权与稳定 invocation ID 保持幂等；拒绝审批不产生副作用。

### 变更

- 构建入口统一为 Makefile 与 justfile，删除两个重复 Docker 构建脚本；默认仅构建 7 个主链路程序，维护工具和旧压测镜像显式选择。镜像打包前重新调用 Go 构建缓存，基础系统依赖层与版本标签分离。

- README、项目介绍和面试问答改为产品链路导向的文档结构。
- 前端 V3 品牌资源与生产 bundle 已刷新。
- 本地规划、编辑器和 Codex 状态已由 `.gitignore` 排除。
