// Base user info (public)
export interface PublicUser {
  uuid: string
  nickname: string
  avatar: string
  user_type: number // 0=普通用户, 1=AI助手
  status: number
}

export interface PrivateUser extends PublicUser {
  telephone: string
  email: string
  is_admin: boolean
  created_at: string
}

export interface Contact {
  user: PublicUser
  remark: string
  status: number // 0=正常, 1=拉黑
}

export interface FriendApplication {
  id: number
  applicant: PublicUser
  target: PublicUser
  message: string
  status: number // 0=待处理, 1=已接受, 2=已拒绝
  created_at: string
}

export interface Group {
  uuid: string
  name: string
  notice: string
  avatar: string
  owner_uuid: string
  member_count: number
  members?: GroupMember[]
}

export interface GroupMember {
  user: PublicUser
  role: number // 0=成员, 1=管理员, 2=群主
  joined_at: string
}

export interface FileInfo {
  file_id: string
  file_name: string
  file_size: number
  download_path: string
  content_type: string
  file_expires_at?: string
}

export interface Message {
  id: number
  message_id: string
  from_uuid: string
  target_uuid: string
  target_type: number   // 0=单聊, 1=群聊
  message_type: number  // 0=文本, 1=文件, 2=AI回复, 3=系统消息
  content: string
  // REST API history format
  file?: FileInfo
  // WS flat format
  file_id?: string
  file_name?: string
  file_size?: number
  download_path?: string
  content_type?: string
  file_expires_at?: string
  sent_at: string
}

export interface LastMessage {
  message_id: string
  message_type: number
  preview: string
  sent_at: string
}

export interface Conversation {
  conversation_key: string
  target_type: number   // 0=单聊, 1=群聊
  target_user?: PublicUser
  target_group?: Group
  last_message: LastMessage
  unread_count: number
}

export interface Device {
  connection_id: string
  device: string
  device_id: string
  ip: string
  connected_at: string
}

// WS packet
export interface WsPacket {
  type: string
  data: Record<string, unknown>
}
