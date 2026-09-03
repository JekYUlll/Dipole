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
  memoryRootId: string
  memoryVersion: number
  supersedesMemoryId?: string
  correctedById?: string
  correctionReason?: string
}

export interface AgentMemoryPage {
  memories: AgentMemory[]
  nextCursor: string
}

export type AgentMemoryCandidateStatus = 'pending' | 'accepted' | 'rejected'

export interface AgentMemoryCandidate {
  candidateId: string
  candidateSha256: string
  summary: string
  status: AgentMemoryCandidateStatus
  reviewId?: string
  promotedMemoryId?: string
  observedAtUnixMs: number
}

export interface AgentMemoryCandidatePage {
  candidates: AgentMemoryCandidate[]
  nextCursor: string
}

export interface AgentMemoryClient {
  list(after?: string, limit?: number): Promise<AgentMemoryPage>
  listCandidates(after?: string, limit?: number): Promise<AgentMemoryCandidatePage>
  revoke(memoryId: string, reason: string): Promise<AgentMemory>
  correct(memoryId: string, expectedVersion: number, content: string, compactContent: string, reason: string): Promise<AgentMemoryCorrection>
  promoteCandidate(candidateId: string, candidateSha256: string, reviewId: string, targetMemoryType?: AgentMemoryType): Promise<AgentMemory>
  reviewCandidate(candidateId: string, candidateSha256: string, decision: 'accepted' | 'rejected', reason: string): Promise<AgentMemoryCandidate>
}

export interface AgentMemoryCorrection {
  previous: AgentMemory
  corrected: AgentMemory
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const cursor = /^[A-Za-z0-9_-]{1,256}$/
const memoryTypes = new Set<AgentMemoryType>(['working', 'episodic', 'semantic', 'procedural', 'observational'])
const memoryKeys = new Set([
  'memoryId', 'agentId', 'memoryType', 'status', 'resourceType', 'resourceId', 'content', 'compactContent',
  'priority', 'provenance', 'validFromUnixMs', 'expiresAtUnixMs', 'revokedAtUnixMs', 'revokedById',
  'revokeReason', 'createdAtUnixMs', 'memoryRootId', 'memoryVersion', 'supersedesMemoryId', 'correctedById', 'correctionReason',
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
  const memoryId = requireIdentifier(raw.memoryId, 'identity')
  const memoryRootId = requireIdentifier(raw.memoryRootId, 'root identity')
  const memoryVersion = positiveInteger(raw.memoryVersion, 'version')
  const supersedesMemoryId = optionalIdentifier(raw.supersedesMemoryId, 'predecessor')
  const correctedById = optionalIdentifier(raw.correctedById, 'corrector')
  const correctionReason = optionalReason(raw.correctionReason, 'correction')
  if (memoryVersion === 1) {
    if (memoryRootId !== memoryId || supersedesMemoryId !== undefined || correctedById !== undefined || correctionReason !== undefined) throw new Error('Agent Memory root lineage is invalid')
  } else if (supersedesMemoryId === undefined || correctedById === undefined || correctionReason === undefined ||
    raw.provenance.sourceType !== 'owner_correction' || raw.provenance.sourceId !== supersedesMemoryId || raw.provenance.sequence !== String(memoryVersion)) {
    throw new Error('Agent Memory correction lineage is invalid')
  }
  return {
    memoryId,
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
    validFromUnixMs, expiresAtUnixMs, createdAtUnixMs, memoryRootId, memoryVersion,
    supersedesMemoryId, correctedById, correctionReason, ...audit,
  }
}

export function parseAgentMemoryCandidatePage(raw: unknown): AgentMemoryCandidatePage {
  if (!isRecord(raw) || !exactKeys(raw, new Set(['candidates', 'nextCursor'])) || !Array.isArray(raw.candidates)) {
    throw new Error('Agent Memory candidate page shape is invalid')
  }
  const nextCursor = raw.nextCursor === undefined || raw.nextCursor === '' ? '' : requireIdentifier(raw.nextCursor, 'cursor')
  return { candidates: raw.candidates.map(parseAgentMemoryCandidate), nextCursor }
}

export function parseAgentMemoryCandidate(raw: unknown): AgentMemoryCandidate {
  if (!isRecord(raw) || !exactKeys(raw, candidateKeys)) throw new Error('Agent Memory candidate shape is invalid')
  if (raw.status !== 'pending' && raw.status !== 'accepted' && raw.status !== 'rejected') throw new Error('Agent Memory candidate status is invalid')
  if (typeof raw.summary !== 'string' || raw.summary.trim() === '' || byteLength(raw.summary) > 4096) throw new Error('Agent Memory candidate summary is invalid')
  if (typeof raw.candidateSha256 !== 'string' || !/^[a-f0-9]{64}$/.test(raw.candidateSha256)) throw new Error('Agent Memory candidate digest is invalid')
  const reviewId = optionalIdentifier(raw.reviewId, 'review')
  const promotedMemoryId = optionalIdentifier(raw.promotedMemoryId, 'promoted memory')
  if (raw.status === 'accepted' && reviewId === undefined) throw new Error('Agent Memory candidate review is required')
  return {
    candidateId: requireIdentifier(raw.candidateId, 'candidate'),
    candidateSha256: raw.candidateSha256,
    summary: raw.summary,
    status: raw.status,
    reviewId,
    promotedMemoryId,
    observedAtUnixMs: positiveInteger(raw.observedAtUnixMs, 'observed time'),
  }
}

const candidateKeys = new Set([
  'candidateId', 'candidateSha256', 'summary', 'status', 'reviewId', 'promotedMemoryId', 'observedAtUnixMs',
])

export const agentMemoryClient: AgentMemoryClient = {
  async list(after = '', limit = 50) {
    if (after) requireCursor(after)
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Memory page limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after) query.set('after', after)
    return parseAgentMemoryPage(await api.get(`/api/v1/agent/memories?${query.toString()}`))
  },
  async listCandidates(after = '', limit = 50) {
    if (after) requireIdentifier(after, 'cursor')
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Memory candidate page limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after) query.set('after', after)
    return parseAgentMemoryCandidatePage(await api.get(`/api/v1/agent/memory-candidates?${query.toString()}`))
  },
  async revoke(memoryId, reason) {
    requireIdentifier(memoryId, 'identity')
    const normalized = reason.trim()
    if (!normalized || [...normalized].length > 1000 || /[\u0000-\u001f\u007f]/u.test(normalized)) throw new Error('Agent Memory revoke reason is invalid')
    return parseAgentMemoryResponse(await api.post(`/api/v1/agent/memories/${encodeURIComponent(memoryId)}/revoke`, { reason: normalized }))
  },
  async correct(memoryId, expectedVersion, content, compactContent, reason) {
    requireIdentifier(memoryId, 'identity')
    if (!Number.isSafeInteger(expectedVersion) || expectedVersion < 1) throw new Error('Agent Memory expected version is invalid')
    const normalizedContent = content.trim()
    const normalizedCompact = compactContent.trim()
    const normalizedReason = normalizeReason(reason, 'correction')
    if (!normalizedContent || byteLength(normalizedContent) > 16 * 1024 || byteLength(normalizedCompact) > 4 * 1024) throw new Error('Agent Memory correction content is invalid')
    return parseAgentMemoryCorrection(await api.post(`/api/v1/agent/memories/${encodeURIComponent(memoryId)}/correct`, {
      expectedVersion, content: normalizedContent, compactContent: normalizedCompact, reason: normalizedReason,
    }), memoryId, expectedVersion, normalizedContent, normalizedCompact, normalizedReason)
  },
  async promoteCandidate(candidateId, candidateSha256, reviewId, targetMemoryType = 'observational') {
    requireIdentifier(candidateId, 'candidate')
    if (!/^[a-f0-9]{64}$/.test(candidateSha256)) throw new Error('Agent Memory candidate digest is invalid')
    requireIdentifier(reviewId, 'review')
    if (!memoryTypes.has(targetMemoryType) || targetMemoryType === 'working') throw new Error('Agent Memory candidate target type is invalid')
    return parseAgentMemoryResponse(await api.post(`/api/v1/agent/memory-candidates/${encodeURIComponent(candidateId)}/promote`, {
      candidateSha256, reviewId, targetMemoryType,
    }))
  },
  async reviewCandidate(candidateId, candidateSha256, decision, reason) {
    requireIdentifier(candidateId, 'candidate')
    if (!/^[a-f0-9]{64}$/.test(candidateSha256)) throw new Error('Agent Memory candidate digest is invalid')
    if (decision !== 'accepted' && decision !== 'rejected') throw new Error('Agent Memory candidate decision is invalid')
    const normalizedReason = normalizeReason(reason, 'review')
    return parseAgentMemoryCandidate(await api.post(`/api/v1/agent/memory-candidates/${encodeURIComponent(candidateId)}/review`, {
      candidateSha256, decision, reason: normalizedReason,
    }))
  },
}

