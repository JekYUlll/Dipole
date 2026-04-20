<template>
  <div class="im-container">
    <!-- Left Panel (nav + session) -->
    <div class="left-panel" :class="{ hidden: !!chat.activeKey }">
      <!-- Nav Bar -->
    <div class="nav-bar">
      <button class="nav-avatar profile-btn" @click="openSelfProfile" title="个人资料">
        <img v-if="auth.currentUser?.avatar" :src="auth.currentUser.avatar" alt="me" />
        <span v-else>{{ getInitials(auth.currentUser?.nickname || '') }}</span>
      </button>
      <div class="nav-icons">
        <button class="icon-btn" :class="{ active: navTab === 'chat' }" @click="navTab = 'chat'" title="消息">💬</button>
        <button class="icon-btn contacts-btn" :class="{ active: navTab === 'contacts' }" @click="switchToContacts" title="联系人">
          👥
          <span v-if="pendingApplications.length > 0" class="nav-badge">{{ pendingApplications.length }}</span>
        </button>
        <button class="icon-btn" :class="{ active: navTab === 'groups' }" @click="navTab = 'groups'" title="群组">🏠</button>
      </div>
      <button class="icon-btn logout-btn" @click="handleLogout" title="退出">🚪</button>
    </div>

    <!-- Session Panel -->
    <div class="session-panel">
      <div class="search-wrap">
        <input v-model="searchText" type="text" placeholder="搜索" />
      </div>

      <!-- 消息列表 -->
      <div v-if="navTab === 'chat'" class="panel-list">
        <div
          v-for="conv in filteredConversations"
          :key="conv.conversation_key"
          class="conv-item"
          :class="{ active: chat.activeKey === conv.conversation_key }"
          @click="selectConversation(conv)"
        >
          <div class="conv-avatar" :class="{ 'conv-avatar-group': conv.target_type === 1 && !convAvatar(conv) }">
            <img v-if="convAvatar(conv)" :src="convAvatar(conv)" :alt="convName(conv)" />
            <span v-else-if="conv.target_type === 1" class="group-icon">👥</span>
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

      <!-- 联系人列表 -->
      <div v-else-if="navTab === 'contacts'" class="panel-list">
        <div class="panel-actions">
          <button class="panel-action-btn" @click="showAddFriend = true">+ 添加好友</button>
        </div>
        <div
          v-for="c in filteredContacts"
          :key="c.user.uuid"
          class="conv-item"
          @click="openDirectChat(c)"
        >
          <div class="conv-avatar">
            <img v-if="c.user.avatar" :src="c.user.avatar" :alt="c.user.nickname" />
            <span v-else>{{ getInitials(c.user.nickname) }}</span>
          </div>
          <div class="conv-body">
            <div class="conv-top">
              <span class="conv-name">{{ c.remark ? `${c.remark}（${c.user.nickname}）` : c.user.nickname }}</span>
            </div>
            <div class="conv-bottom">
              <span class="conv-preview">{{ c.user.nickname }}</span>
            </div>
          </div>
        </div>

        <!-- 好友申请 -->
        <template v-if="pendingApplications.length > 0">
          <div class="section-title">好友申请 ({{ pendingApplications.length }})</div>
          <div v-for="app in pendingApplications" :key="app.id" class="app-item">
            <div class="conv-avatar small">
              <img v-if="app.applicant.avatar" :src="app.applicant.avatar" />
              <span v-else>{{ getInitials(app.applicant.nickname) }}</span>
            </div>
            <div style="flex:1;min-width:0">
              <div style="font-weight:500;font-size:13px">{{ app.applicant.nickname }}</div>
              <div style="font-size:11px;color:#999">{{ app.message || '请求添加好友' }}</div>
            </div>
            <div class="app-actions">
              <button @click.stop="handleApplication(app.id, 'accept')">接受</button>
              <button @click.stop="handleApplication(app.id, 'reject')">拒绝</button>
            </div>
          </div>
        </template>
      </div>

      <!-- 群组列表 -->
      <div v-else-if="navTab === 'groups'" class="panel-list">
        <div class="panel-actions">
          <button class="panel-action-btn" @click="showCreateGroup = true">+ 创建群组</button>
        </div>
        <div
          v-for="conv in groupConversations"
          :key="conv.conversation_key"
          class="conv-item"
          :class="{ active: chat.activeKey === conv.conversation_key }"
          @click="selectConversation(conv)"
        >
          <div class="conv-avatar" :class="{ 'conv-avatar-group': conv.target_type === 1 && !convAvatar(conv) }">
            <img v-if="convAvatar(conv)" :src="convAvatar(conv)" :alt="convName(conv)" />
            <span v-else-if="conv.target_type === 1" class="group-icon">👥</span>
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
    </div><!-- /session-panel -->
    </div><!-- /left-panel -->

    <!-- Chat Area -->
    <div class="chat-area">
      <template v-if="activeConv">
        <div class="chat-header">
          <button class="back-btn" @click="chat.activeKey = ''">‹</button>
          <span class="chat-header-title">{{ activeConvName }}</span>
          <span v-if="isGroupDismissed" class="status-chip danger">已解散</span>
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
              <div class="msg-sender-name clickable-name"
                v-if="activeConv?.target_type === 1 && msg.from_uuid !== auth.currentUser?.uuid"
                @click.stop="openMessageUserProfile(msg)"
              >{{ msgSenderName(msg) }}</div>
              <div class="msg-row">
                <div class="msg-avatar clickable" @click.stop="openMessageUserProfile(msg)">
                  <img
                    v-if="msgAvatar(msg)"
                    :src="msgAvatar(msg)"
                    :alt="msg.from_uuid"
                  />
                  <div v-else class="msg-avatar-fallback">{{ getInitials(msgSenderName(msg)) }}</div>
                </div>
                <div :class="['msg-bubble', isInlineMediaMessage(msg) ? 'media' : '']">
                  <!-- File card -->
                  <template v-if="msg.message_type === 1">
                    <div v-if="isImageMessage(msg)" class="media-card" :data-msg-id="msg.message_id">
                      <img
                        v-if="mediaPreviewSrc(msg)"
                        class="media-image"
                        :src="mediaPreviewSrc(msg)"
                        :alt="fileName(msg)"
                        @click="downloadFile(msg)"
                      />
                      <div v-else class="media-placeholder">图片</div>
                      <div class="media-caption">
                        <span class="media-name">{{ fileName(msg) }}</span>
                        <button class="file-action-btn" @click.stop="downloadFile(msg)">⬇ 下载</button>
                      </div>
                    </div>
                    <div v-else-if="isVideoMessage(msg)" class="media-card" :data-msg-id="msg.message_id">
                      <video
                        v-if="mediaPreviewSrc(msg)"
                        class="media-video"
                        :src="mediaPreviewSrc(msg)"
                        controls
                        preload="metadata"
                      ></video>
                      <div v-else class="media-placeholder">视频</div>
                      <div class="media-caption">
                        <span class="media-name">{{ fileName(msg) }}</span>
                        <button class="file-action-btn" @click.stop="downloadFile(msg)">⬇ 下载</button>
                      </div>
                    </div>
                    <div v-else class="file-card" @click="downloadFile(msg)">
                      <span class="file-icon">{{ fileIcon(msg) }}</span>
                      <div class="file-meta">
                        <div class="name">{{ fileName(msg) }}</div>
                        <div class="size">{{ formatSize(fileSize(msg)) }}</div>
                      </div>
                    </div>
                  </template>
                  <!-- Text / AI -->
                  <template v-else>
                    {{ msg.content }}
                  </template>
                </div>
              </div>
            </template>
          </div>
        </div>

        <div class="input-area">
          <div class="input-toolbar">
            <label class="tool-btn" :class="{ disabled: isGroupDismissed }" :title="isGroupDismissed ? '群已解散，无法发送文件' : '发送文件'">
              📎
              <input type="file" style="display:none" :disabled="isGroupDismissed" @change="uploadFile" />
            </label>
            <span v-if="uploadingFileLabel" class="upload-status">{{ uploadingFileLabel }}</span>
          </div>
          <textarea
            v-model="inputText"
            :disabled="isGroupDismissed"
            :placeholder="isGroupDismissed ? '群已解散，仅可查看历史消息' : '输入消息...'"
            @keydown.enter.exact.prevent="sendMessage"
            @keydown.enter.shift.exact="inputText += '\n'"
          />
          <div class="send-row">
            <button class="send-btn" :disabled="isGroupDismissed" @click="sendMessage">发送</button>
          </div>
        </div>
      </template>

      <div v-else class="empty-chat">
        选择一个会话开始聊天
      </div>
    </div>

    <!-- Detail Panel -->
    <div v-if="activeConv && showDetail" class="detail-panel">
      <template v-if="activeConv.target_type === 1">
        <!-- 群头像 + 基本信息 -->
        <div class="detail-header">
          <div class="detail-avatar">
            <img v-if="groupFromConv(activeConv)?.avatar" :src="groupFromConv(activeConv)!.avatar" alt="group" />
            <div v-else class="detail-avatar-fallback group-avatar-fallback">👥</div>
          </div>
          <div class="detail-name">{{ convName(activeConv) }}</div>
          <div class="detail-uuid">{{ activeConv.conversation_key.replace('group:', '') }}</div>
          <div class="detail-chips">
            <span class="detail-chip">{{ groupFromConv(activeConv)?.member_count ?? '—' }} 人</span>
            <span class="detail-chip" :class="{ danger: isGroupDismissed }">{{ isGroupDismissed ? '已解散' : '正常' }}</span>
          </div>
        </div>

        <!-- 群主编辑区 -->
        <template v-if="isGroupOwner">
          <div class="detail-section-title">群资料</div>
          <div class="detail-field">
            <label>群名称</label>
            <div class="detail-field-row">
              <input v-model="groupNameDraft" class="detail-edit-input" :disabled="isGroupDismissed" placeholder="群名称" maxlength="50" />
              <button class="detail-inline-btn" :disabled="isGroupDismissed" @click="saveGroupProfile">保存</button>
            </div>
          </div>
          <div class="detail-field">
            <label>群头像</label>
            <input ref="groupAvatarInputRef" type="file" accept="image/*" style="display:none" @change="handleGroupAvatarSelected" />
            <div class="detail-field-row">
              <span class="detail-file-hint">{{ selectedGroupAvatarName || '未选择文件' }}</span>
              <button class="detail-inline-btn secondary" :disabled="isGroupDismissed" @click="chooseGroupAvatar">选择</button>
              <button class="detail-inline-btn" :disabled="isGroupDismissed || !selectedGroupAvatarFile || uploadingGroupAvatar" @click="uploadGroupAvatar">
                {{ uploadingGroupAvatar ? '…' : '上传' }}
              </button>
            </div>
          </div>
        </template>

        <!-- 备注（所有人可设置） -->
        <div class="detail-section-title">我的备注</div>
        <div class="detail-field">
          <div class="detail-field-row">
            <input v-model="groupRemarkDraft" class="detail-edit-input" placeholder="设置当前群显示名称" maxlength="50" />
            <button class="detail-inline-btn" @click="saveGroupRemark">保存</button>
          </div>
        </div>

        <!-- 成员 -->
        <div class="detail-section-title">群成员</div>
        <div class="member-grid">
          <div
            v-for="m in [...(groupFromConv(activeConv)?.members ?? [])].sort((a, b) => a.role - b.role)"
            :key="m.user.uuid"
            class="member-grid-item"
            :title="m.user.nickname + (m.role === 0 ? '（群主）' : '')"
            @click="openUserProfile(m.user)"
          >
            <div class="member-grid-avatar">
              <img v-if="m.user.avatar" :src="m.user.avatar" />
              <span v-else>{{ getInitials(m.user.nickname) }}</span>
            </div>
            <div class="member-grid-name">{{ m.user.nickname }}</div>
            <div v-if="m.role === 0" class="member-role-badge">主</div>
          </div>
          <div v-if="isGroupOwner && !isGroupDismissed" class="member-grid-item" @click="openInviteMembers">
            <div class="member-grid-avatar member-grid-add">+</div>
            <div class="member-grid-name">邀请</div>
          </div>
        </div>

        <!-- 危险操作 -->
        <div v-if="isGroupOwner && !isGroupDismissed" class="detail-danger-zone">
          <button class="danger-btn" @click="dismissGroup">解散群</button>
        </div>
      </template>

      <template v-else-if="activeConv.target_user">
        <div class="detail-header">
          <div class="detail-avatar">
            <img v-if="activeConv.target_user.avatar" :src="activeConv.target_user.avatar" alt="user" @click="openUserProfile(activeConv.target_user)" />
            <div v-else class="detail-avatar-fallback" @click="openUserProfile(activeConv.target_user)">{{ getInitials(activeConv.target_user.nickname) }}</div>
          </div>
          <div class="detail-name">{{ convName(activeConv) }}</div>
          <div class="detail-uuid">{{ activeConv.target_user.uuid }}</div>
          <div v-if="activeConv.target_user.signature" class="detail-meta">{{ activeConv.target_user.signature }}</div>
        </div>
      </template>
    </div>
    <!-- Profile Modal -->
    <div v-if="showProfileModal" class="modal-overlay">
      <div class="modal-backdrop" @click="closeProfileModal"></div>
      <div class="modal user-profile-card">
        <button class="upc-close" @click="closeProfileModal">✕</button>
        <div class="upc-avatar" style="cursor:pointer" @click="avatarInputRef?.click()">
          <img v-if="auth.currentUser?.avatar" :src="auth.currentUser.avatar" alt="profile-avatar" />
          <span v-else>{{ getInitials(auth.currentUser?.nickname || '') }}</span>
          <div class="upc-avatar-hint">换头像</div>
        </div>
        <input ref="avatarInputRef" type="file" accept="image/*" style="display:none" @change="handleAvatarSelected" />
        <div class="upc-name">{{ auth.currentUser?.nickname }}</div>
        <div class="upc-uuid">{{ auth.currentUser?.email || auth.currentUser?.telephone }}</div>
        <textarea v-model="profileSignature" class="upc-signature-input" placeholder="写点个性签名" maxlength="255"></textarea>
        <div v-if="selectedAvatarName" class="upc-file-hint">已选择：{{ selectedAvatarName }}</div>
        <div class="upc-actions">
          <button class="upc-btn" :disabled="!selectedAvatarFile || uploadingAvatar" @click="uploadAvatar">
            {{ uploadingAvatar ? '上传中…' : '上传头像' }}
          </button>
          <button class="upc-btn primary" :disabled="savingProfile" @click="saveProfile">
            {{ savingProfile ? '保存中…' : '保存资料' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showUserProfileModal && viewedUser" class="modal-overlay">
      <div class="modal-backdrop" @click="closeUserProfileModal"></div>
      <div class="modal user-profile-card">
        <button class="upc-close" @click="closeUserProfileModal">✕</button>
        <div class="upc-avatar">
          <img v-if="viewedUser.avatar" :src="viewedUser.avatar" alt="user-profile-avatar" />
          <span v-else>{{ getInitials(viewedUser.nickname) }}</span>
        </div>
        <div class="upc-name">{{ displayUserName(viewedUser) }}</div>
        <div class="upc-uuid">{{ viewedUser.uuid }}</div>
        <div v-if="viewedUser.signature" class="upc-signature">{{ viewedUser.signature }}</div>

        <div v-if="isFriend(viewedUser.uuid)" class="upc-remark">
          <input v-model="viewedUserRemark" class="upc-remark-input" placeholder="设置备注" maxlength="50" />
          <button class="upc-remark-btn" :disabled="savingUserRemark" @click="saveUserRemark">
            {{ savingUserRemark ? '…' : '保存备注' }}
          </button>
        </div>

        <div class="upc-actions">
          <button class="upc-btn primary" @click="startDirectChatFromViewedUser">发起单聊</button>
          <button v-if="!isFriend(viewedUser.uuid) && viewedUser.uuid !== auth.currentUser?.uuid" class="upc-btn" @click="quickApplyFriend(viewedUser)">加好友</button>
        </div>
      </div>
    </div>

    <!-- Add Friend Modal -->
    <div v-if="showAddFriend" class="modal-overlay" @click.self="closeAddFriend">
      <div class="modal">
        <div class="modal-title">添加好友</div>
        <input v-model="addFriendKeyword" placeholder="搜索用户名" @keydown.enter="searchUsers" />
        <button class="modal-btn" @click="searchUsers">搜索</button>
        <div v-if="searchResults.length" class="search-results">
          <div v-for="u in searchResults" :key="u.uuid" class="search-result-item">
            <div class="conv-avatar small">
              <img v-if="u.avatar" :src="u.avatar" />
              <span v-else>{{ getInitials(u.nickname) }}</span>
            </div>
            <div style="flex:1;min-width:0">
              <div style="font-size:13px;font-weight:500">{{ u.nickname }}</div>
              <div style="font-size:11px;color:#999">{{ u.uuid }}</div>
            </div>
            <button class="modal-btn small" @click="selectFriendTarget(u)">选择</button>
          </div>
        </div>
        <div v-else-if="addFriendSearched" style="font-size:13px;color:#999;text-align:center;padding:12px">未找到用户</div>
        <template v-if="friendTarget">
          <div style="font-size:12px;color:#888;margin:4px 0 2px">向 <b>{{ friendTarget.nickname }}</b> 发送申请</div>
          <input v-model="friendRequestMsg" placeholder="附言（可选）" maxlength="255" />
          <button class="modal-btn" @click="sendFriendRequest">发送申请</button>
        </template>
        <button class="modal-close" @click="closeAddFriend">关闭</button>
      </div>
    </div>

    <!-- Create Group Modal -->
    <div v-if="showCreateGroup" class="modal-overlay">
      <div class="modal-backdrop" @click="showCreateGroup = false"></div>
      <div class="modal">
        <div class="modal-title">创建群组</div>
        <input v-model="newGroupName" placeholder="群组名称（必填）" />
        <div style="font-size:12px;color:#888;margin:2px 0">选择成员（可选）</div>
        <div class="modal-body">
          <div class="member-select-list">
            <div v-if="chat.contacts.length === 0" style="font-size:12px;color:#aaa;padding:8px">暂无联系人</div>
            <label v-for="c in chat.contacts" :key="c.user.uuid" class="member-select-item">
              <input type="checkbox" :value="c.user.uuid" v-model="selectedMembers" />
              <div class="conv-avatar small">
                <img v-if="c.user.avatar" :src="c.user.avatar" />
                <span v-else>{{ getInitials(c.user.nickname) }}</span>
              </div>
              <span style="font-size:13px">{{ c.remark || c.user.nickname }}</span>
            </label>
          </div>
        </div>
        <div class="modal-footer">
          <button class="modal-btn" :disabled="!newGroupName.trim()" @click="createGroup">创建</button>
          <button class="modal-close" @click="showCreateGroup = false">关闭</button>
        </div>
      </div>
    </div>

    <!-- Invite Members Modal -->
    <div v-if="showInviteMembers" class="modal-overlay">
      <div class="modal-backdrop" @click="showInviteMembers = false"></div>
      <div class="modal">
        <div class="modal-title">邀请成员</div>
        <div style="font-size:12px;color:#888;margin:0 0 4px">从联系人中选择</div>
        <div class="modal-body">
          <div class="member-select-list">
            <div v-if="chat.contacts.length === 0" style="font-size:12px;color:#aaa;padding:8px">暂无联系人</div>
            <label v-for="c in chat.contacts" :key="c.user.uuid" class="member-select-item">
              <input type="checkbox" :value="c.user.uuid" v-model="inviteSelected" />
              <div class="conv-avatar small">
                <img v-if="c.user.avatar" :src="c.user.avatar" />
                <span v-else>{{ getInitials(c.user.nickname) }}</span>
              </div>
              <span style="font-size:13px">{{ c.remark || c.user.nickname }}</span>
            </label>
          </div>
        </div>
        <div class="modal-footer">
          <button class="modal-btn" :disabled="inviteSelected.length === 0" @click="inviteMembers">邀请</button>
          <button class="modal-close" @click="showInviteMembers = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import { useWebSocket } from '@/composables/useWebSocket'
import type { Conversation, Contact, GroupMessageNotify, Message, WsPacket, PublicUser } from '@/types'
import api from '@/api'

const router = useRouter()
const auth = useAuthStore()
const chat = useChatStore()

const navTab = ref<'chat' | 'contacts' | 'groups'>('chat')
const searchText = ref('')
const inputText = ref('')
const showDetail = ref(false)
const msgListRef = ref<HTMLDivElement | null>(null)
const showProfileModal = ref(false)
const avatarInputRef = ref<HTMLInputElement | null>(null)
const selectedAvatarFile = ref<File | null>(null)
const selectedAvatarName = ref('')
const uploadingAvatar = ref(false)
const profileSignature = ref('')
const savingProfile = ref(false)
const showUserProfileModal = ref(false)
const viewedUser = ref<PublicUser | null>(null)
const viewedUserRemark = ref('')
const savingUserRemark = ref(false)
const groupRemarkDraft = ref('')
const groupNameDraft = ref('')
const groupAvatarInputRef = ref<HTMLInputElement | null>(null)
const selectedGroupAvatarFile = ref<File | null>(null)
const selectedGroupAvatarName = ref('')
const uploadingGroupAvatar = ref(false)
const uploadingFileLabel = ref('')
const mediaPreviewMap = ref<Record<string, string>>({})
const mediaPreviewInflight = new Map<string, Promise<void>>()

const directUploadThresholdBytes = 4 * 1024 * 1024

// ── Helpers ──────────────────────────────────────────────────────────────────

const getInitials = (name: string) => name ? name[0].toUpperCase() : '?'
const contactOf = (uuid?: string | null) => chat.contacts.find(c => c.user.uuid === uuid)
const isFriend = (uuid?: string | null) => Boolean(contactOf(uuid))

const displayUserName = (user?: PublicUser | null) => {
  if (!user) return ''
  const remark = contactOf(user.uuid)?.remark
  if (remark) return `${remark}（${user.nickname}）`
  return user.nickname
}

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

const fileName = (msg: Message) => msg.file?.file_name || msg.file_name || '文件'
const fileSize = (msg: Message) => msg.file?.file_size || msg.file_size || 0
const fileContentType = (msg: Message) => (msg.file?.content_type || msg.content_type || '').toLowerCase()
const fileContentPath = (msg: Message) => msg.file?.content_path || msg.content_path || ''

const isImageMessage = (msg: Message) => fileContentType(msg).startsWith('image/')
const isVideoMessage = (msg: Message) => fileContentType(msg).startsWith('video/')
const isInlineMediaMessage = (msg: Message) => isImageMessage(msg) || isVideoMessage(msg)
const mediaPreviewSrc = (msg: Message) => mediaPreviewMap.value[msg.message_id] || ''

const fileIcon = (msg: Message) => {
  const contentType = fileContentType(msg)
  if (contentType.startsWith('image/')) return '🖼️'
  if (contentType.startsWith('video/')) return '🎬'
  if (contentType.startsWith('audio/')) return '🎵'
  if (contentType.includes('pdf')) return '📕'
  if (contentType.includes('zip') || contentType.includes('compressed') || contentType.includes('rar')) return '🗜️'
  if (contentType.includes('word') || contentType.includes('document')) return '📘'
  if (contentType.includes('excel') || contentType.includes('spreadsheet') || contentType.includes('sheet')) return '📗'
  if (contentType.includes('powerpoint') || contentType.includes('presentation')) return '📙'
  if (contentType.startsWith('text/')) return '📄'
  return '📦'
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

const latestLoadedMessageID = (key: string) => {
  const list = chat.messageMap.get(key) || []
  return list.reduce((max, item) => Math.max(max, item.id || 0), 0)
}

const pullHotGroupMessages = async (groupUUID: string) => {
  const key = `group:${groupUUID}`
  const existing = chat.messageMap.get(key) || []
  if (existing.length === 0) {
    await chat.fetchGroupMessages(groupUUID)
    return
  }

  const afterID = latestLoadedMessageID(key)
  if (afterID <= 0) {
    await chat.fetchGroupMessages(groupUUID)
    return
  }
  await chat.fetchGroupMessagesAfter(groupUUID, afterID)
}

const revokeMediaPreviewURLs = () => {
  Object.values(mediaPreviewMap.value).forEach((url) => URL.revokeObjectURL(url))
  mediaPreviewMap.value = {}
  mediaPreviewInflight.clear()
}

const loadMediaPreview = async (msg: Message) => {
  if (!isInlineMediaMessage(msg)) return
  if (mediaPreviewMap.value[msg.message_id]) return
  if (mediaPreviewInflight.has(msg.message_id)) {
    await mediaPreviewInflight.get(msg.message_id)
    return
  }

  const contentPath = fileContentPath(msg)
  const token = auth.token
  if (!contentPath || !token) return

  const request = (async () => {
    const response = await fetch(contentPath, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })
    if (!response.ok) {
      throw new Error(`media preview request failed: ${response.status}`)
    }
    const blob = await response.blob()
    const objectURL = URL.createObjectURL(blob)
    mediaPreviewMap.value = {
      ...mediaPreviewMap.value,
      [msg.message_id]: objectURL,
    }
  })()

  mediaPreviewInflight.set(msg.message_id, request)
  try {
    await request
  } catch (error) {
    console.warn('load media preview failed', error)
  } finally {
    mediaPreviewInflight.delete(msg.message_id)
  }
}

// ── Lazy media loading via IntersectionObserver ───────────────────────────────

let mediaObserver: IntersectionObserver | null = null

const setupMediaObserver = () => {
  if (mediaObserver) mediaObserver.disconnect()
  mediaObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue
        const el = entry.target as HTMLElement
        const msgId = el.dataset.msgId
        if (!msgId) continue
        const msg = currentMessages.value.find(m => m.message_id === msgId)
        if (msg) void loadMediaPreview(msg)
        mediaObserver?.unobserve(el)
      }
    },
    { root: msgListRef.value, rootMargin: '120px' }
  )
}

