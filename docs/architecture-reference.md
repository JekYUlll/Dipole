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

所以我们**不是推翻重来**，而是沿着现有代码继续整理边界。

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

### 7.1 WebSocket 不应和用户/消息业务强耦合

`KamaChat` 目前的 WebSocket 层承担了：

- 在线连接管理
- 消息解析
- 消息落库
- 单聊/群聊分发
- Redis 缓存更新

这对学习很友好，但对长期维护不友好。

更合理的方式是借鉴 `im-server`：

- 连接层负责连接、会话、协议、心跳
- 消息应用层负责校验和路由
- 存储层负责持久化
- 推送/分发层负责在线投递

也就是说，我们未来应该逐步形成：

- `transport/ws`：连接和协议
- `modules/message/application`：消息用例
- `modules/message/infrastructure`：消息仓储

### 7.2 当前阶段仍然不做微服务拆分

虽然 `im-server` 把 `connectmanager` 和 `message` 拆成独立服务是合理的，但 `Dipole` 当前不应直接这么做。

当前最佳做法是：

- 先在单体内拆包
- 等接口和主链路稳定后，再考虑独立进程化

---

## 8. 我们接下来真正要做的改造顺序

### Phase 1：先把用户体系从 demo 改成 IM 体系

1. 重构当前 `user` 模型为 IM 用户模型
2. 新增 `auth` 模块
3. 完成注册、登录、查询资料、修改资料
4. 明确 DTO、entity、repository interface 的边界

### Phase 2：把会话和消息模块拉起来

1. `conversation` 模型
2. `message` 模型
3. 发送消息的 HTTP/应用层
4. WebSocket 建连与在线会话管理
5. 消息持久化和在线投递

### Phase 3：再做联系人和群

1. 联系人关系
2. 联系申请
3. 群组与成员
4. 群消息

### Phase 4：最后再做增强能力

1. CGo 高性能模块
2. AI/Agent 能力
3. 监控、限流、后台管理

---

## 9. 当前阶段的明确技术决策

为了避免后续摇摆，先明确以下决策：

- 架构形态：**模块化单体**
- 核心存储：**MySQL + Redis**
- 用户业务键：**UUID**
- 对外接口风格：**REST + JSON**
- 长连接：**WebSocket**
- 当前不引入：`etcd`、actor system、服务注册发现、Mongo 多引擎

---

## 10. 一句话版本的改造原则

**KamaChat 让我们知道“做什么”，im-server 让我们知道“怎么拆”，Dipole 接下来要做的是：先用模块化单体把 IM 核心主链路做稳，再按需要演进，而不是一开始就把企业级复杂度搬进来。**
