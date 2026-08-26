import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Conversation, Message, Contact, FriendApplication, Group, Device, PublicUser, GroupSyncCheckpoint } from '@/types'
import api from '@/api'
import {
  browserSyncEnabled,
  browserSyncMode,
  clearBrowserMessages,
  compareBrowserSyncMessages,
  recoverBrowserMessages,
} from '@/sync/browserSync'
import { drainLegacyOffline } from '@/sync/legacyOffline'

export type MessageSyncStatus = 'idle' | 'restoring' | 'current' | 'error'

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const contacts = ref<Contact[]>([])
  const applications = ref<{ incoming: FriendApplication[]; outgoing: FriendApplication[] }>({ incoming: [], outgoing: [] })
  const groups = ref<Map<string, Group>>(new Map())
  const users = ref<Map<string, PublicUser>>(new Map())
  const devices = ref<Device[]>([])
  const messageMap = ref<Map<string, Message[]>>(new Map())
  const activeKey = ref('')
  const syncStatus = ref<MessageSyncStatus>('idle')
  const safeSyncSeq = ref(0)
  const lastOfflineID = ref(Number(localStorage.getItem('dipole.web.lastOfflineID') || '0'))
  let activeSync: Promise<number> | undefined
  let pendingComparisonTimer: ReturnType<typeof setTimeout> | undefined
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
  const fetchDirectMessages = async (targetUUID: string, beforeSeq = 0) => {
    const q = `?before_seq=${beforeSeq}&limit=30`
    const data = await api.get(`/api/v1/messages/direct/${targetUUID}${q}`) as Message[]
    const key = myUUID.value
      ? `direct:${[myUUID.value, targetUUID].sort().join(':')}`
      : `direct:${targetUUID}`
    _mergeMessages(key, Array.isArray(data) ? data : [], beforeSeq > 0)
  }

  const fetchGroupMessages = async (groupUUID: string, beforeSeq = 0) => {
    const q = `?before_seq=${beforeSeq}&limit=30`
    const data = await api.get(`/api/v1/messages/group/${groupUUID}${q}`) as Message[]
    _mergeMessages(groupKey(groupUUID), Array.isArray(data) ? data : [], beforeSeq > 0)
  }

  const fetchGroupMessagesAfter = async (groupUUID: string, afterID: number) => {
    const q = `?after_id=${afterID}&limit=100`
    const data = await api.get(`/api/v1/messages/group/${groupUUID}${q}`) as Message[]
    _mergeMessages(groupKey(groupUUID), Array.isArray(data) ? data : [], false)
    return Array.isArray(data) ? data : []
  }

  const fetchGroupMessagesAfterSeq = async (groupUUID: string, afterSeq: number) => {
	const q = `?after_seq=${afterSeq}&limit=100`
	const data = await api.get(`/api/v1/messages/group/${groupUUID}${q}`) as Message[]
	const items = Array.isArray(data) ? data : []
	_mergeMessages(groupKey(groupUUID), items, false)
	return items
  }

  const recoverGroupMessages = async () => {
	const groupUUIDs = conversations.value
	  .filter(conversation => conversation.target_type === 1)
	  .map(conversation => conversation.conversation_key.replace('group:', ''))
	  .filter(Boolean)
	if (groupUUIDs.length === 0) return 0
	const query = new URLSearchParams()
	groupUUIDs.forEach(groupUUID => query.append('group_id', groupUUID))
	const checkpoints = await api.get(`/api/v1/sync/groups/checkpoints?${query.toString()}`) as GroupSyncCheckpoint[]
	let recovered = 0
	for (const checkpoint of Array.isArray(checkpoints) ? checkpoints : []) {
	  const key = groupKey(checkpoint.group_uuid)
	  // Volatile Web memory cannot trust a durable ACK after refresh. IndexedDB clients
	  // may combine pulled_message_seq with their locally committed high-water later.
	  let afterSeq = Math.max(0, ...(messageMap.value.get(key) || []).map(message => message.message_seq || 0))
	  while (afterSeq < checkpoint.latest_message_seq) {
		const page = await fetchGroupMessagesAfterSeq(checkpoint.group_uuid, afterSeq)
		if (page.length === 0) break
		const nextSeq = Math.max(...page.map(message => message.message_seq || 0))
		if (nextSeq <= afterSeq) break
		afterSeq = nextSeq
		recovered += page.length
	  }
	}
	return recovered
  }

  const syncOffline = async () => {
    const items = await _fetchLegacyOfflineMessages(true)
    return items.length
  }

  const syncMessages = () => {
    if (!browserSyncEnabled || !myUUID.value) return Promise.resolve(0)
    if (activeSync) return activeSync
    syncStatus.value = 'restoring'
    activeSync = (async () => {
      let legacyMessages: Message[] | undefined
      try {
        legacyMessages = await _fetchLegacyOfflineMessages(browserSyncMode === 'shadow')
      } catch (error) {
        if (browserSyncMode === 'shadow') throw error
      }

      const remoteSyncMessages: Message[] = []
      const result = await recoverBrowserMessages(myUUID.value, (messages, source) => {
        if (source === 'remote') remoteSyncMessages.push(...messages)
        if (browserSyncMode === 'primary') messages.forEach(message => pushMessage(message))
      })
      if (legacyMessages) {
        const comparison = await compareBrowserSyncMessages(myUUID.value, legacyMessages, remoteSyncMessages)
        if (pendingComparisonTimer) clearTimeout(pendingComparisonTimer)
        pendingComparisonTimer = comparison.pending > 0
          ? setTimeout(() => { void syncMessages().catch(() => {}) }, 61_000)
          : undefined
      }
      safeSyncSeq.value = result.syncSeq
      syncStatus.value = 'current'
      return result.synchronized
    })().catch(error => {
      syncStatus.value = 'error'
      throw error
    }).finally(() => {
      activeSync = undefined
    })
    return activeSync
  }

  const clearLocalMessages = async (userUUID = myUUID.value) => {
    if (activeSync) await activeSync.catch(() => {})
    if (pendingComparisonTimer) clearTimeout(pendingComparisonTimer)
    pendingComparisonTimer = undefined
    if (userUUID) await clearBrowserMessages(userUUID)
    messageMap.value = new Map()
    activeKey.value = ''
    safeSyncSeq.value = 0
    syncStatus.value = 'idle'
  }

  const pushMessage = (msg: Message) => {
    const key = _deriveKey(msg)
    const list = messageMap.value.get(key) || []
    const existingIndex = list.findIndex(message => message.message_id === msg.message_id)
    if (existingIndex < 0) {
      list.push(msg)
      const conv = conversations.value.find(c => c.conversation_key === key)
      if (conv) {
        conv.last_message = { message_id: msg.message_id, message_type: msg.message_type, preview: msg.content, sent_at: msg.sent_at, sender_uuid: msg.from_uuid }
        conv.unread_count = key === activeKey.value ? 0 : conv.unread_count + 1
      }
    } else if (_messagePersistenceRank(msg) > _messagePersistenceRank(list[existingIndex])) {
      list[existingIndex] = msg
    }
    list.sort(_compareMessages)
    messageMap.value.set(key, list)
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
    merged.sort(_compareMessages)
    messageMap.value.set(key, merged)
  }

  const _dedupe = (msgs: Message[]) => {
    const deduped: Message[] = []
    const positions = new Map<string, number>()
    msgs.forEach(message => {
      const position = positions.get(message.message_id)
      if (position === undefined) {
        positions.set(message.message_id, deduped.length)
        deduped.push(message)
      } else if (_messagePersistenceRank(message) > _messagePersistenceRank(deduped[position])) {
        deduped[position] = message
      }
    })
    return deduped
  }

  const _messagePersistenceRank = (message: Message) =>
    ((message.message_seq || 0) > 0 ? 2 : 0) + (message.id > 0 ? 1 : 0)

  const _compareMessages = (left: Message, right: Message) => {
    const leftSeq = left.message_seq || 0
    const rightSeq = right.message_seq || 0
    if (leftSeq > 0 && rightSeq > 0 && leftSeq !== rightSeq) return leftSeq - rightSeq
    const sentAt = new Date(left.sent_at).getTime() - new Date(right.sent_at).getTime()
    return sentAt || left.id - right.id
  }

  const _updateLastOfflineID = (msgs: Message[]) => {
    msgs.forEach(m => { if (m.id > lastOfflineID.value) lastOfflineID.value = m.id })
    localStorage.setItem('dipole.web.lastOfflineID', String(lastOfflineID.value))
  }

  const _fetchLegacyOfflineMessages = async (deliver: boolean) => {
    return drainLegacyOffline(
      lastOfflineID.value,
      async (afterID, limit) => {
        const data = await api.get(`/api/v1/messages/offline?after_id=${afterID}&limit=${limit}`) as Message[]
        return Array.isArray(data) ? data : []
      },
      (items, nextID) => {
        if (deliver) items.forEach(message => pushMessage(message))
        lastOfflineID.value = nextID
        localStorage.setItem('dipole.web.lastOfflineID', String(nextID))
      },
    )
  }

  return {
    conversations, contacts, applications, groups, users, devices, messageMap, activeKey, myUUID,
    syncStatus, safeSyncSeq,
    fetchConversations, markRead,
    fetchDirectMessages, fetchGroupMessages, syncOffline, syncMessages, clearLocalMessages, pushMessage,
	fetchGroupMessagesAfter,
	fetchGroupMessagesAfterSeq, recoverGroupMessages,
    fetchContacts, fetchApplications,
    fetchGroup, fetchDevices, ensureUser,
  }
})

export const groupKey = (uuid: string) => `group:${uuid}`
