<template>
  <div class="im-container">
    <!-- Nav Bar -->
    <div class="nav-bar">
      <div class="nav-avatar">
        <img v-if="auth.currentUser?.avatar" :src="auth.currentUser.avatar" alt="me" />
        <span v-else>{{ getInitials(auth.currentUser?.nickname || '') }}</span>
      </div>
      <div class="nav-icons">
        <button class="icon-btn" :class="{ active: navTab === 'chat' }" @click="navTab = 'chat'" title="消息">💬</button>
        <button class="icon-btn" :class="{ active: navTab === 'contacts' }" @click="navTab = 'contacts'" title="联系人">👥</button>
        <button class="icon-btn" :class="{ active: navTab === 'groups' }" @click="navTab = 'groups'" title="群组">🏠</button>
      </div>
      <button class="icon-btn logout-btn" @click="handleLogout" title="退出">🚪</button>
    </div>

    <!-- Session Panel -->
    <div class="session-panel">
      <div class="search-wrap">
        <input v-model="searchText" type="text" placeholder="搜索" />
      </div>
      <div class="conv-list" ref="convListRef">
        <div
          v-for="conv in filteredConversations"
          :key="conv.conversation_key"
          class="conv-item"
          :class="{ active: chat.activeKey === conv.conversation_key }"
          @click="selectConversation(conv)"
        >
          <div class="conv-avatar">
            <img
              v-if="convAvatar(conv)"
              :src="convAvatar(conv)"
              :alt="convName(conv)"
            />
            <span v-else>{{ getInitials(convName(conv)) }}</span>
          </div>
          <div class="conv-body">
            <div class="conv-top">
              <span class="conv-name">{{ convName(conv) }}</span>
              <span class="conv-time">{{ conv.last_message ? formatTime(conv.last_message.sent_at) : '' }}</span>
            </div>
            <div class="conv-bottom">
              <span class="conv-preview">{{ conv.last_message?.preview || '' }}</span>
              <span v-if="conv.unread_count > 0" class="conv-badge">{{ conv.unread_count }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Chat Area -->
    <div class="chat-area">
      <template v-if="activeConv">
        <div class="chat-header">
          <span>{{ activeConvName }}</span>
          <button class="detail-toggle" @click="showDetail = !showDetail" title="详情">ℹ️</button>
        </div>

        <div class="msg-list" ref="msgListRef">
          <button class="load-more-btn" @click="loadMore">加载更多</button>
          <div
            v-for="msg in currentMessages"
            :key="msg.message_id"
            :class="msgItemClass(msg)"
          >
            <!-- System message -->
            <template v-if="msg.message_type === 3">
              <div class="msg-system">{{ msg.content }}</div>
            </template>

            <!-- Regular / AI message -->
            <template v-else>
              <div class="msg-avatar">
                <img
                  v-if="msgAvatar(msg)"
                  :src="msgAvatar(msg)"
                  :alt="msg.from_uuid"
                />
                <div v-else class="msg-avatar-fallback">{{ getInitials(msgSenderName(msg)) }}</div>
              </div>
              <div class="msg-bubble">
                <!-- File card -->
                <template v-if="msg.message_type === 1">
                  <div class="file-card" @click="downloadFile(msg)">
                    <span class="file-icon">📄</span>
                    <div class="file-meta">
                      <div class="name">{{ msg.file?.file_name || '文件' }}</div>
                      <div class="size">{{ msg.file?.file_size ? formatSize(msg.file.file_size) : '' }}</div>
                    </div>
                  </div>
                </template>
                <!-- Text / AI -->
                <template v-else>
                  {{ msg.content }}
                </template>
              </div>
            </template>
          </div>
        </div>

        <div class="input-area">
          <div class="input-toolbar">
            <label class="tool-btn" title="发送文件">
              📎
              <input type="file" style="display:none" @change="uploadFile" />
            </label>
          </div>
          <textarea
            v-model="inputText"
            placeholder="输入消息..."
            @keydown.enter.exact.prevent="sendMessage"
            @keydown.enter.shift.exact="inputText += '\n'"
          />
          <div class="send-row">
            <button class="send-btn" @click="sendMessage">发送</button>
          </div>
        </div>
      </template>

      <div v-else class="empty-chat">
        选择一个会话开始聊天
      </div>
    </div>

    <!-- Detail Panel -->
    <div v-if="activeConv && showDetail" class="detail-panel">
      <template v-if="activeConv.target_type === 1 && activeConv.target_group">
        <div class="detail-avatar">
          <img v-if="activeConv.target_group.avatar" :src="activeConv.target_group.avatar" alt="group" />
          <div v-else class="detail-avatar-fallback">{{ getInitials(activeConv.target_group.name) }}</div>
        </div>
        <div class="detail-name">{{ activeConv.target_group.name }}</div>
        <div class="detail-uuid">{{ activeConv.target_group.uuid }}</div>
        <div class="detail-meta">成员数: {{ activeConv.target_group.member_count ?? '—' }}</div>
      </template>
      <template v-else-if="activeConv.target_user">
        <div class="detail-avatar">
          <img v-if="activeConv.target_user.avatar" :src="activeConv.target_user.avatar" alt="user" />
          <div v-else class="detail-avatar-fallback">{{ getInitials(activeConv.target_user.nickname) }}</div>
        </div>
        <div class="detail-name">{{ activeConv.target_user.nickname }}</div>
        <div class="detail-uuid">{{ activeConv.target_user.uuid }}</div>
        <div class="detail-meta">状态: {{ activeConv.target_user.status ?? '未知' }}</div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import { useWebSocket } from '@/composables/useWebSocket'
import type { Conversation, Message, WsPacket } from '@/types'
import api from '@/api'

const router = useRouter()
const auth = useAuthStore()
const chat = useChatStore()

const navTab = ref<'chat' | 'contacts' | 'groups'>('chat')
const searchText = ref('')
const inputText = ref('')
const showDetail = ref(false)
const msgListRef = ref<HTMLDivElement | null>(null)
const convListRef = ref<HTMLDivElement | null>(null)

// ── Helpers ──────────────────────────────────────────────────────────────────

const getInitials = (name: string) => name ? name[0].toUpperCase() : '?'

const formatTime = (t: string) => {
  const d = new Date(t)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
  }
  return `${d.getMonth() + 1}/${d.getDate()}`
}

