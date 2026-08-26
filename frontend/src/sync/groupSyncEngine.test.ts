import { describe, expect, it, vi } from 'vitest'
import type { Message } from '@/types'
import {
  GroupMessageSyncEngine,
  type LocalGroupSyncSnapshot,
  type LocalGroupSyncStore,
  type GroupSyncTransport,
} from './groupSyncEngine'

const message = (seq: number): Message => ({
  id: 0,
  message_id: `M${seq}`,
  message_seq: seq,
  from_uuid: 'U2',
  target_uuid: 'G1',
  target_type: 1,
  message_type: 0,
  content: `M${seq}`,
  sent_at: '2026-08-27T12:00:00Z',
})

class MemoryGroupStore implements LocalGroupSyncStore {
  constructor(
    public snapshot: LocalGroupSyncSnapshot,
    private readonly events: string[],
  ) {}

  async loadGroup() {
    this.events.push('load')
    return this.snapshot
  }

  async commitGroupPage(_userUUID: string, _groupUUID: string, messages: Message[], messageSeq: number) {
    this.events.push(`commit:${messageSeq}`)
    this.snapshot = { messageSeq, messages: [...this.snapshot.messages, ...messages] }
  }
}

describe('GroupMessageSyncEngine', () => {
  it('restores local messages and acknowledges only after each durable group page', async () => {
    const events: string[] = []
    const store = new MemoryGroupStore({ messageSeq: 2, messages: [message(2)] }, events)
    const transport: GroupSyncTransport = {
      list: vi.fn(async (_groupUUID, afterSeq) => {
        events.push(`list:${afterSeq}`)
        return afterSeq === 2 ? [message(3), message(4)] : []
      }),
      acknowledge: vi.fn(async (_groupUUID, messageSeq) => { events.push(`ack:${messageSeq}`) }),
    }

    const result = await new GroupMessageSyncEngine(store, transport).recover(
      'U1',
      { groupUUID: 'G1', latestMessageSeq: 4 },
      (messages, source) => events.push(`deliver:${source}:${messages.map(item => item.message_id).join(',')}`),
    )

    expect(result).toEqual({ restored: 1, synchronized: 2, messageSeq: 4 })
    expect(events).toEqual([
      'load',
      'deliver:local:M2',
      'ack:2',
      'list:2',
      'commit:4',
      'deliver:remote:M3,M4',
      'ack:4',
    ])
  })

  it('does not expose or acknowledge a group page when local persistence fails', async () => {
    const store = new MemoryGroupStore({ messageSeq: 0, messages: [] }, [])
    store.commitGroupPage = vi.fn(async () => { throw new Error('quota exceeded') })
    const transport: GroupSyncTransport = {
      list: vi.fn(async () => [message(1)]),
      acknowledge: vi.fn(async () => {}),
    }
    const deliver = vi.fn()

    await expect(new GroupMessageSyncEngine(store, transport).recover(
      'U1',
      { groupUUID: 'G1', latestMessageSeq: 1 },
      deliver,
    )).rejects.toThrow('quota exceeded')
    expect(deliver).not.toHaveBeenCalled()
    expect(transport.acknowledge).not.toHaveBeenCalled()
  })

  it('rejects an empty or non-advancing page before acknowledging the checkpoint', async () => {
    const store = new MemoryGroupStore({ messageSeq: 3, messages: [] }, [])
    const transport: GroupSyncTransport = {
      list: vi.fn(async () => []),
      acknowledge: vi.fn(async () => {}),
    }

    await expect(new GroupMessageSyncEngine(store, transport).recover(
      'U1',
      { groupUUID: 'G1', latestMessageSeq: 4 },
      () => {},
    )).rejects.toThrow('did not advance')
    expect(transport.acknowledge).toHaveBeenCalledTimes(1)
    expect(transport.acknowledge).toHaveBeenCalledWith('G1', 3)
  })

  it('rejects unordered message sequences before local persistence', async () => {
    const store = new MemoryGroupStore({ messageSeq: 0, messages: [] }, [])
    const transport: GroupSyncTransport = {
      list: vi.fn(async () => [message(2), message(1)]),
      acknowledge: vi.fn(async () => {}),
    }

    await expect(new GroupMessageSyncEngine(store, transport).recover(
      'U1',
      { groupUUID: 'G1', latestMessageSeq: 2 },
      () => {},
    )).rejects.toThrow('strictly ordered')
    expect(store.snapshot.messageSeq).toBe(0)
    expect(transport.acknowledge).not.toHaveBeenCalled()
  })
})
