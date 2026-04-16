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
  // current user UUID — set by auth store after login, needed for key derivation
  const myUUID = ref(localStorage.getItem('dipole.web.user')
    ? (() => { try { return JSON.parse(localStorage.getItem('dipole.web.user')!).uuid } catch { return '' } })()
    : '')

  // ── conversations ──────────────────────────────────────────────
  const fetchConversations = async () => {
    const data = await api.get('/api/v1/conversations?limit=50') as Conversation[]
    conversations.value = Array.isArray(data) ? data : []
    conversations.value.forEach(c => { if (c.target_user) users.value.set(c.target_user.uuid, c.target_user) })
    // Backend ConversationResponse has no target_group — fetch group info separately
    const groupUUIDs = conversations.value
      .filter(c => c.target_type === 1)
      .map(c => c.conversation_key.replace('group:', ''))
      .filter(uuid => uuid && !groups.value.has(uuid))
    await Promise.allSettled(groupUUIDs.map(uuid => fetchGroup(uuid)))
  }

  const markRead = async (conv: Conversation) => {
    const path = conv.target_type === 1
      ? `/api/v1/conversations/group/${conv.conversation_key.replace('group:', '')}/read`
      : `/api/v1/conversations/direct/${conv.target_user?.uuid}/read`
    await api.patch(path)
    conv.unread_count = 0
  }

  // ── messages ───────────────────────────────────────────────────
  const fetchDirectMessages = async (targetUUID: string, beforeID?: number) => {
    const q = beforeID ? `?before_id=${beforeID}&limit=30` : '?limit=30'
    const data = await api.get(`/api/v1/messages/direct/${targetUUID}${q}`) as Message[]
    const key = myUUID.value
      ? `direct:${[myUUID.value, targetUUID].sort().join(':')}`
      : `direct:${targetUUID}`
    _mergeMessages(key, Array.isArray(data) ? data : [], Boolean(beforeID))
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
    const key = _deriveKey(msg)
    const list = messageMap.value.get(key) || []
    if (!list.some(m => m.message_id === msg.message_id)) {
      list.push(msg)
      list.sort((a, b) => a.id - b.id || new Date(a.sent_at).getTime() - new Date(b.sent_at).getTime())
      messageMap.value.set(key, list)
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
  const _deriveKey = (msg: Message): string => {
    if (msg.target_type === 1) return `group:${msg.target_uuid}`
    const peer = msg.from_uuid === myUUID.value ? msg.target_uuid : msg.from_uuid
    return `direct:${[myUUID.value, peer].sort().join(':')}`
  }

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
    conversations, contacts, applications, groups, users, devices, messageMap, activeKey, myUUID,
    fetchConversations, markRead,
    fetchDirectMessages, fetchGroupMessages, syncOffline, pushMessage,
    fetchContacts, fetchApplications,
    fetchGroup, fetchDevices, ensureUser,
  }
})

export const groupKey = (uuid: string) => `group:${uuid}`
