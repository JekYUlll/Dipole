import api from '@/api'

export interface AgentArtifactMetadata {
  artifactId: string
  taskId: string
  runId: string
  artifactType: string
  version: number
  title: string
  mediaType: string
  contentSha256: string
  sizeBytes: number
  createdAtUnixMs: number
}

export interface AgentArtifactContent {
  artifactId: string
  mediaType: string
  content: string
}

export interface AgentArtifactPage {
  artifacts: AgentArtifactMetadata[]
  nextCursor: string
}

export interface AgentArtifactClient {
  list?(after?: string, limit?: number): Promise<AgentArtifactPage>
  get(artifactId: string): Promise<AgentArtifactMetadata>
  getContent(artifactId: string): Promise<AgentArtifactContent>
}

const artifactID = /^[a-f0-9]{64}$/
const artifactCursor = /^[1-9][0-9]{0,19}:[a-fA-F0-9]{64}$/
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const mediaType = /^[A-Za-z0-9!#$&^_.+-]+\/[A-Za-z0-9!#$&^_.+-]+(?:;[A-Za-z0-9!#$&^_.+=-]+)?$/
const keys = new Set(['artifactId', 'taskId', 'runId', 'artifactType', 'version', 'title', 'mediaType', 'contentSha256', 'sizeBytes', 'createdAtUnixMs'])
const contentKeys = new Set(['artifactId', 'mediaType', 'content'])
const pageKeys = new Set(['artifacts', 'nextCursor'])

export function parseAgentArtifactMetadata(raw: unknown): AgentArtifactMetadata {
  if (!isRecord(raw) || !exactKeys(raw, keys) || typeof raw.artifactId !== 'string' || !artifactID.test(raw.artifactId) ||
      typeof raw.taskId !== 'string' || !identifier.test(raw.taskId) || typeof raw.runId !== 'string' || !identifier.test(raw.runId) ||
      typeof raw.artifactType !== 'string' || !identifier.test(raw.artifactType) || !Number.isSafeInteger(raw.version) ||
      (raw.version as number) < 1 || !validText(raw.title, 256) || typeof raw.mediaType !== 'string' ||
      !mediaType.test(raw.mediaType) || typeof raw.contentSha256 !== 'string' || !artifactID.test(raw.contentSha256) ||
      !Number.isSafeInteger(raw.sizeBytes) || (raw.sizeBytes as number) < 0 || !Number.isSafeInteger(raw.createdAtUnixMs) ||
      (raw.createdAtUnixMs as number) <= 0) {
    throw new Error('Agent Artifact metadata is invalid')
  }
  // artifactId is a deterministic identity hash of the Artifact binding
  // (schema, task, run, type, version, contentSha256); it is distinct from
  // contentSha256, which addresses only the raw bytes. See
  // internal/application/agent_artifact.go::NewAgentArtifactV1 for the derivation.
  // Both are validated as 64-hex strings above; do not require equality here.
  return {
    artifactId: raw.artifactId,
    taskId: raw.taskId,
    runId: raw.runId,
    artifactType: raw.artifactType,
    version: raw.version as number,
    title: raw.title,
    mediaType: raw.mediaType,
    contentSha256: raw.contentSha256,
    sizeBytes: raw.sizeBytes as number,
    createdAtUnixMs: raw.createdAtUnixMs as number,
  }
}

export function parseAgentArtifactPage(raw: unknown): AgentArtifactPage {
  if (!isRecord(raw) || !exactKeys(raw, pageKeys, ['nextCursor']) || !Array.isArray(raw.artifacts) || raw.artifacts.length > 100) {
    throw new Error('Agent Artifact catalog is invalid')
  }
  const nextCursor = raw.nextCursor === undefined || raw.nextCursor === '' ? '' : requireCursor(raw.nextCursor)
  return { artifacts: raw.artifacts.map(parseAgentArtifactMetadata), nextCursor }
}

export function parseAgentArtifactContent(raw: unknown): AgentArtifactContent {
  if (!isRecord(raw) || !exactKeys(raw, contentKeys) || typeof raw.artifactId !== 'string' || !artifactID.test(raw.artifactId) ||
      typeof raw.mediaType !== 'string' || !mediaType.test(raw.mediaType) || !validText(raw.content, 1_048_576)) {
    throw new Error('Agent Artifact content is invalid')
  }
  return { artifactId: raw.artifactId, mediaType: raw.mediaType, content: raw.content }
}

export const agentArtifactClient: AgentArtifactClient = {
  async list(after = '', limit = 50) {
    if (after) requireCursor(after)
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Artifact catalog limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after) query.set('after', after)
    return parseAgentArtifactPage(await api.get(`/api/v1/agent/artifacts?${query.toString()}`))
  },
  async get(artifactId) {
    if (!artifactID.test(artifactId)) throw new Error('Agent Artifact ID is invalid')
    return parseAgentArtifactMetadata(await api.get(`/api/v1/agent/artifacts/${encodeURIComponent(artifactId)}`))
  },
  async getContent(artifactId) {
    if (!artifactID.test(artifactId)) throw new Error('Agent Artifact ID is invalid')
    return parseAgentArtifactContent(await api.get(`/api/v1/agent/artifacts/${encodeURIComponent(artifactId)}/content`))
  },
}

function validText(value: unknown, maxLength: number): value is string {
  return typeof value === 'string' && value.trim().length > 0 && value.length <= maxLength
}

function requireCursor(raw: unknown): string {
  if (typeof raw !== 'string' || !artifactCursor.test(raw)) throw new Error('Agent Artifact catalog cursor is invalid')
  return raw
}

function exactKeys(value: Record<string, unknown>, allowed: Set<string>, optional: string[] = []): boolean {
  const keysPresent = Object.keys(value)
  return keysPresent.every(key => allowed.has(key)) && [...allowed].every(key => optional.includes(key) || key in value)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