const observeMediaCards = () => {
  if (!msgListRef.value || !mediaObserver) return
  const cards = msgListRef.value.querySelectorAll<HTMLElement>('.media-card[data-msg-id]')
  cards.forEach(el => {
    const msgId = el.dataset.msgId
    if (msgId && !mediaPreviewMap.value[msgId]) {
      mediaObserver!.observe(el)
    }
  })
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
  return convName(conv)
})

const isGroupOwner = computed(() => {
  const conv = activeConv.value
  if (!conv || conv.target_type !== 1) return false
  const group = groupFromConv(conv)
  if (!group) return false
  // me_role 0 = owner（与后端 GroupMemberRoleOwner=0 对齐）
  if (group.me_role === 0) return true
  return group.owner?.uuid === auth.currentUser?.uuid
})

const isGroupDismissed = computed(() => {
  const conv = activeConv.value
  if (!conv || conv.target_type !== 1) return false
  return groupFromConv(conv)?.status === 1
})

const currentMessages = computed(() =>
  chat.messageMap.get(chat.activeKey) ?? []
)

const filteredConversations = computed(() => {
  if (!searchText.value.trim()) return chat.conversations
  const q = searchText.value.toLowerCase()
  return chat.conversations.filter(c => convName(c).toLowerCase().includes(q))
})

