# 更新日志

本文档记录当前版本线中面向用户和开发者的重要变化。详细历史保存在
[文档归档](docs/archive/CHANGELOG-2026-09.md)。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### 修复

- 真实 Agent Experience 全链路通过：私聊、群 @AI、Elasticsearch 检索后二次回答、自然语言审批拒绝/批准、Worker 重启与重复批准、历史和 Sync。统一使用现有体验栈和 `scripts/smoke-agent-experience.mjs`。
- 上下文优先保留最新历史，区分当前请求与历史指令；回答阶段移除重复计划策略，避免实际模型预算超限。Kafka 无 offset 分区从保留历史起读，复用 EventLedger 幂等。
- Elasticsearch 健康检查要求主分片可用；单节点 experience 使用 20/10/5GB 空闲水位。Agent 锁文件下载地址统一官方 npm registry，版本和 integrity 不变。

- Agent experience 明确依赖 Search 健康并开启 Gateway 搜索；修复仅设置新模型 Key 时仍要求旧 Key 的 Compose 插值错误，补齐镜像构建、证书和前端代理启动说明。

- Agent 在只读工具执行后将有界证据送入回答阶段；同一 Task 的计划与回答分别持久化恢复，已完成工具步骤复用保存结果。需应用数据库迁移 `000052`。

- 压测 summary 仅导出统计指标与阈值，排除登录 setup 数据；历史 benchmark JWT 已脱敏并更新对应校验和。

### 新增

- 模型可根据当前私聊的自然语言请求提出系统消息，复用现有审批与 Message Command；批准恢复使用持久化提案，不再次调用模型。实际 Temporal Worker 更换测试覆盖拒绝、重复批准以及提交后响应丢失重试，Core 命令端使用测试替身。

- TypeScript Agent Runtime 已接入 Temporal 主路径，支持私聊 AI、群聊 `@AI`、
  会话搜索、人工审批和 Worker 重启恢复。
- 消息写入通过 Core 授权与稳定 invocation ID 保持幂等；拒绝审批不产生副作用。

### 变更

- README、项目介绍和面试问答改为产品链路导向的文档结构。
- 前端 V3 品牌资源与生产 bundle 已刷新。
- 本地规划、编辑器和 Codex 状态已由 `.gitignore` 排除。
