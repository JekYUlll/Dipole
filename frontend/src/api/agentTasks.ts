import api from '@/api'

export type AgentElicitationSource =
  | { kind: 'agent' }
  | { kind: 'mcp'; serverId: string; toolName: string; invocationId: string; trust: 'untrusted' }

type AgentElicitationFieldBase = { id: string; label: string; required: boolean }
export type AgentElicitationField =
  | (AgentElicitationFieldBase & { type: 'text'; maxLength?: number })
  | (AgentElicitationFieldBase & { type: 'select'; options: string[] })
  | (AgentElicitationFieldBase & { type: 'multiselect'; options: string[]; maxSelections?: number })
  | (AgentElicitationFieldBase & { type: 'boolean' })

export interface AgentElicitationForm {
  schemaVersion: 'dipole.agent.elicitation.v1'
  fields: AgentElicitationField[]
}

export interface AgentInputPending {
  kind: 'input'
  requestId: string
  prompt: string
  form: AgentElicitationForm
  source: AgentElicitationSource
  expiresAtUnixMs: number
}

export interface AgentApprovalPending {
  kind: 'approval'
  requestId: string
  approvalId: string
  summary: string
  expiresAtUnixMs: number
}

export type AgentTaskStatus = 'created' | 'running' | 'waiting_input' | 'waiting_approval' | 'completed' | 'failed' | 'cancelled'

export interface AgentTaskState {
  taskId: string
  status: AgentTaskStatus
  revision: number
  persistentStatus: string
  workflowProjection: { outcome: 'match' | 'missing' | 'stale' | 'ahead' | 'conflict'; status?: string; revision?: number }
  pending?: AgentInputPending | AgentApprovalPending
  cancellation?: { reason: string; requestId?: string }
}

export type AgentElicitationValue = Record<string, string | boolean | string[]>

export interface AgentTaskClient {
  getTask(taskId: string): Promise<AgentTaskState>
  getTimeline?(taskId: string, after?: string, limit?: number): Promise<AgentTaskTimelinePage>
  provideInput(taskId: string, requestId: string, value: AgentElicitationValue): Promise<void>
  resolveApproval(taskId: string, approvalId: string, decision: 'approved' | 'denied'): Promise<void>
  cancelTask(taskId: string): Promise<void>
}

export interface AgentTaskTimelineEvent {
  eventSeq: string
  eventId: string
  taskId: string
  runId: string
  kind: string
  status: string
  capabilityId?: string
  approvalId?: string
  occurredAtUnixMs: number
}

export interface AgentTaskTimelinePage {
  schemaVersion: 'dipole.agent.task_timeline.v1'
  taskId: string
  revision: number
  events: AgentTaskTimelineEvent[]
  nextCursor: string
}

const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/
const fieldIdentifier = /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/
const statuses = new Set<AgentTaskStatus>(['created', 'running', 'waiting_input', 'waiting_approval', 'completed', 'failed', 'cancelled'])
const outcomes = new Set(['match', 'missing', 'stale', 'ahead', 'conflict'])
const sensitiveNames = new Set([
  'password', 'passwd', 'secret', 'secretkey', 'token', 'apikey', 'apitoken', 'authkey', 'authtoken',
  'accesskey', 'accesstoken', 'refreshtoken', 'sessionid', 'sessiontoken', 'bearertoken', 'authorization',
  'cookie', 'credential', 'credentials', 'privatekey', 'clientsecret', 'payment', 'creditcard',
])

export function parseAgentTaskResponse(raw: unknown): AgentTaskState {
  if (!isRecord(raw) || !validIdentity(raw.taskId) || !statuses.has(raw.status as AgentTaskStatus) ||
      !Number.isSafeInteger(raw.revision) || (raw.revision as number) < 0 || typeof raw.persistentStatus !== 'string') {
    throw new Error('Agent Task response identity is invalid')
  }
  const workflowProjection = parseProjection(raw.workflowProjection)
  const state: AgentTaskState = {
    taskId: raw.taskId as string,
    status: raw.status as AgentTaskStatus,
    revision: raw.revision as number,
    persistentStatus: raw.persistentStatus,
    workflowProjection,
  }
  if (raw.status === 'waiting_input') state.pending = parseInputPending(raw.pending)
  if (raw.status === 'waiting_approval') state.pending = parseApprovalPending(raw.pending)
  if (raw.status === 'cancelled' && raw.cancellation !== undefined) state.cancellation = parseCancellation(raw.cancellation)
  return state
}

