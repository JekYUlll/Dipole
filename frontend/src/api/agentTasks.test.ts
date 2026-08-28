import { afterEach, describe, expect, it, vi } from 'vitest'
import api from './index'
import { agentTaskClient, parseAgentTaskResponse } from './agentTasks'

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
})
