import api from '@/api'

export interface DeviceSession {
  connection_id: string
  device: string
  device_id: string
  user_agent: string
  remote_addr: string
  node_id: string
  connected_at: string
  last_seen_at: string
}

export interface DeviceSessionClient {
  list(): Promise<DeviceSession[]>
  logout(connectionID: string): Promise<void>
  logoutAll(): Promise<void>
}

const keys = new Set(['connection_id', 'device', 'device_id', 'user_agent', 'remote_addr', 'node_id', 'connected_at', 'last_seen_at'])
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/

export function parseDeviceSessions(raw: unknown): DeviceSession[] {
  if (!Array.isArray(raw)) throw new Error('device session shape is invalid')
  return raw.map(parseDeviceSession)
}

export const deviceSessionClient: DeviceSessionClient = {
  async list() { return parseDeviceSessions(await api.get('/api/v1/users/me/devices')) },
  async logout(connectionID) { await api.post(`/api/v1/users/me/devices/${encodeURIComponent(connectionID)}/logout`) },
  async logoutAll() { await api.post('/api/v1/users/me/devices/logout-all') },
}

function parseDeviceSession(raw: unknown): DeviceSession {
  if (!isRecord(raw) || !Object.keys(raw).every(key => keys.has(key)) ||
      !validIdentifier(raw.connection_id) || !validDisplayString(raw.device) ||
      !validOptionalString(raw.device_id) || !validOptionalString(raw.user_agent) ||
      !validOptionalString(raw.remote_addr) || !validIdentifier(raw.node_id) ||
      !validString(raw.connected_at) || !validString(raw.last_seen_at)) {
    throw new Error('device session item is invalid')
  }
  return raw as unknown as DeviceSession
}

function validIdentifier(value: unknown): value is string { return typeof value === 'string' && identifier.test(value) }
function validDisplayString(value: unknown): value is string { return typeof value === 'string' && value.trim().length > 0 && value.length <= 128 }
function validString(value: unknown): value is string { return typeof value === 'string' && value.length > 0 && value.length <= 256 }
function validOptionalString(value: unknown): value is string { return value === undefined || (typeof value === 'string' && value.length <= 256) }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