const formatSize = (bytes: number) => {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

const deriveMessageKey = (msg: Message, myUUID: string): string => {
  if (msg.target_type === 1) return `group:${msg.target_uuid}`
  const peer = msg.from_uuid === myUUID ? msg.target_uuid : msg.from_uuid
  return `direct:${[myUUID, peer].sort().join(':')}`
}

const wsDataToMessage = (data: Record<string, unknown>): Message => ({
  id: 0,
  message_id: data.message_id as string,
  from_uuid: data.from_uuid as string,
  target_uuid: data.target_uuid as string,
  target_type: data.target_type as number,
  message_type: data.message_type as number,
  content: (data.content as string) || '',
  file: data.file as Message['file'],
  sent_at: data.sent_at as string,
})

const pushIncomingMessage = (msg: Message) => {
  const key = deriveMessageKey(msg, auth.currentUser!.uuid)
  const list = chat.messageMap.get(key) || []
  if (!list.some(m => m.message_id === msg.message_id)) {
    list.push(msg)
    list.sort((a, b) => new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime())
    chat.messageMap.set(key, list)
  }
}

const scrollToBottom = () => {
  if (msgListRef.value) msgListRef.value.scrollTop = msgListRef.value.scrollHeight
}

// ── Computed ──────────────────────────────────────────────────────────────────

const activeConv = computed(() =>
  chat.conversations.find(c => c.conversation_key === chat.activeKey) ?? null
)

const activeConvName = computed(() => {
  const conv = activeConv.value
  if (!conv) return ''
  if (conv.target_type === 1) return conv.target_group?.name ?? '群组'
  return conv.target_user?.nickname ?? '用户'
})

const currentMessages = computed(() =>
  chat.messageMap.get(chat.activeKey) ?? []
)

const filteredConversations = computed(() => {
  if (!searchText.value.trim()) return chat.conversations
  const q = searchText.value.toLowerCase()
  return chat.conversations.filter(c => {
    const name = c.target_type === 1 ? c.target_group?.name : c.target_user?.nickname
    return name?.toLowerCase().includes(q)
  })
})

// ── Conv helpers ──────────────────────────────────────────────────────────────

const convName = (conv: Conversation) => {
  if (conv.target_type === 1) return conv.target_group?.name ?? '群组'
  return conv.target_user?.nickname ?? '用户'
}

const convAvatar = (conv: Conversation) => {
  if (conv.target_type === 1) return conv.target_group?.avatar ?? ''
  return conv.target_user?.avatar ?? ''
}

// ── Message helpers ───────────────────────────────────────────────────────────

const msgItemClass = (msg: Message) => {
  if (msg.message_type === 3) return 'msg-item system'
  if (msg.message_type === 2) return 'msg-item ai'
  if (msg.from_uuid === auth.currentUser?.uuid) return 'msg-item self'
  return 'msg-item other'
}

const msgAvatar = (msg: Message): string => {
  if (msg.from_uuid === auth.currentUser?.uuid) return auth.currentUser?.avatar ?? ''
  const conv = activeConv.value
  if (!conv) return ''
  if (conv.target_type === 0) return conv.target_user?.avatar ?? ''
  // group: look up member avatar from contacts or fallback
  return ''
}

const msgSenderName = (msg: Message): string => {
  if (msg.from_uuid === auth.currentUser?.uuid) return auth.currentUser?.nickname ?? ''
  const conv = activeConv.value
  if (conv?.target_type === 0) return conv.target_user?.nickname ?? msg.from_uuid
  return msg.from_uuid
}

// ── Actions ───────────────────────────────────────────────────────────────────

const selectConversation = async (conv: Conversation) => {
  chat.activeKey = conv.conversation_key
  showDetail.value = false
  if (conv.target_type === 1 && conv.target_group) {
    await chat.fetchGroupMessages(conv.target_group.uuid)
  } else if (conv.target_user) {
    await chat.fetchDirectMessages(conv.target_user.uuid)
    await chat.markRead(conv)
  }
  scrollToBottom()
}

const sendMessage = () => {
  if (!inputText.value.trim() || !activeConv.value) return
  const conv = activeConv.value
  const targetUUID = conv.target_type === 1 ? conv.target_group!.uuid : conv.target_user!.uuid
  ws.send('chat.send', { target_uuid: targetUUID, content: inputText.value.trim() })
  inputText.value = ''
}

const uploadFile = async (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file || !activeConv.value) return
  const conv = activeConv.value
  const targetUUID = conv.target_type === 1 ? conv.target_group!.uuid : conv.target_user!.uuid
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post('/api/v1/files', formData) as { file_id: string }
  ws.send('chat.send_file', { target_uuid: targetUUID, file_id: res.file_id })
  ;(e.target as HTMLInputElement).value = ''
}

