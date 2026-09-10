# 更新日志

本文档记录当前版本线中面向用户和开发者的重要变化。详细历史保存在
[文档归档](docs/archive/CHANGELOG-2026-09.md)。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [Unreleased]

### 新增

- TypeScript Agent Runtime 已接入 Temporal 主路径，支持私聊 AI、群聊 `@AI`、
  会话搜索、人工审批和 Worker 重启恢复。
- 消息写入通过 Core 授权与稳定 invocation ID 保持幂等；拒绝审批不产生副作用。

### 变更

- README、项目介绍和面试问答改为产品链路导向的文档结构。
- 前端 V3 品牌资源与生产 bundle 已刷新。
- 本地规划、编辑器和 Codex 状态已由 `.gitignore` 排除。
