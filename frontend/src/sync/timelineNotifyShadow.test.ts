import { describe, expect, it, vi } from 'vitest'
import type { Message } from '@/types'
import { TimelineNotifyShadowVerifier, type TimelineNotification } from './timelineNotifyShadow'

const notification = (eventID: string, sequence: number, messageUUID = `M${sequence}`): TimelineNotification => ({
  schema_version: 'v1',
  event_id: eventID,
  message_uuid: messageUUID,
  conversation_key: 'direct:U1:U2',
  message_seq: sequence,
  target_type: 0,
  target_uuid: 'U2',
})

const message = (sequence: number, messageID = `M${sequence}`): Message => ({
  id: 0,
  message_id: messageID,
  message_seq: sequence,
  from_uuid: 'U1',
  target_uuid: 'U2',
  target_type: 0,
  message_type: 0,
  content: `message ${sequence}`,
  sent_at: '2026-08-27T00:00:00Z',
})

describe('TimelineNotifyShadowVerifier', () => {
  it('verifies the first notification from its preceding conversation sequence', async () => {
    const list = vi.fn(async () => [message(42)])
    const report = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list }, report)

    await verifier.observe(notification('E42', 42))

    expect(list).toHaveBeenCalledWith(expect.objectContaining({ message_seq: 42 }), 41, 100)
    expect(report).toHaveBeenCalledWith('match')
  })

  it('pulls the complete sequence gap when intermediate notifications are lost', async () => {
    const list = vi.fn(async (_notify: TimelineNotification, afterSeq: number) => {
      if (afterSeq === 39) return [message(40)]
      if (afterSeq === 40) return [message(41), message(42), message(43)]
      return []
    })
    const report = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list }, report)

    await verifier.observe(notification('E40', 40))
    await verifier.observe(notification('E43', 43))

    expect(list).toHaveBeenNthCalledWith(2, expect.objectContaining({ message_seq: 43 }), 40, 100)
    expect(report).toHaveBeenCalledTimes(2)
    expect(report).toHaveBeenLastCalledWith('match')
  })

  it('delivers a complete verified gap only after the target sequence matches', async () => {
    const list = vi.fn(async (_notify: TimelineNotification, afterSeq: number) => {
      if (afterSeq === 39) return [message(40)]
      return [message(41), message(42)]
    })
    const report = vi.fn()
    const deliver = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list }, report, deliver)

    await verifier.observe(notification('E40', 40))
    await verifier.observe(notification('E42', 42))

    expect(deliver).toHaveBeenLastCalledWith([message(41), message(42)])
    expect(report).toHaveBeenCalledWith('match')
  })

  it('does not deliver a page when the target UUID conflicts', async () => {
    const deliver = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list: vi.fn(async () => [message(42, 'OTHER')]) }, vi.fn(), deliver)

    await verifier.observe(notification('E42', 42))

    expect(deliver).not.toHaveBeenCalled()
  })

  it('deduplicates replayed and stale notifications without moving the verified cursor backwards', async () => {
    const list = vi.fn(async () => [message(42)])
    const report = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list }, report)

    await verifier.observe(notification('E42', 42))
    await verifier.observe(notification('E42', 42))
    await verifier.observe(notification('E41-replay', 41))

    expect(list).toHaveBeenCalledTimes(1)
    expect(report).toHaveBeenCalledTimes(1)
  })

  it.each([
    { name: 'missing target', page: [], notify: notification('E43', 43), outcome: 'missing' },
    { name: 'uuid mismatch', page: [message(42, 'OTHER')], notify: notification('E42', 42), outcome: 'mismatch' },
  ])('reports $name without accepting the target sequence', async ({ page, notify, outcome }) => {
    const report = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list: vi.fn(async () => page) }, report)

    await verifier.observe(notify)

    expect(report).toHaveBeenCalledWith(outcome)
  })

  it('contains transport failures inside the shadow path', async () => {
    const report = vi.fn()
    const verifier = new TimelineNotifyShadowVerifier({ list: vi.fn(async () => { throw new Error('offline') }) }, report)

    await expect(verifier.observe(notification('E42', 42))).resolves.toBeUndefined()
    expect(report).toHaveBeenCalledWith('error')
  })
})
