import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Conversation, Message, Contact, FriendApplication, Group, Device, PublicUser } from '@/types'
import api from '@/api'

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const contacts = ref<Contact[]>([])
  const applications = ref<{ incoming: FriendApplication[]; outgoing: FriendApplication[] }>({ incoming: [], outgoing: [] })
  const groups = ref<Map<string, Group>>(new Map())
  const users = ref<Map<string, PublicUser>>(new Map())
  const devices = ref<Device[]>([])
  const messageMap = ref<Map<string, Message[]>>(new Map())
  const activeKey = ref('')
  const lastOfflineID = ref(Number(localStorage.getItem('dipole.web.lastOfflineID') || '0'))

  // ── conversations ──────────────────────────────────────────────
  const fetchConversations = async () => {
    const data = await api.get('/api/v1/conversations?limit=50') as Conversation[]
    conversations.value = Array.isArray(data) ? data : []
    conversations.value.forEach(c => { if (c.target_user) users.value.set(c.target_user.uuid, c.target_user) })
  }

  const markRead = async (conv: Conversation) => {
    const path = conv.target_type === 1
      ? `/api/v1/conversations/group/${conv.target_group?.uuid}/read`
      : `/api/v1/conversations/direct/${conv.target_user?.uuid}/read`
    await api.patch(path)
    conv.unread_count = 0
  }

  // ── messages ───────────────────────────────────────────────────
  const fetchDirectMessages = async (targetUUID: string, beforeID?: number) => {
    const q = beforeID ? `?before_id=${beforeID}&limit=30` : '?limit=30'
    const data = await api.get(`/api/v1/messages/direct/${targetUUID}${q}`) as Message[]
    _mergeMessages(directKey(targetUUID), Array.isArray(data) ? data : [], Boolean(beforeID))
  }

  const fetchGroupMessages = async (groupUUID: string, beforeID?: number) => {
    const q = beforeID ? `?before_id=${beforeID}&limit=30` : '?limit=30'
    const data = await api.get(`/api/v1/messages/group/${groupUUID}${q}`) as Message[]
    _mergeMessages(groupKey(groupUUID), Array.isArray(data) ? data : [], Boolean(beforeID))
  }

  const syncOffline = async () => {
    const data = await api.get(`/api/v1/messages/offline?after_id=${lastOfflineID.value}&limit=100`) as Message[]
    const items = Array.isArray(data) ? data : []
    items.forEach(m => pushMessage(m))
    _updateLastOfflineID(items)
    return items.length
  }

  const pushMessage = (msg: Message) => {
    const key = deriveKey(msg)
    const list = messageMap.value.get(key) || []
    if (!list.some(m => m.message_id === msg.message_id)) {
      list.push(msg)
      list.sort((a, b) => a.id - b.id || new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime())
      messageMap.value.set(key, list)
      // update conversation preview
      const conv = conversations.value.find(c => c.conversation_key === key)
      if (conv) {
        conv.last_message = { message_id: msg.message_id, message_type: msg.message_type, preview: msg.content, sent_at: msg.sent_at }
        conv.unread_count = key === activeKey.value ? 0 : conv.unread_count + 1
      }
    }
    _updateLastOfflineID([msg])
  }

  // ── contacts ───────────────────────────────────────────────────
  const fetchContacts = async () => {
    const data = await api.get('/api/v1/contacts') as Contact[]
    contacts.value = Array.isArray(data) ? data : []
    contacts.value.forEach(c => users.value.set(c.user.uuid, c.user))
  }

  const fetchApplications = async () => {
    const [inc, out] = await Promise.all([
      api.get('/api/v1/contacts/applications?box=incoming') as Promise<FriendApplication[]>,
      api.get('/api/v1/contacts/applications?box=outgoing') as Promise<FriendApplication[]>,
    ])
    applications.value = { incoming: Array.isArray(inc) ? inc : [], outgoing: Array.isArray(out) ? out : [] }
  }

  // ── groups ─────────────────────────────────────────────────────
  const fetchGroup = async (uuid: string) => {
    const data = await api.get(`/api/v1/groups/${encodeURIComponent(uuid)}`) as Group
    groups.value.set(data.uuid, data)
    return data
  }

  // ── devices ────────────────────────────────────────────────────
  const fetchDevices = async () => {
    const data = await api.get('/api/v1/users/me/devices') as Device[]
    devices.value = Array.isArray(data) ? data : []
  }

  // ── users cache ────────────────────────────────────────────────
  const ensureUser = async (uuid: string): Promise<PublicUser | null> => {
    if (users.value.has(uuid)) return users.value.get(uuid)!
    try {
      const data = await api.get(`/api/v1/users/${encodeURIComponent(uuid)}`) as PublicUser
      users.value.set(uuid, data)
      return data
    } catch { return null }
  }

  // ── helpers ────────────────────────────────────────────────────
  const _mergeMessages = (key: string, items: Message[], prepend: boolean) => {
    const existing = messageMap.value.get(key) || []
    const merged = _dedupe(prepend ? [...items, ...existing] : [...existing, ...items])
    merged.sort((a, b) => a.id - b.id || new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime())
    messageMap.value.set(key, merged)
  }

  const _dedupe = (msgs: Message[]) => {
    const seen = new Set<string>()
    return msgs.filter(m => { if (seen.has(m.message_id)) return false; seen.add(m.message_id); return true })
  }

  const _updateLastOfflineID = (msgs: Message[]) => {
    msgs.forEach(m => { if (m.id > lastOfflineID.value) lastOfflineID.value = m.id })
    localStorage.setItem('dipole.web.lastOfflineID', String(lastOfflineID.value))
  }

  return {
    conversations, contacts, applications, groups, users, devices, messageMap, activeKey,
    fetchConversations, markRead,
    fetchDirectMessages, fetchGroupMessages, syncOffline, pushMessage,
    fetchContacts, fetchApplications,
    fetchGroup, fetchDevices, ensureUser,
  }
})

// ── key helpers (exported for use in views) ──────────────────────
export const directKey = (myUUID: string, peerUUID?: string) => {
  // When called with one arg from pushMessage, we need both UUIDs
  // When called from view with peer UUID, we need current user UUID too
  // We'll handle this by always passing the conversation_key from API when possible
  return peerUUID ? `direct:${[myUUID, peerUUID].sort().join(':')}` : myUUID
}

export const groupKey = (uuid: string) => `group:${uuid}`

export const deriveKey = (msg: Message): string => {
  if (msg.target_type === 1) return groupKey(msg.target_uuid)
  // For direct: key is stored as conversation_key in conversations list
  // We derive it the same way the backend does: sorted UUIDs
  // But we don't have currentUser here — store will handle this via pushMessage caller
  return `direct:${msg.from_uuid}:${msg.target_uuid}` // placeholder, fixed in ChatView
}
