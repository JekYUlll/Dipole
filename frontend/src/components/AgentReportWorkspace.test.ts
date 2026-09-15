import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { webcrypto } from 'node:crypto'
import AgentReportWorkspace from './AgentReportWorkspace.vue'
import { agentTaskClient } from '@/api/agentTasks'
import type { Message } from '@/types'

vi.mock('@/api/agentTasks', () => ({ agentTaskClient: { getTask: vi.fn(), provideInput: vi.fn(), resolveApproval: vi.fn(), cancelTask: vi.fn() } }))
const props = { messages: [] as Message[], ownerId: 'U1', conversationName: 'Project', group: true, agentId: 'UAI' }
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('collaboration report workspace', () => {
  it('validates deadline and sends a group mention through the existing chat command', async () => {
    const wrapper = mount(AgentReportWorkspace, { props })
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('create')).toBeUndefined()
    const tomorrow = new Date(Date.now() + 86400000)
    const local = new Date(tomorrow.getTime() - tomorrow.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
    await wrapper.get('input[type="datetime-local"]').setValue(local)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('create')?.[0]?.[0]).toMatch(/^@AI \/report .*Z /)
    wrapper.unmount()
  })

  it('shows only owner tasks and submits edits to the bound input request', async () => {
    vi.stubGlobal('crypto', webcrypto)
    vi.mocked(agentTaskClient.getTask).mockResolvedValue({ taskId: 'task:1', status: 'waiting_input', revision: 1,
      persistentStatus: 'running', workflowProjection: { outcome: 'match' }, pending: { kind: 'input', requestId: 'report:draft',
        prompt: 'Draft', expiresAtUnixMs: Date.now() + 60000, source: { kind: 'agent' },
        form: { schemaVersion: 'dipole.agent.elicitation.v1', fields: [{ id: 'content', label: 'Edit', type: 'text', required: false, maxLength: 1800 }] } } })
    const messages = [{ from_uuid: 'U1', message_id: 'M1', content: '@AI /report 2026-09-16T00:00:00Z My report' },
      { from_uuid: 'U2', message_id: 'M2', content: '/report 2026-09-16T00:00:00Z Another owner' }] as Message[]
    const wrapper = mount(AgentReportWorkspace, { props: { ...props, messages } })
    await vi.waitFor(() => expect(wrapper.findAll('.task-item')).toHaveLength(1))
    await flushPromises()
    await wrapper.get('main textarea').setValue('Edited report')
    await wrapper.get('main form').trigger('submit')
    await flushPromises()
    expect(agentTaskClient.provideInput).toHaveBeenCalledWith(expect.stringMatching(/^task:/), 'report:draft', { content: 'Edited report' })
    expect(agentTaskClient.resolveApproval).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('indexes ordinary direct messages as Agent tasks and always exposes the timeline', async () => {
    vi.stubGlobal('crypto', webcrypto)
    vi.mocked(agentTaskClient.getTask).mockResolvedValue({ taskId: 'task:1', status: 'completed', revision: 1,
      persistentStatus: 'completed', workflowProjection: { outcome: 'match' } })
    const messages = [{ from_uuid: 'U1', message_id: 'M1', content: '帮我找之前关于 Cassandra 的讨论并总结结论' }] as Message[]
    const wrapper = mount(AgentReportWorkspace, { props: { ...props, group: false, messages } })
    await vi.waitFor(() => expect(wrapper.findAll('.task-item')).toHaveLength(1))
    await flushPromises()
    expect(wrapper.get('a').attributes('href')).toMatch(/^\/app\/agent\/tasks\/task%3A[0-9a-f]+\/timeline$/)
    wrapper.unmount()
  })
})
