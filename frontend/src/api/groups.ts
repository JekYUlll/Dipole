import api from '@/api'
import type { Group, PublicUser } from '@/types'

export interface GroupDirectoryClient {
  list(): Promise<Group[]>
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const groupKeys = new Set(['uuid', 'name', 'notice', 'avatar', 'status', 'member_count', 'is_hot', 'recent_message_count', 'owner', 'me_role', 'members', 'created_at'])
const conversationKeys = new Set(['conversation_key', 'target_type', 'target_user', 'remark', 'last_message', 'unread_count', 'last_message_seq', 'read_seq'])
const messageKeys = new Set(['message_id', 'message_type', 'preview', 'sent_at', 'sender_uuid'])
const userKeys = new Set(['uuid', 'nickname', 'avatar', 'signature', 'user_type', 'status'])

export const groupDirectoryClient: GroupDirectoryClient = {
  async list(): Promise<Group[]> {
    const groupIDs = parseConversationGroupIDs(await api.get('/api/v1/conversations?limit=50'))
    const groups = await Promise.all(groupIDs.map(async (id) => parseGroupDirectoryItem(await api.get(`/api/v1/groups/${id}`))))
    return groups
  },
}

export function parseConversationGroupIDs(raw: unknown): string[] {
  if (!Array.isArray(raw)) throw new Error('conversation projection shape is invalid')
  const ids = new Set<string>()
  for (const item of raw) {
    if (!isRecord(item) || !exactKeys(item, conversationKeys) || typeof item.conversation_key !== 'string' ||
      !Number.isSafeInteger(item.target_type) || typeof item.remark !== 'string' ||
      !Number.isSafeInteger(item.unread_count) || !nonNegativeInteger(item.last_message_seq) || !nonNegativeInteger(item.read_seq) ||
      !validLastMessage(item.last_message) || (item.target_user !== undefined && !validUser(item.target_user))) {
      throw new Error('conversation directory item is invalid')
    }
    if (item.target_type !== 1) continue
    const match = /^group:([A-Za-z0-9][A-Za-z0-9_.:-]{0,127})$/.exec(item.conversation_key)
    if (!match) throw new Error('group conversation key is invalid')
    ids.add(match[1])
  }
  return [...ids]
}

export function parseGroupDirectoryItem(raw: unknown): Group {
  if (!isRecord(raw)) throw new Error('group directory item is invalid')
  const status = raw.status
  const owner = raw.owner
  if (!isRecord(raw) || !exactKeys(raw, groupKeys) || typeof raw.uuid !== 'string' || !identifier.test(raw.uuid) ||
    typeof raw.name !== 'string' || typeof raw.notice !== 'string' || typeof raw.avatar !== 'string' ||
    (status !== 0 && status !== 1) || !nonNegativeInteger(raw.member_count) || typeof raw.is_hot !== 'boolean' ||
    !nonNegativeInteger(raw.recent_message_count) || !Number.isSafeInteger(raw.me_role) || typeof raw.created_at !== 'string' ||
    (owner !== undefined && !validUser(owner)) || raw.members !== undefined) {
    throw new Error('group directory item is invalid')
  }
  return {
    uuid: raw.uuid, name: raw.name, notice: raw.notice, avatar: raw.avatar, status,
    member_count: raw.member_count, is_hot: raw.is_hot, recent_message_count: raw.recent_message_count,
    owner: owner === undefined ? undefined : toUser(owner), me_role: raw.me_role,
  }
}

function validLastMessage(value: unknown): boolean {
  return isRecord(value) && exactKeys(value, messageKeys) && typeof value.message_id === 'string' &&
    Number.isSafeInteger(value.message_type) && typeof value.preview === 'string' &&
    typeof value.sent_at === 'string' && typeof value.sender_uuid === 'string'
}

function validUser(value: unknown): value is Record<string, unknown> {
  return isRecord(value) && exactKeys(value, userKeys) && typeof value.uuid === 'string' && identifier.test(value.uuid) &&
    typeof value.nickname === 'string' && typeof value.avatar === 'string' && typeof value.signature === 'string' &&
    Number.isSafeInteger(value.user_type) && Number.isSafeInteger(value.status)
}

function toUser(value: Record<string, unknown>): PublicUser {
  return value as unknown as PublicUser
}

function nonNegativeInteger(value: unknown): value is number { return Number.isSafeInteger(value) && (value as number) >= 0 }
function exactKeys(value: Record<string, unknown>, allowed: Set<string>): boolean { return Object.keys(value).every(key => allowed.has(key)) }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
