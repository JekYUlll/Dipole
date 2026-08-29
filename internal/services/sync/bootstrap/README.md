# Sync Bootstrap

本目录是 Sync 服务的启动装配边界。runtime、数据库权限校验和相关测试已迁入本目录，直接组合 Sync application、projector、hydration 与平台能力；Internal RPC 暂通过窄 compatibility adapter 接入共享 transport。

Sync 入口必须通过本目录初始化；旧共享 runtime 路径已移除。RPC compatibility adapter 保留为下一阶段抽取平台 RPC 的回滚边界。