const downloadFile = async (msg: Message) => {
  const fileId = msg.file?.file_id || (msg as any).file_id
  if (!fileId) return
  const res = await api.get(`/api/v1/files/${fileId}/download`) as { download_url: string }
  window.open(res.download_url, '_blank', 'noopener,noreferrer')
}

const loadMore = async () => {
  if (!activeConv.value) return
  const msgs = currentMessages.value
  const oldest = msgs[0]
  const beforeID = oldest?.id || 0
  if (activeConv.value.target_type === 1) {
    await chat.fetchGroupMessages(activeConv.value.target_group!.uuid, beforeID)
  } else {
    await chat.fetchDirectMessages(activeConv.value.target_user!.uuid, beforeID)
  }
}

const handleLogout = async () => {
  await auth.logout()
  router.push({ name: 'login' })
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

const handleWsPacket = async (packet: WsPacket) => {
  const { type, data } = packet
  switch (type) {
    case 'connected':
      await Promise.allSettled([chat.fetchConversations(), chat.fetchDevices()])
      break
    case 'chat.message':
    case 'chat.sent': {
      const msg = wsDataToMessage(data as Record<string, unknown>)
      pushIncomingMessage(msg)
      await chat.fetchConversations()
      const key = deriveMessageKey(msg, auth.currentUser!.uuid)
      if (key === chat.activeKey) scrollToBottom()
      break
    }
    case 'session.kicked':
      alert(`被踢下线: ${(data as any)?.reason || ''}`)
      await auth.logout()
      router.push({ name: 'login' })
      break
    case 'group.created':
    case 'group.updated':
    case 'group.members_added':
    case 'group.members_removed':
    case 'group.dismissed':
      await chat.fetchConversations()
      break
  }
}

const ws = useWebSocket({ onMessage: handleWsPacket })

// ── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(async () => {
  if (!auth.token) return
  await auth.fetchMe()
  await Promise.allSettled([chat.fetchConversations(), chat.fetchContacts()])
  ws.connect(auth.token)
})

watch(currentMessages, () => nextTick(scrollToBottom))
</script>

<style scoped>
.im-container {
  display: flex;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
}

/* Nav Bar */
.nav-bar {
  width: 48px;
  background: #1a1a2e;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0;
  flex-shrink: 0;
}

.nav-avatar {
  width: 34px;
  height: 34px;
  border-radius: 4px;
  overflow: hidden;
  background: #555;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 14px;
  font-weight: bold;
  flex-shrink: 0;
}

.nav-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.nav-icons {
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex: 1;
  margin-top: 20px;
  align-items: center;
}

.icon-btn {
  background: none;
  border: none;
  font-size: 22px;
  cursor: pointer;
  opacity: 0.5;
  color: white;
  padding: 0;
  line-height: 1;
  transition: opacity 0.15s;
}

.icon-btn:hover,
.icon-btn.active {
  opacity: 1;
}

.logout-btn {
  margin-top: auto;
}

/* Session Panel */
.session-panel {
  width: 260px;
  background: #ededed;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.search-wrap {
  padding: 8px 10px;
  background: #e0e0e0;
}

.search-wrap input {
  width: 100%;
  padding: 5px 10px;
  border-radius: 14px;
  border: none;
  background: #d4d4d4;
  font-size: 13px;
  outline: none;
}

