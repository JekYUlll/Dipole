import api from '@/api'

export interface DeviceSession {
  connection_id: string
  device: string
  device_id?: string
  connected_at: string
  last_seen_at: string
}

export interface DeviceSessionClient {
  list(): Promise<DeviceSession[]>
  logout(connectionID: string): Promise<void>
  logoutOthers(): Promise<void>
}

const sessionKeys = new Set(['connection_id', 'device', 'device_id', 'connected_at', 'last_seen_at'])
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const deviceLabel = /^[A-Za-z0-9 _.-]{1,64}$/

export function parseDeviceSessions(raw: unknown): DeviceSession[] {
  if (!Array.isArray(raw)) throw new Error('device sessions shape is invalid')
  return raw.map(parseSession)
}

export const deviceSessionClient: DeviceSessionClient = {
  async list() {
    return parseDeviceSessions(await api.get('/api/v1/users/me/devices'))
  },
  async logout(connectionID: string) {
    if (!identifier.test(connectionID)) throw new Error('device connection identifier is invalid')
    await api.post(`/api/v1/users/me/devices/${encodeURIComponent(connectionID)}/logout`)
  },
  async logoutOthers() {
    await api.post('/api/v1/users/me/devices/logout-others')
  },
}

function parseSession(raw: unknown): DeviceSession {
  if (!isRecord(raw) || !Object.keys(raw).every(key => sessionKeys.has(key)) ||
      typeof raw.connection_id !== 'string' || !identifier.test(raw.connection_id) ||
      typeof raw.device !== 'string' || !deviceLabel.test(raw.device) ||
      (raw.device_id !== undefined && (typeof raw.device_id !== 'string' || !identifier.test(raw.device_id))) ||
      typeof raw.connected_at !== 'string' || !isTimestamp(raw.connected_at) ||
      typeof raw.last_seen_at !== 'string' || !isTimestamp(raw.last_seen_at)) {
    throw new Error('device session item is invalid')
  }
  return {
    connection_id: raw.connection_id,
    device: raw.device,
    ...(raw.device_id ? { device_id: raw.device_id } : {}),
    connected_at: raw.connected_at,
    last_seen_at: raw.last_seen_at,
  }
}

function isTimestamp(value: string) {
  return Number.isFinite(Date.parse(value))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
