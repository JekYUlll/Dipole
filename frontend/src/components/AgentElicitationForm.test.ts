import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentElicitationForm from './AgentElicitationForm.vue'
import type { AgentTaskClient, AgentTaskState } from '@/api/agentTasks'

const waitingTask: AgentTaskState = {
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

const runningTask: AgentTaskState = {
  taskId: 'TASK-1', status: 'running', revision: 23, persistentStatus: 'running',
  workflowProjection: { outcome: 'stale', status: 'waiting_input', revision: 22 },
}

const client = (states: AgentTaskState[]): AgentTaskClient => ({
  getTask: vi.fn().mockImplementation(async () => states.shift() ?? runningTask),
  provideInput: vi.fn().mockResolvedValue(undefined),
  cancelTask: vi.fn().mockResolvedValue(undefined),
})

describe('AgentElicitationForm', () => {
  it('renders all ordinary fields and submits the exact Task/request binding', async () => {
    const service = client([waitingTask, runningTask])
    const wrapper = mount(AgentElicitationForm, { props: { taskId: 'TASK-1', client: service, now: () => 1_000 } })
    await flushPromises()

    expect(wrapper.text()).toContain('calendar.example')
    expect(wrapper.text()).toContain('calendar.create')
    await wrapper.get('[data-agent-field="title"]').setValue('Release review')
    await wrapper.get('[data-agent-field="visibility"]').setValue('team')
    await wrapper.get('[data-agent-field="labels"][value="release"]').setValue(true)
    await wrapper.get('[data-agent-field="notify"]').setValue(true)
    await wrapper.get('[data-agent-submit]').trigger('submit')
    await flushPromises()

    expect(service.provideInput).toHaveBeenCalledWith('TASK-1', 'INPUT-1', {
      title: 'Release review', visibility: 'team', labels: ['release'], notify: true,
    })
    expect(wrapper.get('[data-agent-elicit-state]').attributes('data-agent-elicit-state')).toBe('running')
  })

  it('keeps local values and the durable request pending after validation fails', async () => {
    const service = client([waitingTask])
    const wrapper = mount(AgentElicitationForm, { props: { taskId: 'TASK-1', client: service, now: () => 1_000 } })
    await flushPromises()
    await wrapper.get('[data-agent-field="title"]').setValue('')
    await wrapper.get('[data-agent-submit]').trigger('submit')
    await flushPromises()

    expect(service.provideInput).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Event title为必填项')
    expect((wrapper.get('[data-agent-field="title"]').element as HTMLInputElement).value).toBe('')
    expect(wrapper.get('[data-agent-elicit-state]').attributes('data-agent-elicit-state')).toBe('validation_error')
  })

  it('fails closed without rendering a cached Form when Task Query is unavailable', async () => {
    const service = client([])
    vi.mocked(service.getTask).mockRejectedValueOnce(new Error('secret upstream detail')).mockResolvedValueOnce(waitingTask)
    const wrapper = mount(AgentElicitationForm, { props: { taskId: 'TASK-1', client: service, now: () => 1_000 } })
    await flushPromises()

    expect(wrapper.text()).toContain('无法确认当前输入请求')
    expect(wrapper.text()).not.toContain('secret upstream detail')
    expect(wrapper.find('[data-agent-submit]').exists()).toBe(false)
    await wrapper.get('[data-agent-retry]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-agent-submit]').exists()).toBe(true)
  })
})