const groupConversations = computed(() =>
  chat.conversations.filter(c => c.target_type === 1)
)

const filteredContacts = computed(() => {
  if (!searchText.value.trim()) return chat.contacts
  const q = searchText.value.toLowerCase()
  return chat.contacts.filter(c =>
    c.user.nickname.toLowerCase().includes(q) ||
    (c.remark && c.remark.toLowerCase().includes(q))
  )
})

const pendingApplications = computed(() =>
  chat.applications.incoming.filter(a => a.status === 0)
)

// ── Conv helpers ──────────────────────────────────────────────────────────────

const groupFromConv = (conv: Conversation) => {
  if (conv.target_type !== 1) return null
  const uuid = conv.conversation_key.replace('group:', '')
  return conv.target_group ?? chat.groups.get(uuid) ?? null
}

const convName = (conv: Conversation) => {
  if (conv.target_type === 1) {
    const name = groupFromConv(conv)?.name || '群组'
    return conv.remark ? `${conv.remark}（${name}）` : name
  }
  const nickname = conv.target_user?.nickname || '用户'
  const remark = contactOf(conv.target_user?.uuid)?.remark
  return remark ? `${remark}（${nickname}）` : nickname
}

const convAvatar = (conv: Conversation) => {
  if (conv.target_type === 1) return groupFromConv(conv)?.avatar ?? ''
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
  // 群聊：从成员列表查找
  const group = groupFromConv(conv)
  return group?.members?.find(m => m.user.uuid === msg.from_uuid)?.user.avatar ?? ''
}

