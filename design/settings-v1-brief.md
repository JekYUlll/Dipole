# Dipole Settings v1

为 Dipole 设计一个认证后的 Settings 页面，作为现代 IM 完整流程的一部分。请在现有 canonical 设计中只新增一个小范围 `Settings / Desktop`、`Settings / Mobile` 和状态说明区域，不要修改已有 Chat、Search、Sync 或 Agent 页面。

页面只呈现已经有权威接口支持的能力：当前账户资料、可编辑的个人签名、设备会话安全入口、客户端同步状态说明和退出登录。资料保存复用当前用户 profile API；设备管理复用现有 device session API。不要设计通知、主题、隐私或其他没有后端持久化接口的可写开关。

视觉沿用 Dipole 现有浅色工作区、深色信息层级、清晰的风险分级和紧凑导航。桌面按内容分区，移动端单列；“退出所有其他设备”和“退出登录”属于危险操作，应有明确范围说明和二次确认状态。设计应展示 loading、保存成功、设备列表为空和接口不可用的有界状态。
