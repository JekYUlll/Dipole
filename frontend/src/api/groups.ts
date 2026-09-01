import api from '@/api'
import type { Group, PublicUser } from '@/types'

export interface GroupDirectoryClient {
  list(): Promise<Group[]>
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const groupKeys = new Set(['uuid', 'name', 'notice', 'avatar', 'status', 'member_count', 'is_hot', 'recent_message_count', 'owner', 'me_role', 'members', 'created_at'])
const conversationKeys = new Set(['conversation_key', 'target_type', 'target_user', 'target_group', 'remark', 'last_message', 'unread_count', 'last_message_seq', 'read_seq'])
const messageKeys = new Set(['message_id', 'message_type', 'preview', 'sent_at', 'sender_uuid'])
const userKeys = new Set(['uuid', 'nickname', 'avatar', 'signature', 'user_type', 'status'])
const memberKeys = new Set(['user', 'role', 'joined_at'])

export const groupDirectoryClient: GroupDirectoryClient = {
  async list(): Promise<Group[]> {
    const groupIDs = parseConversationGroupIDs(await api.get('/api/v1/conversations?limit=50'))
    return Promise.all(groupIDs.map(async (id) => parseGroupDirectoryItem(await api.get(`/api/v1/groups/${id}`))))
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
  if (!isRecord(raw) || !exactKeys(raw, groupKeys) || typeof raw.uuid !== 'string' || !identifier.test(raw.uuid) ||
    typeof raw.name !== 'string' || typeof raw.notice !== 'string' || typeof raw.avatar !== 'string' ||
    (raw.status !== 0 && raw.status !== 1) || !nonNegativeInteger(raw.member_count) || typeof raw.is_hot !== 'boolean' ||
    !nonNegativeInteger(raw.recent_message_count) || !Number.isSafeInteger(raw.me_role) || typeof raw.created_at !== 'string' ||
    (raw.owner !== undefined && !validUser(raw.owner)) || (raw.members !== undefined && !validMembers(raw.members))) {
    throw new Error('group directory item is invalid')
  }
  const group = raw as Record<string, unknown>
  return {
    uuid: group.uuid as string,
    name: group.name as string,
    notice: group.notice as string,
    avatar: group.avatar as string,
    status: group.status as number,
    member_count: group.member_count as number,
    is_hot: group.is_hot as boolean,
    recent_message_count: group.recent_message_count as number,
    owner: group.owner === undefined ? undefined : toUser(group.owner as Record<string, unknown>),
    me_role: group.me_role as number,
  }
}

function validLastMessage(value: unknown): boolean {
  return isRecord(value) && exactKeys(value, messageKeys) && typeof value.message_id === 'string' &&
    Number.isSafeInteger(value.message_type) && typeof value.preview === 'string' &&
    typeof value.sent_at === 'string' && typeof value.sender_uuid === 'string'
}

function validMembers(value: unknown): boolean {
  return Array.isArray(value) && value.every((member) => isRecord(member) && exactKeys(member, memberKeys) &&
    validUser(member.user) && Number.isSafeInteger(member.role) && typeof member.joined_at === 'string')
}

function validUser(value: unknown): boolean {
  return isRecord(value) && exactKeys(value, userKeys) && typeof value.uuid === 'string' && identifier.test(value.uuid) &&
    typeof value.nickname === 'string' && typeof value.avatar === 'string' && typeof value.signature === 'string' &&
    Number.isSafeInteger(value.user_type) && Number.isSafeInteger(value.status)
}

function toUser(value: Record<string, unknown>): PublicUser {
  return value as unknown as PublicUser
}

function nonNegativeInteger(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) >= 0
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>): boolean {
  return Object.keys(value).every((key) => allowed.has(key))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