.conv-list {
  flex: 1;
  overflow-y: auto;
}

.conv-item {
  display: flex;
  padding: 10px 12px;
  cursor: pointer;
  gap: 10px;
  border-bottom: 1px solid #d8d8d8;
  align-items: center;
}

.conv-item:hover {
  background: #d8d8d8;
}

.conv-item.active {
  background: #c8c8c8;
}

.conv-avatar {
  width: 42px;
  height: 42px;
  border-radius: 4px;
  flex-shrink: 0;
  overflow: hidden;
  background: #bbb;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 16px;
  font-weight: bold;
}

.conv-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.conv-body {
  flex: 1;
  min-width: 0;
}

.conv-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 3px;
}

.conv-name {
  font-size: 14px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-time {
  font-size: 11px;
  color: #999;
  flex-shrink: 0;
}

.conv-bottom {
  display: flex;
  align-items: center;
}

.conv-preview {
  font-size: 12px;
  color: #999;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.conv-badge {
  background: #f00;
  color: #fff;
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 10px;
  margin-left: auto;
  flex-shrink: 0;
}

/* Chat Area */
.chat-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #f5f5f5;
  min-width: 0;
}

.chat-header {
  height: 52px;
  background: #f5f5f5;
  border-bottom: 1px solid #e0e0e0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  font-size: 16px;
  font-weight: 600;
  flex-shrink: 0;
}

.detail-toggle {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 18px;
  opacity: 0.5;
  transition: opacity 0.15s;
}

.detail-toggle:hover {
  opacity: 1;
}

.msg-list {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.load-more-btn {
  align-self: center;
  background: none;
  border: 1px solid #ddd;
  border-radius: 12px;
  padding: 3px 14px;
  font-size: 12px;
  color: #999;
  cursor: pointer;
  margin-bottom: 4px;
}

.load-more-btn:hover {
  background: #eee;
}

.msg-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.msg-item.self {
  flex-direction: row-reverse;
}

.msg-item.system {
  justify-content: center;
}

.msg-avatar img,
.msg-avatar-fallback {
  width: 36px;
  height: 36px;
  border-radius: 4px;
  flex-shrink: 0;
  object-fit: cover;
}

.msg-avatar-fallback {
  background: #bbb;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 14px;
  font-weight: bold;
}

.msg-bubble {
  max-width: 60%;
  padding: 9px 13px;
  border-radius: 4px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-all;
}

.msg-item.other .msg-bubble {
  background: #fff;
  border: 1px solid #e8e8e8;
}

.msg-item.self .msg-bubble {
  background: #95ec69;
}

.msg-item.ai .msg-bubble {
  background: #e8d5ff;
  border: 1px solid #d0b0ff;
}

.msg-system {
  text-align: center;
  color: #aaa;
  font-size: 12px;
  padding: 4px 0;
}

.file-card {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 0;
}

.file-icon {
  font-size: 28px;
}

.file-meta .name {
  font-weight: 500;
  font-size: 13px;
}

.file-meta .size {
  font-size: 11px;
  color: #888;
}

/* Input Area */
.input-area {
  background: #fff;
  border-top: 1px solid #e0e0e0;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}

.input-toolbar {
  height: 34px;
  display: flex;
  align-items: center;
  padding: 0 12px;
  gap: 8px;
  border-bottom: 1px solid #f0f0f0;
}

.tool-btn {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 18px;
  opacity: 0.6;
  transition: opacity 0.15s;
}

.tool-btn:hover {
  opacity: 1;
}

.input-area textarea {
  flex: 1;
  border: none;
  outline: none;
  padding: 10px 14px;
  resize: none;
  font-size: 14px;
  font-family: inherit;
  background: #fff;
  min-height: 60px;
}

.send-row {
  height: 38px;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding: 0 14px;
}

.send-btn {
  padding: 5px 16px;
  background: #07c160;
  color: #fff;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}

.send-btn:hover {
  background: #06ad56;
}

/* Empty state */
.empty-chat {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  color: #ccc;
  font-size: 18px;
}

/* Detail Panel */
.detail-panel {
  width: 240px;
  background: #f7f7f7;
  border-left: 1px solid #e0e0e0;
  padding: 20px 16px;
  overflow-y: auto;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.detail-avatar {
  width: 64px;
  height: 64px;
  border-radius: 6px;
  overflow: hidden;
  background: #bbb;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 4px;
}

.detail-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-avatar-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 24px;
  font-weight: bold;
}

.detail-name {
  font-size: 15px;
  font-weight: 600;
  text-align: center;
}

.detail-uuid {
  font-size: 11px;
  color: #aaa;
  word-break: break-all;
  text-align: center;
}

.detail-meta {
  font-size: 13px;
  color: #666;
}
</style>
