import { describe, expect, it } from 'vitest'
import { parseDeviceSessions } from './devices'

const session = {
  connection_id: 'C100', device: 'browser', device_id: 'D100', user_agent: 'Chrome',
  remote_addr: '10.0.0.1', node_id: 'N1', connected_at: '2026-09-01T10:00:00Z', last_seen_at: '2026-09-01T10:01:00Z',
}

describe('parseDeviceSessions', () => {
  it('accepts the owner-scoped session projection', () => { expect(parseDeviceSessions([session])).toEqual([session]) })
  it('rejects unknown fields and missing identity', () => {
    expect(() => parseDeviceSessions([{ ...session, principal: 'U1' }])).toThrow()
    expect(() => parseDeviceSessions([{ ...session, connection_id: '' }])).toThrow()
  })
  it('accepts omitted optional device metadata', () => {
    const { device_id: _deviceID, user_agent: _agent, remote_addr: _addr, ...minimal } = session
    expect(parseDeviceSessions([minimal])[0].device_id).toBeUndefined()
  })
})
