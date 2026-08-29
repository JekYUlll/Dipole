# 参考项目

`acc/` 是本仓库的本地参考项目区，服务于架构学习和对照验证，不参与 Dipole 的构建、部署或运行时依赖。

参考仓库链接见 [`SOURCE-LINKS.md`](SOURCE-LINKS.md)。

| 项目 | 用途 |
| --- | --- |
| `acc/KamaChat` | 学习 IM 业务链路和基础领域模型 |
| `acc/im-server` | 对照成熟 IM 的服务边界与工程组织 |
| `acc/open-im-server` | 对照商业 IM 的消息、同步和服务拆分 |
| `acc/glide-im` | 对照 Go IM 的模块化实现 |
| `acc/gowebsocket` | 对照 WebSocket 连接和协议处理 |

参考项目保持独立 Go Module。修改 Dipole 时，应优先把结论沉淀到 `docs/architecture/architecture-reference.md`，避免业务代码直接依赖 `acc/`。