const msgSenderName = (msg: Message): string => {
  if (msg.from_uuid === auth.currentUser?.uuid) return auth.currentUser?.nickname ?? ''
  const conv = activeConv.value
  if (conv?.target_type === 0) return conv.target_user?.nickname ?? msg.from_uuid
  // 群聊：从成员列表查找
  const group = groupFromConv(conv!)
  const user = group?.members?.find(m => m.user.uuid === msg.from_uuid)?.user
  if (!user) return msg.from_uuid
  return displayUserName(user)
}

// ── Actions ───────────────────────────────────────────────────────────────────

const selectConversation = async (conv: Conversation) => {
  chat.activeKey = conv.conversation_key
  showDetail.value = false
  if (conv.target_type === 1) {
    const groupUUID = conv.target_group?.uuid ?? conv.conversation_key.replace('group:', '')
    await chat.fetchGroup(groupUUID).catch(() => {})
    await chat.fetchGroupMessages(groupUUID)
  } else if (conv.target_user) {
    await chat.fetchDirectMessages(conv.target_user.uuid)
    await chat.markRead(conv)
  }
  nextTick(scrollToBottom)
}

const sendMessage = () => {
  if (!inputText.value.trim() || !activeConv.value) return
  if (isGroupDismissed.value) return
  const conv = activeConv.value
  if (conv.target_type === 1) {
    const groupUUID = conv.target_group?.uuid ?? conv.conversation_key.replace('group:', '')
    ws.send('chat.send', { target_uuid: groupUUID, target_type: 1, content: inputText.value.trim() })
  } else {
    ws.send('chat.send', { target_uuid: conv.target_user!.uuid, target_type: 0, content: inputText.value.trim() })
  }
  inputText.value = ''
}

