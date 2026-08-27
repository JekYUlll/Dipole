import { describe, expect, it } from 'vitest'
import { DeliveryDeduplicator } from './useWebSocket'

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
})
