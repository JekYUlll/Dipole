import api from '@/api'

export interface OwnedFileDirectoryItem {
  file_id: string
  file_name: string
  file_size: number
  content_type: string
  created_at: string
  download_path: string
}

export interface OwnedFileDirectoryPage {
  files: OwnedFileDirectoryItem[]
  next_cursor?: string
  has_more: boolean
}

export interface OwnedFileDirectoryClient {
  list(cursor?: string): Promise<OwnedFileDirectoryPage>
  download(fileID: string): Promise<string>
}

const itemKeys = new Set(['file_id', 'file_name', 'file_size', 'content_type', 'created_at', 'download_path'])
const pageKeys = new Set(['files', 'next_cursor', 'has_more'])
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/

export function parseOwnedFileDirectoryPage(raw: unknown): OwnedFileDirectoryPage {
  if (!isRecord(raw) || !exactKeys(raw, pageKeys) || !Array.isArray(raw.files) || typeof raw.has_more !== 'boolean' ||
      (raw.next_cursor !== undefined && (typeof raw.next_cursor !== 'string' || !identifier.test(raw.next_cursor)))) {
    throw new Error('file directory shape is invalid')
  }
  return {
    files: raw.files.map(parseItem),
    ...(raw.next_cursor ? { next_cursor: raw.next_cursor } : {}),
    has_more: raw.has_more,
  }
}

export const ownedFileDirectoryClient: OwnedFileDirectoryClient = {
  async list(cursor?: string): Promise<OwnedFileDirectoryPage> {
    const params = new URLSearchParams({ limit: '30' })
    if (cursor) params.set('cursor', cursor)
    return parseOwnedFileDirectoryPage(await api.get(`/api/v1/files?${params.toString()}`))
  },
  async download(fileID: string): Promise<string> {
    if (!identifier.test(fileID)) throw new Error('file identifier is invalid')
    const raw = await api.get(`/api/v1/files/${encodeURIComponent(fileID)}/download`)
    if (!isRecord(raw) || typeof raw.download_url !== 'string' || raw.download_url.trim() === '') {
      throw new Error('file download response is invalid')
    }
    return raw.download_url
  },
}

function parseItem(raw: unknown): OwnedFileDirectoryItem {
  if (!isRecord(raw) || !exactKeys(raw, itemKeys) || typeof raw.file_id !== 'string' || !identifier.test(raw.file_id) ||
      typeof raw.file_name !== 'string' || raw.file_name.trim() === '' || typeof raw.file_size !== 'number' ||
      !Number.isSafeInteger(raw.file_size) || raw.file_size < 0 || typeof raw.content_type !== 'string' ||
      typeof raw.created_at !== 'string' || !Number.isFinite(Date.parse(raw.created_at)) ||
      typeof raw.download_path !== 'string' || raw.download_path !== `/api/v1/files/${raw.file_id}/download`) {
    throw new Error('file directory item is invalid')
  }
  return raw as OwnedFileDirectoryItem
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>) {
  return Object.keys(value).every(key => allowed.has(key))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