const uploadFile = async (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file || !activeConv.value) return
  if (isGroupDismissed.value) return
  const conv = activeConv.value
  const targetUUID = conv.target_type === 1
    ? (conv.target_group?.uuid ?? conv.conversation_key.replace('group:', ''))
    : conv.target_user!.uuid
  try {
    const res = await uploadChatFile(file)
    ws.send('chat.send_file', { target_uuid: targetUUID, target_type: conv.target_type, file_id: res.file_id })
  } catch (err: any) {
    alert(err?.message || '文件上传失败')
  } finally {
    uploadingFileLabel.value = ''
    ;(e.target as HTMLInputElement).value = ''
  }
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
  // WS-pushed messages have id=0; only use messages with real DB ids
  const oldest = msgs.find(m => m.id > 0)
  if (!oldest) return
  const beforeID = oldest.id
  if (activeConv.value.target_type === 1) {
    const groupUUID = activeConv.value.target_group?.uuid ?? activeConv.value.conversation_key.replace('group:', '')
    await chat.fetchGroupMessages(groupUUID, beforeID)
  } else {
    await chat.fetchDirectMessages(activeConv.value.target_user!.uuid, beforeID)
  }
}

const handleLogout = async () => {
  ws.close()
  await auth.logout()
  router.push({ name: 'login' })
}

const closeProfileModal = () => {
  showProfileModal.value = false
  selectedAvatarFile.value = null
  selectedAvatarName.value = ''
  if (avatarInputRef.value) avatarInputRef.value.value = ''
}

const openSelfProfile = () => {
  profileSignature.value = auth.currentUser?.signature || ''
  showProfileModal.value = true
}

const handleAvatarSelected = (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0] ?? null
  selectedAvatarFile.value = file
  selectedAvatarName.value = file?.name ?? ''
}

const uploadAvatar = async () => {
  if (!auth.currentUser || !selectedAvatarFile.value) return

  const formData = new FormData()
  formData.append('avatar', selectedAvatarFile.value)
  uploadingAvatar.value = true
  try {
    await api.post(`/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/avatar`, formData)
    await auth.fetchMe()
    closeProfileModal()
  } catch (e: any) {
    alert(e?.message || '头像上传失败')
  } finally {
    uploadingAvatar.value = false
  }
}

const saveProfile = async () => {
  if (!auth.currentUser) return
  savingProfile.value = true
  try {
    await api.patch(`/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile`, {
      signature: profileSignature.value.trim(),
    })
    await auth.fetchMe()
    profileSignature.value = auth.currentUser?.signature || ''
  } catch (e: any) {
    alert(e?.message || '资料保存失败')
  } finally {
    savingProfile.value = false
  }
}

const uploadChatFile = async (file: File): Promise<{ file_id: string }> => {
  if (file.size <= directUploadThresholdBytes) {
    uploadingFileLabel.value = '上传中...'
    const formData = new FormData()
    formData.append('file', file)
    return await api.post('/api/v1/files', formData) as { file_id: string }
  }

  const init = await api.post('/api/v1/files/uploads/initiate', {
    file_name: file.name,
    file_size: file.size,
    content_type: file.type || 'application/octet-stream',
  }) as { session_id: string; chunk_size: number; total_parts: number }

  try {
    for (let partNumber = 1; partNumber <= init.total_parts; partNumber += 1) {
      const start = (partNumber - 1) * init.chunk_size
      const end = Math.min(start + init.chunk_size, file.size)
      const chunk = file.slice(start, end)
      uploadingFileLabel.value = `上传中 ${partNumber}/${init.total_parts}...`
      await api.put(`/api/v1/files/uploads/${encodeURIComponent(init.session_id)}/parts/${partNumber}`, chunk, {
        headers: {
          'Content-Type': 'application/octet-stream',
        },
      })
    }
    uploadingFileLabel.value = '合并文件中...'
    return await api.post(`/api/v1/files/uploads/${encodeURIComponent(init.session_id)}/complete`) as { file_id: string }
  } catch (err) {
    try {
      await api.delete(`/api/v1/files/uploads/${encodeURIComponent(init.session_id)}`)
    } catch {
      // 上传失败时尽力清理服务端会话，避免 Redis 和 MinIO 留下无主分片。
    }
    throw err
  }
}

const switchToContacts = async () => {
  navTab.value = 'contacts'
  await chat.fetchApplications()
}

const openDirectChat = async (c: Contact) => {
  await openDirectChatByUser(c.user)
}

const openDirectChatByUser = async (user: PublicUser) => {
  const myUUID = auth.currentUser!.uuid
  const peerUUID = user.uuid
  const key = `direct:${[myUUID, peerUUID].sort().join(':')}`
  // Ensure a conversation entry exists so activeConv resolves
  if (!chat.conversations.find(conv => conv.conversation_key === key)) {
    chat.conversations.unshift({
      conversation_key: key,
      target_type: 0,
      target_user: user,
      remark: contactOf(user.uuid)?.remark || '',
      last_message: { message_id: '', message_type: 0, preview: '', sent_at: '' },
      unread_count: 0,
    })
  }
  chat.activeKey = key
  navTab.value = 'chat'
  try {
    await chat.fetchDirectMessages(peerUUID)
  } catch {
    // keep the chat shell open; message permission is validated on send
  }
  nextTick(scrollToBottom)
}

const openUserProfile = async (user: PublicUser) => {
  if (user.uuid === auth.currentUser?.uuid) {
    openSelfProfile()
    return
  }
  try {
    const detail = await api.get(`/api/v1/users/${encodeURIComponent(user.uuid)}`) as PublicUser
    viewedUser.value = detail
    viewedUserRemark.value = contactOf(detail.uuid)?.remark || ''
    showUserProfileModal.value = true
  } catch (e: any) {
    alert(e?.message || '获取用户资料失败')
  }
}

const openMessageUserProfile = async (msg: Message) => {
  if (!activeConv.value) return
  if (msg.from_uuid === auth.currentUser?.uuid) {
    openSelfProfile()
    return
  }
  if (activeConv.value.target_type === 0 && activeConv.value.target_user) {
    await openUserProfile(activeConv.value.target_user)
    return
  }
  const group = groupFromConv(activeConv.value)
  const user = group?.members?.find(m => m.user.uuid === msg.from_uuid)?.user
  if (user) await openUserProfile(user)
}

const closeUserProfileModal = () => {
  showUserProfileModal.value = false
  viewedUser.value = null
  viewedUserRemark.value = ''
}

const startDirectChatFromViewedUser = async () => {
  if (!viewedUser.value) return
  const user = viewedUser.value
  closeUserProfileModal()
  await openDirectChatByUser(user)
}

const saveUserRemark = async () => {
  if (!viewedUser.value) return
  savingUserRemark.value = true
  try {
    await api.patch(`/api/v1/contacts/${encodeURIComponent(viewedUser.value.uuid)}/remark`, {
      remark: viewedUserRemark.value.trim(),
    })
    await Promise.allSettled([chat.fetchContacts(), chat.fetchConversations()])
    if (viewedUser.value) {
      viewedUser.value = { ...viewedUser.value }
    }
  } catch (e: any) {
    alert(e?.message || '备注保存失败')
  } finally {
    savingUserRemark.value = false
  }
}

