import api from '@/api'

export type AgentMemoryType = 'working' | 'episodic' | 'semantic' | 'procedural' | 'observational'
export type AgentMemoryStatus = 'active' | 'revoked'

export interface AgentMemory {
  memoryId: string
  agentId: string
  memoryType: AgentMemoryType
  status: AgentMemoryStatus
  resourceType: string
  resourceId: string
  content: string
  compactContent?: string
  priority: number
  provenance: { sourceType: string, sourceId: string, sequence?: string }
  validFromUnixMs: number
  expiresAtUnixMs?: number
  revokedAtUnixMs?: number
  revokedById?: string
  revokeReason?: string
  createdAtUnixMs: number
}

export interface AgentMemoryPage {
  memories: AgentMemory[]
  nextCursor: string
}

export interface AgentMemoryClient {
  list(after?: string, limit?: number): Promise<AgentMemoryPage>
  revoke(memoryId: string, reason: string): Promise<AgentMemory>
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const cursor = /^[A-Za-z0-9_-]{1,256}$/
const memoryTypes = new Set<AgentMemoryType>(['working', 'episodic', 'semantic', 'procedural', 'observational'])
const memoryKeys = new Set([
  'memoryId', 'agentId', 'memoryType', 'status', 'resourceType', 'resourceId', 'content', 'compactContent',
  'priority', 'provenance', 'validFromUnixMs', 'expiresAtUnixMs', 'revokedAtUnixMs', 'revokedById',
  'revokeReason', 'createdAtUnixMs',
])

export function parseAgentMemoryPage(raw: unknown): AgentMemoryPage {
  if (!isRecord(raw) || !exactKeys(raw, new Set(['memories', 'nextCursor'])) || !Array.isArray(raw.memories)) {
    throw new Error('Agent Memory page shape is invalid')
  }
  const nextCursor = raw.nextCursor === undefined || raw.nextCursor === '' ? '' : requireCursor(raw.nextCursor)
  return { memories: raw.memories.map(parseAgentMemoryResponse), nextCursor }
}

export function parseAgentMemoryResponse(raw: unknown): AgentMemory {
  if (!isRecord(raw) || !exactKeys(raw, memoryKeys)) throw new Error('Agent Memory response shape is invalid')
  if (!memoryTypes.has(raw.memoryType as AgentMemoryType)) throw new Error('Agent Memory type is invalid')
  if (raw.status !== 'active' && raw.status !== 'revoked') throw new Error('Agent Memory status is invalid')
  if (typeof raw.content !== 'string' || raw.content.trim() === '' || byteLength(raw.content) > 16 * 1024) throw new Error('Agent Memory content is invalid')
  if (raw.compactContent !== undefined && (typeof raw.compactContent !== 'string' || byteLength(raw.compactContent) > 4 * 1024)) throw new Error('Agent Memory compact content is invalid')
  if (!Number.isInteger(raw.priority) || (raw.priority as number) < 0 || (raw.priority as number) > 1000) throw new Error('Agent Memory priority is invalid')
  if (!isRecord(raw.provenance) || !exactKeys(raw.provenance, new Set(['sourceType', 'sourceId', 'sequence']))) throw new Error('Agent Memory provenance is invalid')
  const validFromUnixMs = positiveInteger(raw.validFromUnixMs, 'valid time')
  const createdAtUnixMs = positiveInteger(raw.createdAtUnixMs, 'created time')
  const expiresAtUnixMs = optionalPositiveInteger(raw.expiresAtUnixMs, 'expiry time')
  if (expiresAtUnixMs !== undefined && expiresAtUnixMs <= validFromUnixMs) throw new Error('Agent Memory expiry is invalid')
  const audit = {
    revokedAtUnixMs: optionalPositiveInteger(raw.revokedAtUnixMs, 'revoked time'),
    revokedById: optionalIdentifier(raw.revokedById, 'revoker'),
    revokeReason: optionalReason(raw.revokeReason),
  }
  if (raw.status === 'active' && Object.values(audit).some(value => value !== undefined)) throw new Error('Agent Memory active audit is invalid')
  if (raw.status === 'revoked' && Object.values(audit).some(value => value === undefined)) throw new Error('Agent Memory revoked audit is invalid')
  return {
    memoryId: requireIdentifier(raw.memoryId, 'identity'),
    agentId: requireIdentifier(raw.agentId, 'Agent'),
    memoryType: raw.memoryType as AgentMemoryType,
    status: raw.status,
    resourceType: requireIdentifier(raw.resourceType, 'resource type'),
    resourceId: requireIdentifier(raw.resourceId, 'resource'),
    content: raw.content,
    compactContent: raw.compactContent as string | undefined,
    priority: raw.priority as number,
    provenance: {
      sourceType: requireIdentifier(raw.provenance.sourceType, 'source type'),
      sourceId: requireIdentifier(raw.provenance.sourceId, 'source'),
      sequence: optionalIdentifier(raw.provenance.sequence, 'source sequence'),
    },
    validFromUnixMs, expiresAtUnixMs, createdAtUnixMs, ...audit,
  }
}

export const agentMemoryClient: AgentMemoryClient = {
  async list(after = '', limit = 50) {
    if (after) requireCursor(after)
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Memory page limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after) query.set('after', after)
    return parseAgentMemoryPage(await api.get(`/api/v1/agent/memories?${query.toString()}`))
  },
  async revoke(memoryId, reason) {
    requireIdentifier(memoryId, 'identity')
    const normalized = reason.trim()
    if (!normalized || [...normalized].length > 1000 || /[\u0000-\u001f\u007f]/u.test(normalized)) throw new Error('Agent Memory revoke reason is invalid')
    return parseAgentMemoryResponse(await api.post(`/api/v1/agent/memories/${encodeURIComponent(memoryId)}/revoke`, { reason: normalized }))
  },
}

function requireCursor(raw: unknown): string {
  if (typeof raw !== 'string' || !cursor.test(raw)) throw new Error('Agent Memory cursor is invalid')
  return raw
}

function requireIdentifier(raw: unknown, label: string): string {
  if (typeof raw !== 'string' || !identifier.test(raw)) throw new Error(`Agent Memory ${label} is invalid`)
  return raw
}

function optionalIdentifier(raw: unknown, label: string): string | undefined {
  return raw === undefined ? undefined : requireIdentifier(raw, label)
}

function optionalReason(raw: unknown): string | undefined {
  if (raw === undefined) return undefined
  if (typeof raw !== 'string' || raw.trim() !== raw || !raw || [...raw].length > 1000 || /[\u0000-\u001f\u007f]/u.test(raw)) throw new Error('Agent Memory revoke reason is invalid')
  return raw
}

function positiveInteger(raw: unknown, label: string): number {
  if (!Number.isSafeInteger(raw) || (raw as number) <= 0) throw new Error(`Agent Memory ${label} is invalid`)
  return raw as number
}

function optionalPositiveInteger(raw: unknown, label: string): number | undefined {
  return raw === undefined ? undefined : positiveInteger(raw, label)
}

function byteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>): boolean {
  return Object.keys(value).every(key => allowed.has(key))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