export function parseAgentMemoryCorrection(raw: unknown, memoryId: string, expectedVersion: number, content: string, compactContent: string, reason: string): AgentMemoryCorrection {
  if (!isRecord(raw) || !exactKeys(raw, new Set(['previous', 'corrected'])) || raw.previous === undefined || raw.corrected === undefined) {
    throw new Error('Agent Memory correction response shape is invalid')
  }
  const previous = parseAgentMemoryResponse(raw.previous)
  const corrected = parseAgentMemoryResponse(raw.corrected)
  if (previous.memoryId !== memoryId || previous.memoryVersion !== expectedVersion || previous.status !== 'revoked' ||
    corrected.status !== 'active' || corrected.memoryRootId !== previous.memoryRootId || corrected.memoryVersion !== expectedVersion + 1 ||
    corrected.supersedesMemoryId !== memoryId || corrected.content !== content || corrected.compactContent !== compactContent || corrected.correctionReason !== reason) {
    throw new Error('Agent Memory correction response is inconsistent')
  }
  return { previous, corrected }
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

function optionalReason(raw: unknown, label = 'revoke'): string | undefined {
  if (raw === undefined) return undefined
  if (typeof raw !== 'string' || raw.trim() !== raw || !raw || [...raw].length > 1000 || /[\u0000-\u001f\u007f]/u.test(raw)) throw new Error(`Agent Memory ${label} reason is invalid`)
  return raw
}

function normalizeReason(raw: string, label: string): string {
  const normalized = raw.trim()
  return optionalReason(normalized, label) ?? ''
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
