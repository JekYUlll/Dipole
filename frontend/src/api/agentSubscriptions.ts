import api from '@/api'

export type AgentSubscriptionStatus = 'active' | 'revoked'
export type AgentSubscriptionFilterKind = 'all' | 'message_contains_any'

export interface AgentSubscription {
  subscriptionId: string
  definitionId: string
  definitionVersion: number
  agentId: string
  eventType: string
  resourceType: 'conversation'
  resourceId: string
  filterKind: AgentSubscriptionFilterKind
  filter: { terms?: string[] }
  status: AgentSubscriptionStatus
  createdById: string
  revokedById?: string
  revokeReason?: string
  createdAtUnixMs: number
  updatedAtUnixMs: number
  revokedAtUnixMs?: number
}

export interface AgentSubscriptionPage {
  subscriptions: AgentSubscription[]
  nextCursor: string
}

export interface AgentSubscriptionClient {
  list(after?: string, limit?: number): Promise<AgentSubscriptionPage>
  revoke(subscriptionId: string, reason: string): Promise<AgentSubscription>
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const itemKeys = new Set([
  'subscriptionId', 'definitionId', 'definitionVersion', 'agentId', 'eventType', 'resourceType', 'resourceId',
  'filterKind', 'filter', 'status', 'createdById', 'revokedById', 'revokeReason', 'createdAtUnixMs',
  'updatedAtUnixMs', 'revokedAtUnixMs',
])

export function parseAgentSubscriptionPage(raw: unknown): AgentSubscriptionPage {
  if (!isRecord(raw) || !exactKeys(raw, new Set(['subscriptions', 'nextCursor'])) || !Array.isArray(raw.subscriptions)) {
    throw new Error('Agent Subscription page shape is invalid')
  }
  const nextCursor = raw.nextCursor === undefined || raw.nextCursor === '' ? '' : requireIdentifier(raw.nextCursor, 'cursor')
  return { subscriptions: raw.subscriptions.map(parseAgentSubscriptionResponse), nextCursor }
}

export function parseAgentSubscriptionResponse(raw: unknown): AgentSubscription {
  if (!isRecord(raw) || !exactKeys(raw, itemKeys)) throw new Error('Agent Subscription response shape is invalid')
  const status = raw.status
  if (status !== 'active' && status !== 'revoked') throw new Error('Agent Subscription status is invalid')
  const filterKind = raw.filterKind
  if (filterKind !== 'all' && filterKind !== 'message_contains_any') throw new Error('Agent Subscription filter kind is invalid')
  const filter = parseFilter(filterKind, raw.filter)
  const definitionVersion = requirePositiveInteger(raw.definitionVersion, 'Definition version')
  const createdAtUnixMs = requireNonNegativeInteger(raw.createdAtUnixMs, 'created time')
  const updatedAtUnixMs = requireNonNegativeInteger(raw.updatedAtUnixMs, 'updated time')
  const optionalAudit = {
    revokedById: optionalIdentifier(raw.revokedById, 'revoker'),
    revokeReason: optionalReason(raw.revokeReason),
    revokedAtUnixMs: optionalPositiveInteger(raw.revokedAtUnixMs, 'revoked time'),
  }
  if (status === 'active' && Object.values(optionalAudit).some(value => value !== undefined)) {
    throw new Error('Agent Subscription active audit state is invalid')
  }
  if (status === 'revoked' && Object.values(optionalAudit).some(value => value === undefined)) {
    throw new Error('Agent Subscription revoked audit state is invalid')
  }
  if (raw.resourceType !== 'conversation') throw new Error('Agent Subscription resource type is invalid')
  return {
    subscriptionId: requireIdentifier(raw.subscriptionId, 'identity'),
    definitionId: requireIdentifier(raw.definitionId, 'Definition'),
    definitionVersion,
    agentId: requireIdentifier(raw.agentId, 'Agent'),
    eventType: requireIdentifier(raw.eventType, 'event'),
    resourceType: 'conversation',
    resourceId: requireIdentifier(raw.resourceId, 'resource'),
    filterKind,
    filter,
    status,
    createdById: requireIdentifier(raw.createdById, 'creator'),
    createdAtUnixMs,
    updatedAtUnixMs,
    ...optionalAudit,
  }
}

export const agentSubscriptionClient: AgentSubscriptionClient = {
  async list(after = '', limit = 50) {
    if (after) requireIdentifier(after, 'cursor')
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Subscription page limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after) query.set('after', after)
    return parseAgentSubscriptionPage(await api.get(`/api/v1/agent/subscriptions?${query.toString()}`))
  },
  async revoke(subscriptionId, reason) {
    requireIdentifier(subscriptionId, 'identity')
    const normalized = reason.trim()
    if (!normalized || [...normalized].length > 1000 || /[\u0000-\u001f\u007f]/u.test(normalized)) {
      throw new Error('Agent Subscription revoke reason is invalid')
    }
    return parseAgentSubscriptionResponse(await api.post(
      `/api/v1/agent/subscriptions/${encodeURIComponent(subscriptionId)}/revoke`, { reason: normalized },
    ))
  },
}

function parseFilter(kind: AgentSubscriptionFilterKind, raw: unknown): { terms?: string[] } {
  if (!isRecord(raw)) throw new Error('Agent Subscription filter is invalid')
  if (kind === 'all') {
    if (Object.keys(raw).length !== 0) throw new Error('Agent Subscription all filter is invalid')
    return {}
  }
  if (!exactKeys(raw, new Set(['terms'])) || !Array.isArray(raw.terms) || raw.terms.length < 1 || raw.terms.length > 32 ||
      raw.terms.some(term => typeof term !== 'string' || term.trim() !== term || term.length === 0 || [...term].length > 64 || /[\u0000-\u001f\u007f]/u.test(term)) ||
      new Set(raw.terms).size !== raw.terms.length) throw new Error('Agent Subscription terms filter is invalid')
  return { terms: [...raw.terms] as string[] }
}

function requireIdentifier(raw: unknown, label: string): string {
  if (typeof raw !== 'string' || !identifier.test(raw)) throw new Error(`Agent Subscription ${label} is invalid`)
  return raw
}

function optionalIdentifier(raw: unknown, label: string): string | undefined {
  return raw === undefined ? undefined : requireIdentifier(raw, label)
}

function optionalReason(raw: unknown): string | undefined {
  if (raw === undefined) return undefined
  if (typeof raw !== 'string' || raw.trim().length === 0 || [...raw].length > 1000 || /[\u0000-\u001f\u007f]/u.test(raw)) {
    throw new Error('Agent Subscription revoke reason is invalid')
  }
  return raw
}

function requirePositiveInteger(raw: unknown, label: string): number {
  if (!Number.isSafeInteger(raw) || (raw as number) <= 0) throw new Error(`Agent Subscription ${label} is invalid`)
  return raw as number
}

function requireNonNegativeInteger(raw: unknown, label: string): number {
  if (!Number.isSafeInteger(raw) || (raw as number) < 0) throw new Error(`Agent Subscription ${label} is invalid`)
  return raw as number
}

function optionalPositiveInteger(raw: unknown, label: string): number | undefined {
  return raw === undefined ? undefined : requirePositiveInteger(raw, label)
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>): boolean {
  return Object.keys(value).every(key => allowed.has(key))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
