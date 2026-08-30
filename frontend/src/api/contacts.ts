import api from '@/api'
import type { Contact, PublicUser } from '@/types'

export interface ContactDirectoryClient {
  list(): Promise<Contact[]>
}

const userKeys = new Set(['uuid', 'nickname', 'avatar', 'signature', 'user_type', 'status'])
const contactKeys = new Set(['user', 'remark', 'status'])
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/

export function parseContactList(raw: unknown): Contact[] {
  if (!Array.isArray(raw)) throw new Error('contact directory shape is invalid')
  return raw.map(parseContact)
}

export const contactDirectoryClient: ContactDirectoryClient = {
  async list(): Promise<Contact[]> {
    return parseContactList(await api.get('/api/v1/contacts'))
  },
}

function parseContact(raw: unknown): Contact {
  if (!isRecord(raw) || !exactKeys(raw, contactKeys) || typeof raw.remark !== 'string' || !validStatus(raw.status)) {
    throw new Error('contact directory item is invalid')
  }
  return { user: parsePublicUser(raw.user), remark: raw.remark, status: raw.status }
}

function parsePublicUser(raw: unknown): PublicUser {
  if (!isRecord(raw) || !exactKeys(raw, userKeys) || typeof raw.uuid !== 'string' || !identifier.test(raw.uuid) ||
      typeof raw.nickname !== 'string' || typeof raw.avatar !== 'string' || typeof raw.signature !== 'string' ||
      !Number.isSafeInteger(raw.user_type) || !Number.isSafeInteger(raw.status)) {
    throw new Error('contact user projection is invalid')
  }
  return {
    uuid: raw.uuid, nickname: raw.nickname, avatar: raw.avatar, signature: raw.signature,
    user_type: raw.user_type as number, status: raw.status as number,
  }
}

function validStatus(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value === 0 || value === 1)
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>): boolean {
  return Object.keys(value).every(key => allowed.has(key))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
