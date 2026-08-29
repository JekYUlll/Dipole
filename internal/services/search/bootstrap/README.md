# Search Bootstrap

本目录是 Search 服务的启动装配边界。runtime 已迁入本目录，直接组合 Search application、Elasticsearch 和运行时平台能力；Internal RPC 暂通过本目录的窄兼容 adapter 接入共享 transport。

Search 入口必须通过本目录初始化运行时；旧共享 runtime 路径已移除。RPC compatibility adapter 保留为下一阶段抽取平台 RPC 的回滚边界。
