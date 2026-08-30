import { describe, expect, it } from 'vitest'
import { parseDeviceSessions } from './devices'

const session = {
  connection_id: 'C100', device: 'desktop', device_id: 'web-current',
  connected_at: '2026-08-30T00:00:00.000Z', last_seen_at: '2026-08-30T00:01:00.000Z',
}

describe('device session projection', () => {
  it('accepts the action-safe public session projection', () => {
    expect(parseDeviceSessions([session])).toEqual([session])
  })

  it('rejects network and infrastructure metadata before it reaches the UI', () => {
    for (const key of ['remote_addr', 'user_agent', 'node_id', 'token']) {
      expect(() => parseDeviceSessions([{ ...session, [key]: 'sensitive' }])).toThrow('item')
    }
  })
})
