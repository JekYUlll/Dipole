# Dipole 架构参考与改造方向

## 1. 文档目的

这份文档用于指导 `Dipole` 从当前的 demo 化骨架，逐步演进成一个**适合 IM 场景、又不过度设计**的 Go 项目。

参考对象有两个：

- `acc/KamaChat`：学习型项目，适合帮助我们快速理解 IM 业务主线
- `acc/im-server`：企业级实现，适合帮助我们校正系统边界、模块拆分和演进方向

我们的目标不是照搬任一项目，而是：

- 吸收 `KamaChat` 的业务完整性
- 吸收 `im-server` 的边界意识和模块设计
- 保持当前阶段仍然是**可快速推进的模块化单体**

---

## 2. 两个参考项目分别给我们的价值

### 2.1 KamaChat 提供的价值

`KamaChat` 更像一个“从业务视角组织的 IM 单体项目”，优点是链路很完整，容易上手：

- 用户：注册、登录、验证码登录、封禁、管理员
- 联系人：添加、删除、黑名单、申请/通过/拒绝
- 会话：单聊/群聊会话列表
- 消息：文本、文件、音视频数据
- WebSocket：在线收发消息
- Redis：缓存消息列表、验证码等
- Kafka：作为可选消息通道

值得借鉴的部分：

- 业务模块覆盖面完整
- 从 HTTP 到 service 到存储再到 WebSocket 的链路比较直观
- 数据模型对 IM 业务比较友好，例如 `UserInfo`、`Session`、`Message`

对应参考文件：

- `acc/KamaChat/internal/model/user_info.go`
- `acc/KamaChat/internal/model/session.go`
- `acc/KamaChat/internal/model/message.go`
- `acc/KamaChat/internal/service/gorm/user_info_service.go`
- `acc/KamaChat/internal/service/chat/server.go`
- `acc/KamaChat/internal/https_server/https_server.go`

### 2.2 im-server 提供的价值

`im-server` 更像一个“IM 核心平台”，不是普通业务项目。它强调的是边界、扩展性和部署演进：

- `connectmanager`：只负责连接与协议
- `usermanager`：只负责用户领域
- `message`：只负责消息处理与分发
- `conversation`：只负责会话
- `friendmanager`：只负责好友/关系
- `group`：只负责群
- `historymsg`：只负责历史消息
- `apigateway`：对外 HTTP API
- `navigator`：导航地址下发

值得借鉴的部分：

- **连接管理和业务管理解耦**
- **HTTP Gateway 与核心服务解耦**
- **消息、用户、会话、关系分别成域**
- **启动器统一初始化配置、数据库、服务注册**
- **存储层可切换**，例如 `message/storages/storage.go` 中按配置选择 MySQL/Mongo 实现

对应参考文件：

- `acc/im-server/launcher/main.go`
- `acc/im-server/services/connectmanager/server/imwebsocketserver.go`
- `acc/im-server/services/connectmanager/server/imlistener.go`
- `acc/im-server/services/usermanager/starter.go`
- `acc/im-server/services/usermanager/services/userservice.go`
- `acc/im-server/services/usermanager/storages/dbs/userdao.go`
- `acc/im-server/services/message/starter.go`
- `acc/im-server/services/message/services/msgservice.go`
- `acc/im-server/services/message/storages/storage.go`
- `acc/im-server/services/apigateway/routers/router.go`

---

## 3. 我们不应该直接照搬的部分

### 3.1 KamaChat 里不该直接照搬的部分

- 路由全部集中在一个文件里，扩展后会迅速失控
- controller 直接依赖全局 service 单例，模块边界较弱
- WebSocket 连接管理、消息落库、消息分发耦合较重
- 大量全局变量和 `init()` 初始化，不利于测试和替换
- HTTP 返回码设计较学习化，不适合长期扩展

### 3.2 im-server 里当前阶段不该照搬的部分

- 一开始就拆成大量服务
- actor system / cluster / rpc 总线
- protobuf 全链路协议
- 多种存储引擎并存
- 复杂的网关、导航、管理台体系

原因很明确：`Dipole` 目前还处于“把单体主链路做稳”的阶段，直接引入这些复杂度只会拖慢我们。

---

## 4. 对 Dipole 的核心判断

### 4.1 当前问题

当前 `Dipole` 已经有了：

- 配置加载
- HTTP 服务骨架
- MySQL/Redis 初始化
- demo 级 `user` 链路

