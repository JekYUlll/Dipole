import type { Message } from '@/types'

export interface SyncItem {
  sync_seq: number
  conversation_key: string
  message_uuid: string
  message_seq: number
  message: Message
}

export interface SyncPage {
  items: SyncItem[]
  next_seq: number
  has_more: boolean
}

export interface LocalSyncSnapshot {
  syncSeq: number
  messages: Message[]
}

export interface LocalSyncStore {
  load(userUUID: string): Promise<LocalSyncSnapshot>
  commitPage(userUUID: string, page: SyncPage): Promise<void>
  clearUser(userUUID: string): Promise<void>
}

export interface SyncTransport {
  list(afterSeq: number, limit: number): Promise<SyncPage>
  acknowledge(syncSeq: number): Promise<void>
}

export interface SyncRecoveryResult {
  restored: number
  synchronized: number
  syncSeq: number
}

export type SyncDeliverySource = 'local' | 'remote'

export class MessageSyncEngine {
  private readonly pageSize: number

  constructor(
    private readonly store: LocalSyncStore,
    private readonly transport: SyncTransport,
    options: { pageSize?: number } = {},
  ) {
    this.pageSize = Math.min(200, Math.max(1, options.pageSize ?? 100))
  }

  async recover(userUUID: string, deliver: (messages: Message[], source: SyncDeliverySource) => void): Promise<SyncRecoveryResult> {
    const snapshot = await this.store.load(userUUID)
    if (snapshot.messages.length > 0) deliver(snapshot.messages, 'local')

    let cursor = snapshot.syncSeq
    let synchronized = 0

    // A previous run may have committed locally and then lost its ACK response.
    if (cursor > 0) await this.transport.acknowledge(cursor)

    while (true) {
      const rawPage = await this.transport.list(cursor, this.pageSize)
      const page = validatePage(rawPage, cursor)
      if (page.items.length > 0) {
        await this.store.commitPage(userUUID, page)
        deliver(page.items.map(item => item.message), 'remote')
        synchronized += page.items.length
        cursor = page.next_seq
        await this.transport.acknowledge(cursor)
      }
      if (!page.has_more) break
    }

    return { restored: snapshot.messages.length, synchronized, syncSeq: cursor }
  }
}

function validatePage(page: SyncPage, afterSeq: number): SyncPage {
  if (!page || !Array.isArray(page.items)) throw new Error('sync response is invalid')
  if (!Number.isSafeInteger(page.next_seq) || page.next_seq < afterSeq) {
    throw new Error('sync cursor cannot move backwards')
  }
  if (page.has_more && page.next_seq <= afterSeq) {
    throw new Error('sync page did not advance')
  }

  let previous = afterSeq
  const items = page.items.map(item => {
    if (!item?.message || !item.message_uuid) throw new Error('sync item is invalid')
    if (!Number.isSafeInteger(item.sync_seq) || item.sync_seq <= previous) {
      throw new Error('sync items are not strictly ordered')
    }
    previous = item.sync_seq
    return {
      ...item,
      message: {
        ...item.message,
        message_id: item.message_uuid,
        message_seq: item.message_seq,
      },
    }
  })

  if (items.length > 0 && page.next_seq !== items[items.length - 1].sync_seq) {
    throw new Error('sync page cursor does not match its last item')
  }
  return { ...page, items }
}