const quickApplyFriend = async (user: PublicUser) => {
  try {
    await api.post('/api/v1/contacts/applications', { target_uuid: user.uuid, message: '' })
    alert('好友申请已发送')
  } catch (e: any) {
    alert(e?.message || '发送好友申请失败')
  }
}

const handleApplication = async (id: number, action: 'accept' | 'reject') => {
  await api.patch(`/api/v1/contacts/applications/${id}`, { action })
  await Promise.allSettled([chat.fetchApplications(), chat.fetchContacts(), chat.fetchConversations()])
}

// ── Add Friend ────────────────────────────────────────────────────────────────

const showAddFriend = ref(false)
const addFriendKeyword = ref('')
const addFriendSearched = ref(false)
const searchResults = ref<PublicUser[]>([])
const friendTarget = ref<PublicUser | null>(null)
const friendRequestMsg = ref('')

const closeAddFriend = () => {
  showAddFriend.value = false
  addFriendKeyword.value = ''
  addFriendSearched.value = false
  searchResults.value = []
  friendTarget.value = null
  friendRequestMsg.value = ''
}

const searchUsers = async () => {
  if (!addFriendKeyword.value.trim()) return
  friendTarget.value = null
  const data = await api.get(`/api/v1/users?keyword=${encodeURIComponent(addFriendKeyword.value.trim())}&limit=20`) as PublicUser[]
  searchResults.value = Array.isArray(data) ? data.filter(u => u.uuid !== auth.currentUser?.uuid) : []
  addFriendSearched.value = true
}

const selectFriendTarget = (u: PublicUser) => {
  friendTarget.value = u
}

const sendFriendRequest = async () => {
  if (!friendTarget.value) return
  try {
    await api.post('/api/v1/contacts/applications', { target_uuid: friendTarget.value.uuid, message: friendRequestMsg.value.trim() })
    alert('好友申请已发送')
    closeAddFriend()
  } catch (e: any) {
    alert(e?.message || '发送失败')
  }
}

// ── Create Group ──────────────────────────────────────────────────────────────

const showCreateGroup = ref(false)
const newGroupName = ref('')
const selectedMembers = ref<string[]>([])

const createGroup = async () => {
  console.log('[createGroup] called, name=', newGroupName.value, 'members=', selectedMembers.value)
  if (!newGroupName.value.trim()) { console.log('[createGroup] empty name, abort'); return }
  try {
    console.log('[createGroup] posting...')
    const res = await api.post('/api/v1/groups', { name: newGroupName.value.trim(), member_uuids: selectedMembers.value })
    console.log('[createGroup] success, res=', res)
    newGroupName.value = ''
    selectedMembers.value = []
    showCreateGroup.value = false
    navTab.value = 'groups'
    // Kafka processes group.created asynchronously — poll a few times as fallback
    setTimeout(() => chat.fetchConversations(), 800)
    setTimeout(() => chat.fetchConversations(), 2500)
    setTimeout(() => chat.fetchConversations(), 5000)
  } catch (e: any) {
    console.error('[createGroup] error:', e)
    alert(e?.message || String(e) || '创建失败')
  }
}

// ── Invite Members ────────────────────────────────────────────────────────────

const showInviteMembers = ref(false)
const inviteSelected = ref<string[]>([])

const openInviteMembers = () => {
  inviteSelected.value = []
  showInviteMembers.value = true
}

const inviteMembers = async () => {
  if (!activeConv.value || inviteSelected.value.length === 0) return
  if (isGroupDismissed.value) return
  const groupUUID = activeConv.value.conversation_key.replace('group:', '')
  try {
    await api.post(`/api/v1/groups/${encodeURIComponent(groupUUID)}/members`, { member_uuids: inviteSelected.value })
    inviteSelected.value = []
    showInviteMembers.value = false
    await chat.fetchGroup(groupUUID)
  } catch (e: any) {
    alert(e?.message || '邀请失败')
  }
}

const saveGroupProfile = async () => {
  if (!activeConv.value || activeConv.value.target_type !== 1 || !isGroupOwner.value || isGroupDismissed.value) return
  const groupUUID = activeConv.value.conversation_key.replace('group:', '')
  try {
    await api.post(`/api/v1/groups/${encodeURIComponent(groupUUID)}/update`, {
      name: groupNameDraft.value.trim(),
    })
    await Promise.allSettled([chat.fetchConversations(), chat.fetchGroup(groupUUID)])
  } catch (e: any) {
    alert(e?.message || '群资料保存失败')
  }
}

const chooseGroupAvatar = () => {
  groupAvatarInputRef.value?.click()
}

const handleGroupAvatarSelected = (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0] ?? null
  selectedGroupAvatarFile.value = file
  selectedGroupAvatarName.value = file?.name ?? ''
}

const uploadGroupAvatar = async () => {
  if (!activeConv.value || activeConv.value.target_type !== 1 || !selectedGroupAvatarFile.value || isGroupDismissed.value) return
  const groupUUID = activeConv.value.conversation_key.replace('group:', '')
  const formData = new FormData()
  formData.append('avatar', selectedGroupAvatarFile.value)
  uploadingGroupAvatar.value = true
  try {
    await api.post(`/api/v1/groups/${encodeURIComponent(groupUUID)}/avatar`, formData)
    await Promise.allSettled([chat.fetchConversations(), chat.fetchGroup(groupUUID)])
    selectedGroupAvatarFile.value = null
    selectedGroupAvatarName.value = ''
    if (groupAvatarInputRef.value) groupAvatarInputRef.value.value = ''
  } catch (e: any) {
    alert(e?.message || '群头像上传失败')
  } finally {
    uploadingGroupAvatar.value = false
  }
}

const dismissGroup = async () => {
  if (!activeConv.value || activeConv.value.target_type !== 1 || !isGroupOwner.value || isGroupDismissed.value) return
  const groupUUID = activeConv.value.conversation_key.replace('group:', '')
  const confirmed = window.confirm('解散后群聊将进入只读状态，成员只能查看历史消息。确认解散吗？')
  if (!confirmed) return
  try {
    await api.post(`/api/v1/groups/${encodeURIComponent(groupUUID)}/dismiss`)
    await Promise.allSettled([chat.fetchConversations(), chat.fetchGroup(groupUUID)])
  } catch (e: any) {
    alert(e?.message || '解散群失败')
  }
}

