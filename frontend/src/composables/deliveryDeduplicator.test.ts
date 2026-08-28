import { describe, expect, it } from 'vitest'
import { DeliveryDeduplicator, acceptDeliveryPacket } from './useWebSocket'

describe('DeliveryDeduplicator', () => {
  it('allows legacy packets and suppresses repeated stable delivery ids', () => {
    const deduplicator = new DeliveryDeduplicator(2)

    expect(deduplicator.accept({ type: 'legacy', data: {} })).toBe(true)
    expect(deduplicator.accept({ type: 'message', delivery_id: 'D1', data: {} })).toBe(true)
    expect(deduplicator.accept({ type: 'message', delivery_id: 'D1', data: {} })).toBe(false)
  })

  it('uses bounded fifo retention for replay ids', () => {
    const deduplicator = new DeliveryDeduplicator(2)

    expect(deduplicator.accept({ type: 'message', delivery_id: 'D1', data: {} })).toBe(true)
    expect(deduplicator.accept({ type: 'message', delivery_id: 'D2', data: {} })).toBe(true)
    expect(deduplicator.accept({ type: 'message', delivery_id: 'D3', data: {} })).toBe(true)
    expect(deduplicator.accept({ type: 'message', delivery_id: 'D1', data: {} })).toBe(true)
  })

  it('rejects unsafe capacity', () => {
    expect(() => new DeliveryDeduplicator(0)).toThrow(/capacity/)
  })

  it('checks stable ids against the persistent user-scoped replay store', async () => {
    const claimed = new Set<string>()
    const claim = async (userUUID: string, deliveryID: string) => {
      const key = `${userUUID}:${deliveryID}`
      if (claimed.has(key)) return false
      claimed.add(key)
      return true
    }

    await expect(acceptDeliveryPacket(
      new DeliveryDeduplicator(2),
      { type: 'message', delivery_id: 'D1', data: {} },
      'U1',
      claim,
    )).resolves.toBe(true)
    await expect(acceptDeliveryPacket(
      new DeliveryDeduplicator(2),
      { type: 'message', delivery_id: 'D1', data: {} },
      'U1',
      claim,
    )).resolves.toBe(false)
  })

  it('fails open when persistent replay storage is temporarily unavailable', async () => {
    await expect(acceptDeliveryPacket(
      new DeliveryDeduplicator(2),
      { type: 'message', delivery_id: 'D1', data: {} },
      'U1',
      async () => { throw new Error('IndexedDB unavailable') },
    )).resolves.toBe(true)
  })
})