export const agentTaskClient: AgentTaskClient = {
  async getTask(taskId) {
    requireIdentity(taskId, 'Task')
    return parseAgentTaskResponse(await api.get(`/api/v1/agent/tasks/${encodeURIComponent(taskId)}`))
  },
  async getTimeline(taskId, after = '', limit = 50) {
    requireIdentity(taskId, 'Task')
    if (after !== '' && !/^[0-9]{1,20}$/.test(after)) throw new Error('Agent Task Timeline cursor is invalid')
    if (!Number.isInteger(limit) || limit < 1 || limit > 100) throw new Error('Agent Task Timeline limit is invalid')
    const query = new URLSearchParams({ limit: String(limit) })
    if (after !== '') query.set('after', after)
    return parseAgentTaskTimelineResponse(await api.get(`/api/v1/agent/tasks/${encodeURIComponent(taskId)}/timeline?${query.toString()}`))
  },
  async provideInput(taskId, requestId, value) {
    requireIdentity(taskId, 'Task')
    requireIdentity(requestId, 'request')
    await api.post(`/api/v1/agent/tasks/${encodeURIComponent(taskId)}/inputs/${encodeURIComponent(requestId)}`, { value })
  },
  async resolveApproval(taskId, approvalId, decision) {
    requireIdentity(taskId, 'Task')
    requireIdentity(approvalId, 'approval')
    if (decision !== 'approved' && decision !== 'denied') throw new Error('Agent approval decision is invalid')
    await api.post(`/api/v1/agent/tasks/${encodeURIComponent(taskId)}/approvals/${encodeURIComponent(approvalId)}`, { decision })
  },
  async cancelTask(taskId) {
    requireIdentity(taskId, 'Task')
    await api.post(`/api/v1/agent/tasks/${encodeURIComponent(taskId)}/cancel`, { reason: 'user_cancelled' })
  },
}

function parseAgentTaskTimelineResponse(raw: unknown): AgentTaskTimelinePage {
  if (!isRecord(raw) || raw.schemaVersion !== 'dipole.agent.task_timeline.v1' || !validIdentity(raw.taskId) ||
      !Number.isSafeInteger(raw.revision) || (raw.revision as number) < 0 || !Array.isArray(raw.events) ||
      raw.events.length > 100 || typeof raw.nextCursor !== 'string' || raw.nextCursor.length > 384 ||
      (raw.nextCursor !== '' && !/^[0-9]{1,20}$/.test(raw.nextCursor))) {
    throw new Error('Agent Task Timeline response is invalid')
  }
  const events = raw.events.map(parseAgentTaskTimelineEvent)
  if (events.some(event => event.taskId !== raw.taskId)) throw new Error('Agent Task Timeline event binding is invalid')
  return {
    schemaVersion: 'dipole.agent.task_timeline.v1', taskId: raw.taskId as string,
    revision: raw.revision as number, events, nextCursor: raw.nextCursor,
  }
}

function parseAgentTaskTimelineEvent(raw: unknown): AgentTaskTimelineEvent {
  if (!isRecord(raw) || typeof raw.eventSeq !== 'string' || !/^[1-9][0-9]{0,19}$/.test(raw.eventSeq) ||
      !validIdentity(raw.eventId) || !validIdentity(raw.taskId) || typeof raw.runId !== 'string' || raw.runId.length > 128 ||
      typeof raw.kind !== 'string' || raw.kind.length === 0 || raw.kind.length > 64 || typeof raw.status !== 'string' ||
      raw.status.length === 0 || raw.status.length > 64 || !Number.isSafeInteger(raw.occurredAtUnixMs) ||
      (raw.occurredAtUnixMs as number) <= 0) throw new Error('Agent Task Timeline event is invalid')
  if (raw.capabilityId !== undefined && !validIdentity(raw.capabilityId)) throw new Error('Agent Task Timeline capability is invalid')
  if (raw.approvalId !== undefined && !validIdentity(raw.approvalId)) throw new Error('Agent Task Timeline approval is invalid')
  return {
    eventSeq: raw.eventSeq as string, eventId: raw.eventId as string, taskId: raw.taskId as string, runId: raw.runId as string,
    kind: raw.kind as string, status: raw.status as string,
    ...(raw.capabilityId === undefined ? {} : { capabilityId: raw.capabilityId as string }),
    ...(raw.approvalId === undefined ? {} : { approvalId: raw.approvalId as string }),
    occurredAtUnixMs: raw.occurredAtUnixMs as number,
  }
}

function parseInputPending(raw: unknown): AgentInputPending {
  if (!isRecord(raw) || raw.kind !== 'input' || !validIdentity(raw.requestId) || typeof raw.prompt !== 'string' ||
      raw.prompt.trim().length === 0 || raw.prompt.length > 2000 || !Number.isSafeInteger(raw.expiresAtUnixMs) ||
      (raw.expiresAtUnixMs as number) <= 0) throw new Error('Agent Elicitation pending request is invalid')
  return {
    kind: 'input', requestId: raw.requestId as string, prompt: raw.prompt,
    form: parseForm(raw.form), source: parseSource(raw.source), expiresAtUnixMs: raw.expiresAtUnixMs as number,
  }
}

function parseApprovalPending(raw: unknown): AgentApprovalPending {
  if (!isRecord(raw) || raw.kind !== 'approval' || !validIdentity(raw.requestId) ||
      !validIdentity(raw.approvalId) || typeof raw.summary !== 'string' ||
      raw.summary.trim().length === 0 || raw.summary.length > 2000 ||
      !Number.isSafeInteger(raw.expiresAtUnixMs) || (raw.expiresAtUnixMs as number) <= 0) {
    throw new Error('Agent approval pending request is invalid')
  }
  return {
    kind: 'approval', requestId: raw.requestId as string, approvalId: raw.approvalId as string,
    summary: raw.summary, expiresAtUnixMs: raw.expiresAtUnixMs as number,
  }
}

