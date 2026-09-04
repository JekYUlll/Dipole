import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentTaskInbox from './AgentTaskInbox.vue'
import type { AgentOwnedTask, AgentTaskClient } from '@/api/agentTasks'

const source = readFileSync(resolve(import.meta.dirname, 'AgentTaskInbox.vue'), 'utf8')

const waiting: AgentOwnedTask = {
  taskId: 'TASK-1', status: 'waiting_approval', revision: 2, pendingKind: 'approval',
  goal: 'Summarize unread work', updatedAtUnixMs: 1_700_000_000_000,
}

function client(tasks = [waiting], nextCursor = ''): AgentTaskClient {
  return {
    list: vi.fn().mockResolvedValue({ tasks, nextCursor }),
    getTask: vi.fn(),
    provideInput: vi.fn(),
    resolveApproval: vi.fn(),
    cancelTask: vi.fn(),
  }
}

function mountInbox(taskClient: AgentTaskClient) {
  return mount(AgentTaskInbox, {
    props: { client: taskClient },
    global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-route-name="to.name"><slot /></a>' } } },
  })
}

describe('AgentTaskInbox', () => {
  it('uses the shared Pencil token surface for the inbox shell', () => {
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-font-body)')
    expect(source).toContain('OWNER INBOX')
  })

  it('renders owner tasks and links waiting rows to timeline', async () => {
    const taskClient = client()
    const wrapper = mountInbox(taskClient)
    await flushPromises()

    expect(taskClient.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.get('[data-agent-task-id="TASK-1"]').text()).toContain('Summarize unread work')
    expect(wrapper.get('[data-agent-task-inbox-timeline]').attributes('data-route-name')).toBe('agent-task-timeline')
  })

  it('appends only a distinct next page', async () => {
    const taskClient = client([waiting], 'NEXT')
    vi.mocked(taskClient.list!)
      .mockResolvedValueOnce({ tasks: [waiting], nextCursor: 'NEXT' })
      .mockResolvedValueOnce({ tasks: [{ ...waiting, taskId: 'TASK-2', goal: 'Second task' }], nextCursor: '' })
    const wrapper = mountInbox(taskClient)
    await flushPromises()
    await wrapper.get('[data-agent-task-inbox-more]').trigger('click')
    await flushPromises()

    expect(taskClient.list).toHaveBeenLastCalledWith('NEXT', 50)
    expect(wrapper.findAll('[data-agent-task-id]').length).toBe(2)
  })

  it('hides the control rail when embedded', async () => {
    const wrapper = mount(AgentTaskInbox, {
      props: { client: client(), embedded: true },
      global: { stubs: { RouterLink: { props: ['to'], template: '<a><slot /></a>' } } },
    })
    await flushPromises()
    expect(wrapper.find('.control-rail').exists()).toBe(false)
  })
})
