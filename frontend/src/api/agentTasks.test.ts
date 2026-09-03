import { afterEach, describe, expect, it, vi } from 'vitest'
import api from './index'
import { agentTaskClient, parseAgentTaskInboxPage, parseAgentTaskResponse, parseAgentTaskStartResponse } from './agentTasks'

const waitingTask = {
  taskId: 'TASK-1', status: 'waiting_input', revision: 22, persistentStatus: 'running',
  workflowProjection: { outcome: 'match', status: 'waiting_input', revision: 22 },
  pending: {
    kind: 'input', requestId: 'INPUT-1', prompt: 'Choose event settings', expiresAtUnixMs: 4_000,
    source: { kind: 'mcp', serverId: 'calendar.example', toolName: 'calendar.create', invocationId: 'INV-1', trust: 'untrusted' },
    form: { schemaVersion: 'dipole.agent.elicitation.v1', fields: [
      { id: 'title', label: 'Event title', type: 'text', required: true, maxLength: 120 },
      { id: 'visibility', label: 'Visibility', type: 'select', required: true, options: ['team', 'private'] },
      { id: 'labels', label: 'Labels', type: 'multiselect', required: false, options: ['release', 'incident'], maxSelections: 1 },
      { id: 'notify', label: 'Notify attendees', type: 'boolean', required: false },
    ] },
  },
}

describe('Agent Task response parser', () => {
  afterEach(() => vi.restoreAllMocks())
  it('accepts an exact source-bound ordinary Form', () => {
    expect(parseAgentTaskResponse(waitingTask)).toMatchObject({
      taskId: 'TASK-1', status: 'waiting_input',
      pending: { requestId: 'INPUT-1', source: { kind: 'mcp', trust: 'untrusted' } },
    })
  })

  it('rejects authority-bearing sources and sensitive Form fields', () => {
    expect(() => parseAgentTaskResponse({
      ...waitingTask,
      pending: { ...waitingTask.pending, source: { ...waitingTask.pending.source, trust: 'trusted' } },
    })).toThrow(/source/i)
    expect(() => parseAgentTaskResponse({
      ...waitingTask,
      pending: { ...waitingTask.pending, form: {
        schemaVersion: 'dipole.agent.elicitation.v1',
        fields: [{ id: 'api_token', label: 'API token', type: 'text', required: true }],
      } },
    })).toThrow(/sensitive/i)
  })

  it('parses and validates waiting approval state', () => {
    const task = {
      ...waitingTask,
      status: 'waiting_approval',
      pending: { kind: 'approval', requestId: 'APPROVAL-1', approvalId: 'GRANT-1', summary: 'Send the project update', expiresAtUnixMs: 8_000 },
    }
    expect(parseAgentTaskResponse(task)).toMatchObject({
      status: 'waiting_approval', pending: { kind: 'approval', approvalId: 'GRANT-1' },
    })
    expect(() => parseAgentTaskResponse({ ...task, pending: { ...task.pending, summary: '' } })).toThrow(/approval/i)
    expect(() => parseAgentTaskResponse({ ...task, pending: { ...task.pending, approvalId: 'bad approval id' } })).toThrow(/approval/i)
  })

  it('posts approval decisions with the authenticated Task binding', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({} as never)
    await agentTaskClient.resolveApproval('TASK-1', 'APPROVAL-1', 'approved')
    expect(post).toHaveBeenCalledWith('/api/v1/agent/tasks/TASK-1/approvals/APPROVAL-1', { decision: 'approved' })
    await expect(agentTaskClient.resolveApproval('TASK-1', 'bad approval id', 'approved')).rejects.toThrow(/approval/i)
    await expect(agentTaskClient.resolveApproval('TASK-1', 'APPROVAL-1', 'invalid' as 'approved')).rejects.toThrow(/decision/i)
  })

  it('starts an idempotent interactive Task and validates the accepted binding', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({ taskId: 'TASK-1', status: 'accepted' } as never)
    await expect(agentTaskClient.startTask!({ clientRequestId: 'local:001', goal: '  Summarize unread work  ' }))
      .resolves.toEqual({ taskId: 'TASK-1', status: 'accepted' })
    expect(post).toHaveBeenCalledWith('/api/v1/agent/tasks', { client_request_id: 'local:001', goal: 'Summarize unread work' })
    await expect(agentTaskClient.startTask!({ clientRequestId: 'bad id', goal: 'work' })).rejects.toThrow(/start request/i)
    expect(() => parseAgentTaskStartResponse({ taskId: 'TASK-1', status: 'running' })).toThrow(/start response/i)
  })

  it('lists owner-scoped inbox tasks and rejects pending-kind drift', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      tasks: [{ taskId: 'TASK-1', status: 'waiting_approval', revision: 2, pendingKind: 'approval', goal: 'Summarize unread work', updatedAtUnixMs: 1_700_000_000_000 }],
      nextCursor: 'CURSOR-1',
    } as never)
    await expect(agentTaskClient.list!('CURSOR-0', 20)).resolves.toMatchObject({ tasks: [{ taskId: 'TASK-1', pendingKind: 'approval' }], nextCursor: 'CURSOR-1' })
    expect(get).toHaveBeenCalledWith('/api/v1/agent/tasks?limit=20&after=CURSOR-0')
    expect(() => parseAgentTaskInboxPage({
      tasks: [{ taskId: 'TASK-1', status: 'waiting_input', revision: 1, pendingKind: 'approval', goal: 'Need input', updatedAtUnixMs: 1_700_000_000_000 }],
      nextCursor: '',
    })).toThrow(/pending kind/i)
    await expect(agentTaskClient.list!('bad cursor', 20)).rejects.toThrow(/cursor/i)
    await expect(agentTaskClient.list!('', 101)).rejects.toThrow(/limit/i)
  })

  it('fetches and validates an owner-scoped timeline page', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 4,
      events: [{ eventSeq: '7', eventId: 'EV-7', taskId: 'TASK-1', runId: 'RUN-1', kind: 'artifact', status: 'created', artifactId: 'a'.repeat(64), occurredAtUnixMs: 7_000 }],
      nextCursor: '7',
    } as never)
    await expect(agentTaskClient.getTimeline!('TASK-1', '3', 20)).resolves.toMatchObject({ taskId: 'TASK-1', nextCursor: '7' })
    expect(get).toHaveBeenCalledWith('/api/v1/agent/tasks/TASK-1/timeline?limit=20&after=3')
    await expect(agentTaskClient.getTimeline!('TASK-1', 'bad cursor', 20)).rejects.toThrow(/cursor/i)
    await expect(agentTaskClient.getTimeline!('TASK-1', '', 101)).rejects.toThrow(/limit/i)
    get.mockResolvedValueOnce({
      schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 4,
      events: [{ eventSeq: '8', eventId: 'EV-8', taskId: 'TASK-1', runId: 'RUN-1', kind: 'model_call', status: 'completed', artifactId: 'a'.repeat(64), occurredAtUnixMs: 8_000 }],
      nextCursor: '',
    } as never)
    await expect(agentTaskClient.getTimeline!('TASK-1')).rejects.toThrow(/Artifact/i)
    get.mockResolvedValueOnce({
      schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 4,
      events: [{ eventSeq: '7', eventId: 'EV-7', taskId: 'TASK-2', runId: 'RUN-1', kind: 'tool_invocation', status: 'completed', occurredAtUnixMs: 7_000 }],
      nextCursor: '',
    } as never)
    await expect(agentTaskClient.getTimeline!('TASK-1')).rejects.toThrow(/binding/i)
  })
})