const saveGroupRemark = async () => {
  if (!activeConv.value || activeConv.value.target_type !== 1) return
  const groupUUID = activeConv.value.conversation_key.replace('group:', '')
  try {
    const data = await api.patch(`/api/v1/conversations/group/${encodeURIComponent(groupUUID)}/remark`, {
      remark: groupRemarkDraft.value.trim(),
    }) as { remark: string }
    activeConv.value.remark = data.remark || ''
    await chat.fetchConversations()
  } catch (e: any) {
    alert(e?.message || '群备注保存失败')
  }
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

const handleWsPacket = async (packet: WsPacket) => {
  const { type, data } = packet
  switch (type) {
    case 'connected':
      await Promise.allSettled([chat.fetchConversations(), chat.fetchDevices(), chat.fetchApplications()])
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
    case 'group.dismissed': {
      const groupUUID = (data as any)?.group_uuid
      await chat.fetchConversations()
      if (groupUUID) await chat.fetchGroup(groupUUID).catch(() => {})
      break
    }
    case 'group.message.notify': {
      const notify = data as unknown as GroupMessageNotify
      const groupUUID = notify.group_uuid
      const key = `group:${groupUUID}`
      const conv = chat.conversations.find(item => item.conversation_key === key)
      if (conv) {
        conv.last_message = {
          message_id: notify.latest_message_id,
          message_type: notify.message_type,
          preview: notify.preview || '',
          sent_at: notify.sent_at,
        }
        if (chat.activeKey !== key) {
          conv.unread_count += 1
        }
      } else {
        await chat.fetchConversations()
      }
      const group = chat.groups.get(groupUUID)
      if (group) {
        group.is_hot = true
        group.recent_message_count = notify.recent_message_count
      }
      if (chat.activeKey === key && groupUUID) {
        await pullHotGroupMessages(groupUUID)
        const activeConv = chat.conversations.find(item => item.conversation_key === key)
        if (activeConv) {
          await chat.markRead(activeConv).catch(() => {})
        }
        scrollToBottom()
      }
      break
    }
  }
}

const ws = useWebSocket({ onMessage: handleWsPacket })

// ── Lifecycle ─────────────────────────────────────────────────────────────────

onMounted(async () => {
  if (!auth.token) return
  await auth.fetchMe()
  await Promise.allSettled([chat.fetchConversations(), chat.fetchContacts()])
  ws.connect(auth.token)
  setupMediaObserver()
})

watch(currentMessages, () => nextTick(scrollToBottom))
watch(currentMessages, () => {
  nextTick(observeMediaCards)
})
watch(() => activeConv.value?.conversation_key, () => {
  revokeMediaPreviewURLs()
  setupMediaObserver()
  nextTick(observeMediaCards)
  if (activeConv.value?.target_type === 1) {
    groupRemarkDraft.value = activeConv.value.remark || ''
    const group = groupFromConv(activeConv.value)
    groupNameDraft.value = group?.name || ''
  } else {
    groupRemarkDraft.value = ''
    groupNameDraft.value = ''
  }
})

onBeforeUnmount(() => {
  revokeMediaPreviewURLs()
  mediaObserver?.disconnect()
})
</script>

<style scoped>
.im-container {
  display: flex;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  position: relative;
}

.left-panel {
  display: flex;
  flex-shrink: 0;
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
  border: none;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 14px;
  font-weight: bold;
  flex-shrink: 0;
}

.profile-btn {
  cursor: pointer;
}

.nav-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.profile-modal {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 4px 0 8px;
}

.profile-avatar-preview {
  width: 72px;
  height: 72px;
  border-radius: 10px;
  overflow: hidden;
  background: #d8d8d8;
  color: #555;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  font-weight: 600;
}

.profile-avatar-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.profile-meta {
  font-size: 14px;
  font-weight: 600;
}

.profile-meta.secondary {
  font-size: 12px;
  color: #888;
  font-weight: 400;
}

.profile-file-name {
  font-size: 12px;
  color: #666;
  word-break: break-all;
}

.profile-signature-input {
  width: 100%;
  min-height: 72px;
  border: 1px solid #ddd;
  border-radius: 8px;
  padding: 10px;
  box-sizing: border-box;
  font-size: 13px;
  resize: vertical;
}

.profile-signature {
  width: 100%;
  background: #f4f5f6;
  border-radius: 8px;
  padding: 10px;
  box-sizing: border-box;
  font-size: 13px;
  color: #555;
  line-height: 1.5;
  word-break: break-word;
}

/* User Profile Card */
.user-profile-card {
  width: 280px;
  padding: 28px 24px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  position: relative;
}

.upc-close {
  position: absolute;
  top: 12px;
  right: 12px;
  background: none;
  border: none;
  font-size: 14px;
  color: #aaa;
  cursor: pointer;
  line-height: 1;
  padding: 4px;
}

.upc-close:hover { color: #555; }

.upc-avatar {
  width: 72px;
  height: 72px;
  border-radius: 12px;
  overflow: hidden;
  background: #d8d8d8;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  font-weight: 600;
  color: #555;
  margin-bottom: 12px;
}

.upc-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.upc-name {
  font-size: 16px;
  font-weight: 600;
  color: #1a1a1a;
  text-align: center;
  margin-bottom: 4px;
}

.upc-uuid {
  font-size: 11px;
  color: #bbb;
  text-align: center;
  word-break: break-all;
  margin-bottom: 8px;
}

.upc-signature {
  font-size: 12px;
  color: #888;
  text-align: center;
  line-height: 1.5;
  margin-bottom: 8px;
  padding: 0 4px;
}

.upc-remark {
  width: 100%;
  display: flex;
  gap: 6px;
  margin: 10px 0 4px;
}

.upc-remark-input {
  flex: 1;
  min-width: 0;
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  padding: 6px 10px;
  font-size: 12px;
  outline: none;
  background: #fafafa;
}

.upc-remark-input:focus { border-color: #07c160; }

.upc-remark-btn {
  flex-shrink: 0;
  padding: 6px 12px;
  border: none;
  border-radius: 6px;
  background: #e8e8e8;
  color: #444;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
}

.upc-remark-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.upc-actions {
  width: 100%;
  display: flex;
  gap: 8px;
  margin-top: 16px;
}

.upc-btn {
  flex: 1;
  padding: 9px 0;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  background: #fff;
  color: #333;
  font-size: 13px;
  cursor: pointer;
}

.upc-btn.primary {
  background: #07c160;
  border-color: #07c160;
  color: #fff;
  font-weight: 500;
}

.upc-btn:hover { opacity: 0.88; }

.upc-avatar {
  position: relative;
}

.upc-avatar-hint {
  position: absolute;
  inset: 0;
  border-radius: 12px;
  background: rgba(0,0,0,0.38);
  color: #fff;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s;
}

.upc-avatar:hover .upc-avatar-hint { opacity: 1; }

.upc-signature-input {
  width: 100%;
  min-height: 72px;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 10px;
  box-sizing: border-box;
  font-size: 13px;
  resize: vertical;
  outline: none;
  font-family: inherit;
  margin-top: 4px;
}

.upc-signature-input:focus { border-color: #07c160; }

.upc-file-hint {
  font-size: 11px;
  color: #aaa;
  align-self: flex-start;
  margin-top: -4px;
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

.contacts-btn {
  position: relative;
}

.nav-badge {
  position: absolute;
  top: -4px;
  right: -6px;
  background: #f00;
  color: #fff;
  font-size: 9px;
  padding: 1px 4px;
  border-radius: 8px;
  line-height: 1.4;
  pointer-events: none;
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

.clickable {
  cursor: pointer;
}

.clickable-name {
  cursor: pointer;
}

.clickable-name:hover {
  text-decoration: underline;
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

.status-chip {
  font-size: 11px;
  line-height: 1;
  padding: 4px 8px;
  border-radius: 999px;
  background: #e8f7ee;
  color: #07c160;
}

.status-chip.danger {
  background: #fdeaea;
  color: #c53b3b;
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
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.msg-item.self {
  align-items: flex-end;
}

.msg-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.msg-item.self .msg-row {
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

.tool-btn.disabled {
  cursor: not-allowed;
  opacity: 0.35;
}

.upload-status {
  font-size: 12px;
  color: #666;
}

.msg-bubble.media {
  padding: 0;
  overflow: hidden;
  max-width: 240px;
  background: transparent;
  border: none;
}

.msg-item.self .msg-bubble.media {
  background: transparent;
}

.media-card {
  display: flex;
  flex-direction: column;
  width: 240px;
  border-radius: 8px;
  overflow: hidden;
  background: #1a1a1a;
  box-sizing: border-box;
}

.media-image,
.media-video {
  width: 100%;
  max-height: 220px;
  border-radius: 0;
  background: #111;
  object-fit: contain;
  display: block;
}

.media-image {
  cursor: pointer;
}

.media-placeholder {
  width: 100%;
  min-height: 140px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #1a1a1a;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
}

.media-caption {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 8px;
  background: rgba(0, 0, 0, 0.55);
  font-size: 12px;
  color: #e0e0e0;
}

.media-name {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  line-height: 1.3;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.file-action-btn {
  border: 1px solid rgba(255, 255, 255, 0.25);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.12);
  color: #e0e0e0;
  padding: 4px 10px;
  white-space: nowrap;
  cursor: pointer;
  font-size: 11px;
  font-weight: 500;
  line-height: 1;
  flex-shrink: 0;
}

.file-action-btn:hover {
  background: rgba(255, 255, 255, 0.22);
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

.input-area textarea:disabled {
  background: #f5f5f5;
  color: #888;
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

.send-btn:disabled {
  background: #b8c1cc;
  cursor: not-allowed;
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
  padding: 0;
  overflow-y: auto;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}

.detail-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 20px 16px 14px;
  border-bottom: 1px solid #e8e8e8;
}

.detail-avatar {
  width: 60px;
  height: 60px;
  border-radius: 8px;
  overflow: hidden;
  background: #bbb;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 6px;
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
  font-size: 14px;
  font-weight: 600;
  text-align: center;
}

.detail-uuid {
  font-size: 10px;
  color: #bbb;
  word-break: break-all;
  text-align: center;
}

.detail-meta {
  font-size: 12px;
  color: #888;
  text-align: center;
}

.detail-chips {
  display: flex;
  gap: 6px;
  margin-top: 4px;
}

.detail-chip {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  background: #e8e8e8;
  color: #555;
}

.detail-chip.danger {
  background: #fdeaea;
  color: #c53b3b;
}

.detail-section-title {
  font-size: 11px;
  font-weight: 600;
  color: #999;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 12px 16px 4px;
}

.detail-field {
  padding: 0 16px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-field label {
  font-size: 12px;
  color: #666;
}

.detail-field-row {
  display: flex;
  gap: 6px;
  align-items: center;
}

.detail-edit-input {
  flex: 1;
  min-width: 0;
  border: 1px solid #ddd;
  border-radius: 6px;
  padding: 6px 8px;
  box-sizing: border-box;
  font-size: 12px;
  background: #fff;
}

.detail-inline-btn {
  flex-shrink: 0;
  padding: 5px 10px;
  border: none;
  border-radius: 6px;
  background: #07c160;
  color: #fff;
  cursor: pointer;
  font-size: 12px;
  white-space: nowrap;
}

.detail-inline-btn.secondary {
  background: #5a6672;
}

.detail-inline-btn:disabled {
  background: #c8ced6;
  cursor: not-allowed;
}

.detail-file-hint {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  color: #aaa;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-danger-zone {
  margin-top: auto;
  padding: 16px;
  border-top: 1px solid #e8e8e8;
}

.danger-btn {
  width: 100%;
  padding: 8px 0;
  border: none;
  border-radius: 8px;
  background: #df4c4c;
  color: #fff;
  cursor: pointer;
  font-size: 13px;
}

.danger-btn:hover {
  background: #c63f3f;
}

/* Back button (mobile) */
.back-btn {
  display: none;
  background: none;
  border: none;
  font-size: 24px;
  line-height: 1;
  cursor: pointer;
  color: #555;
  padding: 0 4px;
  margin-right: 4px;
}

.chat-header-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Mobile responsive */
@media (max-width: 768px) {
  .left-panel {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 10;
    width: 100%;
    transition: transform 0.22s ease;
    background: #ededed;
  }

  .left-panel.hidden {
    transform: translateX(-100%);
    pointer-events: none;
  }

  .session-panel {
    flex: 1;
    width: auto;
  }

  .chat-area {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    bottom: 0;
  }

  .detail-panel {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    z-index: 10;
    width: 82% !important;
    box-shadow: -4px 0 20px rgba(0,0,0,0.15);
  }

  .back-btn {
    display: block;
  }
}

/* Panel list (shared by all tabs) */
.panel-list {
  flex: 1;
  overflow-y: auto;
}

/* Contact / app items */
.conv-avatar.small {
  width: 32px;
  height: 32px;
  font-size: 13px;
}

.section-title {
  padding: 6px 12px;
  font-size: 12px;
  color: #888;
  background: #e4e4e4;
  border-bottom: 1px solid #d8d8d8;
}

.app-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  gap: 8px;
  border-bottom: 1px solid #e8e8e8;
  font-size: 13px;
}

.app-actions {
  display: flex;
  gap: 5px;
  flex-shrink: 0;
}

.app-actions button {
  padding: 3px 8px;
  border: 1px solid #ddd;
  border-radius: 3px;
  cursor: pointer;
  font-size: 11px;
  background: #f5f5f5;
}

.app-actions button:hover {
  background: #e0e0e0;
}

/* Panel action bar */
.panel-actions {
  padding: 8px 10px;
  border-bottom: 1px solid #d8d8d8;
}

.panel-action-btn {
  width: 100%;
  padding: 6px 0;
  background: #07c160;
  color: #fff;
  border: none;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
}

.panel-action-btn:hover {
  background: #06ad56;
}

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0,0,0,0.4);
}

.modal {
  position: relative;
  z-index: 1;
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  width: 320px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  gap: 8px;
  box-sizing: border-box;
  overflow: hidden;
}

.modal-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.modal-footer {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex-shrink: 0;
  padding-top: 8px;
}

.modal-title {
  font-size: 15px;
  font-weight: 600;
  margin-bottom: 4px;
}

.modal input[type="text"],
.modal input:not([type="checkbox"]) {
  width: 100%;
  padding: 7px 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
  box-sizing: border-box;
}

.modal-btn {
  padding: 7px 0;
  background: #07c160;
  color: #fff;
  border: none;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
}

.modal-btn:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.modal-btn.small {
  padding: 4px 10px;
  font-size: 12px;
}

.modal-close {
  padding: 6px 0;
  background: none;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 13px;
  cursor: pointer;
  color: #666;
}

.modal-close:hover {
  background: #f5f5f5;
}

/* 群聊头像（对话列表） */
.conv-avatar-group {
  background: #5b8dd9;
  font-size: 20px;
}

.group-icon {
  font-size: 20px;
  line-height: 1;
}

/* 群成员格子 */
.member-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  width: 100%;
  margin-top: 8px;
}

.member-grid-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  width: 52px;
  position: relative;
  cursor: default;
}

.member-grid-avatar {
  width: 40px;
  height: 40px;
  border-radius: 4px;
  background: #bbb;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 14px;
  font-weight: bold;
  overflow: hidden;
  flex-shrink: 0;
}

.member-grid-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.member-grid-add {
  background: #e0e0e0;
  color: #888;
  font-size: 22px;
  cursor: pointer;
  border: 1px dashed #bbb;
}

.member-grid-add:hover {
  background: #d0d0d0;
}

.member-grid-name {
  font-size: 10px;
  color: #666;
  text-align: center;
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-role-badge {
  position: absolute;
  top: 0;
  right: 0;
  background: #f5a623;
  color: white;
  font-size: 9px;
  padding: 0 3px;
  border-radius: 2px;
  line-height: 1.4;
}

/* 群聊消息发送者名字 */
.msg-sender-name {
  font-size: 11px;
  color: #888;
  margin-bottom: 3px;
}

/* detail panel 群头像 fallback */
.group-avatar-fallback {
  font-size: 28px;
  background: #5b8dd9;
}

.search-results {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.search-result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  border-bottom: 1px solid #f0f0f0;
}

.member-select-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  border: 1px solid #eee;
  border-radius: 4px;
  padding: 4px;
}

.member-select-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 6px;
  cursor: pointer;
  border-radius: 3px;
}

.member-select-item:hover {
  background: #f5f5f5;
}
</style>
