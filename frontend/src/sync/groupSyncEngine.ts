import type { Message } from '@/types'
import type { SyncDeliverySource } from './syncEngine'

export interface GroupSyncTarget {
  groupUUID: string
  latestMessageSeq: number
}

export interface LocalGroupSyncSnapshot {
  messageSeq: number
  messages: Message[]
}

export interface LocalGroupSyncStore {
  loadGroup(userUUID: string, groupUUID: string): Promise<LocalGroupSyncSnapshot>
  commitGroupPage(userUUID: string, groupUUID: string, messages: Message[], messageSeq: number): Promise<void>
}

export interface GroupSyncTransport {
  list(groupUUID: string, afterSeq: number, limit: number): Promise<Message[]>
  acknowledge(groupUUID: string, messageSeq: number): Promise<void>
}

export interface GroupSyncRecoveryResult {
  restored: number
  synchronized: number
  messageSeq: number
}

export class GroupMessageSyncEngine {
  private readonly pageSize: number
  private readonly maxPages: number

  constructor(
    private readonly store: LocalGroupSyncStore,
    private readonly transport: GroupSyncTransport,
    options: { pageSize?: number; maxPages?: number } = {},
  ) {
    this.pageSize = boundedInteger(options.pageSize, 100, 1, 200)
    this.maxPages = boundedInteger(options.maxPages, 1_000, 1, 10_000)
  }

  async recover(
    userUUID: string,
    target: GroupSyncTarget,
    deliver: (messages: Message[], source: SyncDeliverySource) => void,
  ): Promise<GroupSyncRecoveryResult> {
    const groupUUID = target.groupUUID.trim()
    if (!groupUUID || !Number.isSafeInteger(target.latestMessageSeq) || target.latestMessageSeq < 0) {
      throw new Error('group sync checkpoint is invalid')
    }

    const snapshot = await this.store.loadGroup(userUUID, groupUUID)
    if (!Number.isSafeInteger(snapshot.messageSeq) || snapshot.messageSeq < 0 || snapshot.messageSeq > target.latestMessageSeq) {
      throw new Error('local group sync cursor is invalid')
    }
    if (snapshot.messages.length > 0) deliver(snapshot.messages, 'local')

    let cursor = snapshot.messageSeq
    let synchronized = 0
    if (cursor > 0) await this.transport.acknowledge(groupUUID, cursor)

    for (let pageNumber = 0; cursor < target.latestMessageSeq; pageNumber += 1) {
      if (pageNumber >= this.maxPages) throw new Error('group sync page limit exceeded')
      const page = validateMessages(await this.transport.list(groupUUID, cursor, this.pageSize), cursor)
      if (page.length === 0) throw new Error('group sync page did not advance')

      const nextSeq = page[page.length - 1].message_seq!
      await this.store.commitGroupPage(userUUID, groupUUID, page, nextSeq)
      deliver(page, 'remote')
      synchronized += page.length
      cursor = nextSeq
      await this.transport.acknowledge(groupUUID, cursor)
    }

    return { restored: snapshot.messages.length, synchronized, messageSeq: cursor }
  }
}

function validateMessages(messages: Message[], afterSeq: number) {
  if (!Array.isArray(messages)) throw new Error('group sync response is invalid')
  let previous = afterSeq
  return messages.map(message => {
    const messageSeq = message?.message_seq
    if (!message?.message_id || !Number.isSafeInteger(messageSeq) || messageSeq! <= previous) {
      throw new Error('group sync messages are not strictly ordered')
    }
    previous = messageSeq!
    return message
  })
}

function boundedInteger(value: number | undefined, fallback: number, minimum: number, maximum: number) {
  const candidate = Number.isSafeInteger(value) ? value as number : fallback
  return Math.min(maximum, Math.max(minimum, candidate))
}