但问题也很明显：

- `user` 模型还是偏 demo，不是 IM 用户模型
- 还没有 auth/register/login 主链路
- HTTP、业务、连接管理、消息模型之间还没有清晰边界
- 当前目录结构虽有分层，但**还不够模块化**

### 4.2 目标选择

当前阶段最合理的目标不是“做成 im-server 那样的微服务”，而是：

**做成一个模块化单体（modular monolith）的 IM 后端。**

也就是：

- 业务上按域拆分
- 部署上先保持单进程
- 存储上先用 MySQL + Redis
- WebSocket 保留为单独的连接层
- 后续若需要横向扩展，再把部分模块服务化

---

## 5. 推荐的目标架构

推荐逐步演进到如下结构：

```text
cmd/
  server/

internal/
  bootstrap/
    app.go
    http.go
    store.go

  platform/
    config/
    logger/
    database/
    cache/

  transport/
    http/
      middleware/
      response/
      routes/
    ws/
      hub/
      codec/
      session/

  modules/
    auth/
      domain/
      application/
      infrastructure/
      delivery/http/
    user/
      domain/
      application/
      infrastructure/
      delivery/http/
    contact/
    conversation/
    message/
    group/

docs/
```

### 5.1 为什么这样设计

这套结构综合了两个参考项目的优点：

- 从 `KamaChat` 学业务主线
- 从 `im-server` 学按领域拆模块
- 但不在当前阶段引入分布式 RPC 和 actor

### 5.2 当前项目与目标结构的映射

当前已有目录可以作为过渡：

- `internal/config` -> 未来并入 `platform/config`
- `internal/store` -> 未来拆到 `platform/database` 和 `platform/cache`
- `internal/handler/http` -> 未来进入 `modules/*/delivery/http`
- `internal/repository` -> 未来进入各模块 `infrastructure`
- `internal/service` -> 未来进入各模块 `application`

所以我们会沿着现有代码继续整理边界，不做推翻式重写。

---

## 6. 对用户域的直接参考结论

### 6.1 用户模型不应该再停留在 demo 级

我们当前的 `user` 需要从 demo 模型升级为 IM 用户模型。

建议第一阶段保留这些核心字段：

- `ID`
- `UUID`
- `Nickname`
- `Telephone`
- `Email`
- `Avatar`
- `Password`
- `Status`
- `IsAdmin`
- `CreatedAt`
- `UpdatedAt`

先不急着加：

- `Gender`
- `Birthday`
- `Signature`
- `LastOnlineAt`
- `LastOfflineAt`
- `DeletedAt`

这些可以在用户主链路跑稳之后再补。

### 6.2 UUID 和自增 ID 双轨并存

参考 `KamaChat` 和 `im-server`，我们应该明确：

- DB 主键用 `ID`
- 对外业务标识用 `UUID`

这样做的好处：

- 数据库存储和关联简单
- 外部 API 不暴露内部自增主键
- 后续会话、消息、联系人等模块都能统一以 `UUID` 为业务键

### 6.3 注册/登录应该成为下一个业务主线