function parseSource(raw: unknown): AgentElicitationSource {
  if (!isRecord(raw)) throw new Error('Agent Elicitation source is invalid')
  if (raw.kind === 'agent' && Object.keys(raw).length === 1) return { kind: 'agent' }
  if (raw.kind !== 'mcp' || raw.trust !== 'untrusted' || !validIdentity(raw.serverId) ||
      !validIdentity(raw.toolName) || !validIdentity(raw.invocationId) ||
      Object.keys(raw).some(key => !['kind', 'serverId', 'toolName', 'invocationId', 'trust'].includes(key))) {
    throw new Error('Agent Elicitation MCP source is invalid')
  }
  return {
    kind: 'mcp', serverId: raw.serverId as string, toolName: raw.toolName as string,
    invocationId: raw.invocationId as string, trust: 'untrusted',
  }
}

function parseForm(raw: unknown): AgentElicitationForm {
  if (!isRecord(raw) || raw.schemaVersion !== 'dipole.agent.elicitation.v1' || !Array.isArray(raw.fields) ||
      raw.fields.length < 1 || raw.fields.length > 16 || byteLength(raw) > 16 * 1024) {
    throw new Error('Agent Elicitation Form is invalid')
  }
  const fields = raw.fields.map(parseField)
  if (new Set(fields.map(field => field.id)).size !== fields.length) throw new Error('Agent Elicitation field IDs must be unique')
  return { schemaVersion: 'dipole.agent.elicitation.v1', fields }
}

function parseField(raw: unknown): AgentElicitationField {
  if (!isRecord(raw) || typeof raw.id !== 'string' || !fieldIdentifier.test(raw.id) || sensitiveField(raw.id) ||
      typeof raw.label !== 'string' || raw.label.trim().length === 0 || raw.label.length > 256 || sensitiveField(raw.label) ||
      typeof raw.required !== 'boolean') throw new Error('Agent Elicitation field is sensitive or invalid')
  const base = { id: raw.id, label: raw.label, required: raw.required }
  if (raw.type === 'text') {
    if (raw.maxLength !== undefined && (!Number.isInteger(raw.maxLength) || (raw.maxLength as number) < 1 || (raw.maxLength as number) > 4000)) {
      throw new Error('Agent Elicitation text field is invalid')
    }
    return { ...base, type: 'text', ...(raw.maxLength === undefined ? {} : { maxLength: raw.maxLength as number }) }
  }
  if (raw.type === 'boolean') return { ...base, type: 'boolean' }
  if (raw.type !== 'select' && raw.type !== 'multiselect') throw new Error('Agent Elicitation field type is unsupported')
  const options = parseOptions(raw.options)
  if (raw.type === 'select') return { ...base, type: 'select', options }
  if (raw.maxSelections !== undefined && (!Number.isInteger(raw.maxSelections) || (raw.maxSelections as number) < 1 || (raw.maxSelections as number) > options.length)) {
    throw new Error('Agent Elicitation multiselect field is invalid')
  }
  return { ...base, type: 'multiselect', options, ...(raw.maxSelections === undefined ? {} : { maxSelections: raw.maxSelections as number }) }
}

function parseOptions(raw: unknown): string[] {
  if (!Array.isArray(raw) || raw.length < 1 || raw.length > 32 || raw.some(value => typeof value !== 'string' || value.trim().length === 0 || value.length > 128) || new Set(raw).size !== raw.length) {
    throw new Error('Agent Elicitation options are invalid')
  }
  return [...raw]
}

function parseProjection(raw: unknown): AgentTaskState['workflowProjection'] {
  if (!isRecord(raw) || !outcomes.has(raw.outcome as string)) throw new Error('Agent Task projection is invalid')
  const projection: AgentTaskState['workflowProjection'] = { outcome: raw.outcome as AgentTaskState['workflowProjection']['outcome'] }
  if (typeof raw.status === 'string') projection.status = raw.status
  if (Number.isSafeInteger(raw.revision) && (raw.revision as number) >= 0) projection.revision = raw.revision as number
  return projection
}

function parseCancellation(raw: unknown): { reason: string; requestId?: string } {
  if (!isRecord(raw) || typeof raw.reason !== 'string' || raw.reason.length > 256) throw new Error('Agent Task cancellation is invalid')
  return { reason: raw.reason, ...(validIdentity(raw.requestId) ? { requestId: raw.requestId as string } : {}) }
}

function sensitiveField(value: string): boolean {
  return sensitiveNames.has(value.toLowerCase().replace(/[^a-z0-9]/g, ''))
}

function validIdentity(value: unknown): boolean {
  return typeof value === 'string' && identifier.test(value)
}

function requireIdentity(value: string, label: string): void {
  if (!validIdentity(value)) throw new Error(`Agent ${label} identity is invalid`)
}

function byteLength(value: unknown): number {
  return new TextEncoder().encode(JSON.stringify(value)).length
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
