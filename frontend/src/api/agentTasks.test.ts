import { describe, expect, it } from 'vitest'
import { parseAgentTaskResponse } from './agentTasks'

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
})
