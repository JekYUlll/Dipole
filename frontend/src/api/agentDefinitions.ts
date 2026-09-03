import api from '@/api'

export interface AgentDefinitionCatalogItem {
  definitionId: string
  version: number
  agentId: string
  conversationScopes: string[]
  validFromUnixMs: number
  expiresAtUnixMs?: number
  createdAtUnixMs: number
  updatedAtUnixMs: number
}

export interface AgentDefinitionCatalogPage {
  definitions: AgentDefinitionCatalogItem[]
  nextCursor: string
}

export type AgentDefinitionCreateProfile = 'subscription_autoreply'

export interface AgentDefinitionCatalogClient {
  list(after?: string, limit?: number): Promise<AgentDefinitionCatalogPage>
  create?(profile: AgentDefinitionCreateProfile): Promise<AgentDefinitionCatalogItem>
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const cursor = /^[A-Za-z0-9_-]{1,384}$/
const itemKeys = new Set(['definitionId', 'version', 'agentId', 'conversationScopes', 'validFromUnixMs', 'expiresAtUnixMs', 'createdAtUnixMs', 'updatedAtUnixMs'])

export function parseAgentDefinitionCatalogPage(raw: unknown): AgentDefinitionCatalogPage {
  if (!isRecord(raw) || !exactKeys(raw, new Set(['definitions', 'nextCursor'])) || !Array.isArray(raw.definitions)) {
    throw new Error('Agent Definition catalog shape is invalid')
  }
  const nextCursor = raw.nextCursor === undefined || raw.nextCursor === '' ? '' : requireCursor(raw.nextCursor)
  return { definitions: raw.definitions.map(parseAgentDefinitionCatalogItem), nextCursor }
}

export function parseAgentDefinitionCatalogItem(raw: unknown): AgentDefinitionCatalogItem {
  if (!isRecord(raw) || !exactKeys(raw, itemKeys) || !Array.isArray(raw.conversationScopes) || raw.conversationScopes.length === 0 || raw.conversationScopes.length > 100) {
    throw new Error('Agent Definition catalog item is invalid')
  }
  const scopes = raw.conversationScopes.map(scope => {
    if (typeof scope !== 'string' || (scope !== '*' && !identifier.test(scope))) throw new Error('Agent Definition conversation scope is invalid')
    return scope
  })
  if (new Set(scopes).size !== scopes.length) throw new Error('Agent Definition conversation scope is duplicated')
  const item: AgentDefinitionCatalogItem = {
    definitionId: requireIdentifier(raw.definitionId),
    version: requirePositiveInteger(raw.version),
    agentId: requireIdentifier(raw.agentId),
    conversationScopes: scopes,
    validFromUnixMs: requirePositiveInteger(raw.validFromUnixMs),
    createdAtUnixMs: requirePositiveInteger(raw.createdAtUnixMs),
    updatedAtUnixMs: requirePositiveInteger(raw.updatedAtUnixMs),
  }
  if (raw.expiresAtUnixMs !== undefined) item.expiresAtUnixMs = requirePositiveInteger(raw.expiresAtUnixMs)
  return item
}

export const agentDefinitionCatalogClient: AgentDefinitionCatalogClient = {
  async list(after = '', limit = 50): Promise<AgentDefinitionCatalogPage> {
    if (after) requireCursor(after)
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Definition page limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after) query.set('after', after)
    return parseAgentDefinitionCatalogPage(await api.get(`/api/v1/agent/definitions?${query.toString()}`))
  },
  async create(profile: AgentDefinitionCreateProfile): Promise<AgentDefinitionCatalogItem> {
    if (profile !== 'subscription_autoreply') throw new Error('Agent Definition profile is unsupported')
    return parseAgentDefinitionCatalogItem(await api.post('/api/v1/agent/definitions', { profile }))
  },
}

function requireIdentifier(raw: unknown): string {
  if (typeof raw !== 'string' || !identifier.test(raw)) throw new Error('Agent Definition identity is invalid')
  return raw
}

function requireCursor(raw: unknown): string {
  if (typeof raw !== 'string' || !cursor.test(raw)) throw new Error('Agent Definition cursor is invalid')
  return raw
}

function requirePositiveInteger(raw: unknown): number {
  if (!Number.isSafeInteger(raw) || (raw as number) <= 0) throw new Error('Agent Definition number is invalid')
  return raw as number
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>): boolean {
  return Object.keys(value).every(key => allowed.has(key))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