下一步重点不应是继续扩 `GET /users/:id` 这种 demo 接口，而是补上：

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/:uuid`
- `PATCH /api/v1/users/:uuid/profile`

这是从 demo 化走向业务化的第一步。

---

## 7. 对消息与连接层的直接参考结论

### 7.1 三个参考项目里的 WebSocket 分别怎么做

`KamaChat` 的参考文件：

- `acc/KamaChat/api/v1/ws_controller.go`
- `acc/KamaChat/internal/service/chat/server.go`
- `acc/KamaChat/internal/service/chat/client.go`

它的特点是：

- HTTP 入口直接升级为 WebSocket 连接
- `ChatServer` 自己维护在线客户端映射和广播 channel
- 连接管理、消息解析、消息落库、在线转发集中在同一个聊天服务里

这套方案非常适合学习 IM 的最小闭环，因为链路短、阅读成本低、容易快速跑起来。

`im-server` 的参考文件：

- `acc/im-server/services/connectmanager/server/imwebsocketserver.go`
- `acc/im-server/services/connectmanager/server/imlistener.go`
- `acc/im-server/services/connectmanager/server/codec/*`

它的特点是：

- `connectmanager` 负责连接生命周期、协议和消息入口
- `listener` 负责把不同消息类型分派给后续业务处理
- 连接层和用户、消息、会话等业务域分层更明确

这套方案更像一个“连接管理中心”，很适合帮助我们建立边界意识。

`open-im-server` 的参考文件：

- `acc/open-im-server/internal/msggateway/ws_server.go`
- `acc/open-im-server/internal/msggateway/client.go`
- `acc/open-im-server/internal/msggateway/client_conn.go`
- `acc/open-im-server/internal/msggateway/user_map.go`
- `acc/open-im-server/internal/msggateway/message_handler.go`

它的特点是：

- 网关职责拆分更细
- 用户连接映射、多端管理、消息处理、压缩等能力都有独立对象
- 更适合多节点、多端、多协议演进

这套设计更成熟，也更重，当前阶段更适合作为远期边界参考。

### 7.2 我们采用的路线

`Dipole` 当前采用：

**`KamaChat` 的最小闭环 + `im-server` 的分层边界**

具体含义是：

- 学 `KamaChat`，先把单聊消息跑通
- 学 `im-server`，从第一版开始就把连接层和消息业务层分开
- 参考 `open-im-server`，提前知道后续多端、多节点、多协议会长成什么样

### 7.3 WebSocket 组件的职责划分

当前建议在单体内先形成下面这组职责：

- `transport/ws`：连接建立、鉴权、读写循环、心跳、断线清理
- `transport/ws/hub`：在线连接注册、注销、按用户路由连接
- `modules/message/application`：消息校验、发送用例、在线投递调用
- `modules/message/infrastructure`：消息持久化

第一版避免把太多能力塞进 WebSocket 层，先保证边界清楚：

- 连接层不直接操作复杂业务规则
- 消息层不直接感知底层连接实现细节
- 存储层不承担在线分发职责

### 7.4 当前阶段仍然不做微服务拆分

虽然 `im-server` 把 `connectmanager` 和 `message` 拆成独立服务是合理的，但 `Dipole` 当前不直接这么做。

当前最佳做法是：

- 先在单体内拆包
- 先把单聊 MVP 跑通
- 等接口和主链路稳定后，再考虑独立进程化

---

## 8. 我们接下来真正要做的改造顺序

### Phase 1：先把用户体系从 demo 改成 IM 体系

1. 重构当前 `user` 模型为 IM 用户模型
2. 新增 `auth` 模块
3. 完成注册、登录、查询资料、修改资料
4. 明确 DTO、entity、repository interface 的边界

### Phase 2：先拉起单聊消息与 WebSocket MVP

1. `transport/ws` 建连与鉴权
2. 在线连接管理与心跳
3. 文本消息发送、持久化、在线投递
4. 离线消息的最小回补能力

### Phase 3：补齐会话层

1. `conversation` 模型
2. 最近会话列表
3. 未读数与最后一条消息摘要
4. 单聊和群聊会话抽象

### Phase 4：补联系人链路

1. 联系人关系
2. 联系申请
3. 联系人列表
4. 黑名单能力

### Phase 5：做群与群消息

1. 群组
2. 群成员与角色
3. 群会话
4. 群消息

### Phase 6：接入 AI 能力

1. AI 助手账号体系
2. AI 会话与消息编排
3. 总结、辅助回复、内容治理等能力

### Phase 7：接入 Cgo 高性能模块

1. 为热点路径建立 benchmark
2. 选择 1-2 个高收益点做 Cgo 加速
3. 保留 pure Go 回退实现

### Phase 8：补齐工程化能力

1. 监控
2. 限流
3. 后台管理
4. 部署与压测

---

## 9. 当前阶段的明确技术决策

为了避免后续摇摆，先明确以下决策：

- 架构形态：**模块化单体**
- 核心存储：**MySQL + Redis**
- 用户业务键：**UUID**
- 对外接口风格：**REST + JSON**
- 长连接：**WebSocket**
- 当前不引入：`etcd`、actor system、服务注册发现、Mongo 多引擎

具体执行顺序、每阶段交付项与阶段完成后的重构任务，见 [development-roadmap.md](./development-roadmap.md)。

---

## 10. 一句话版本的改造原则

**KamaChat 让我们知道“做什么”，im-server 让我们知道“怎么拆”，Dipole 接下来要做的是：先用模块化单体把 IM 核心主链路做稳，再按需要演进，而不是一开始就把企业级复杂度搬进来。**
