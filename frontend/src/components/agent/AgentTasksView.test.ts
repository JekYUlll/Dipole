import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'
import PrimeVue from 'primevue/config'
import AgentTasksView from './AgentTasksView.vue'
import type { AgentOwnedTask, AgentTaskClient } from '@/api/agentTasks'

const waiting: AgentOwnedTask = {
  taskId: 'TASK-1', status: 'waiting_input', revision: 1, pendingKind: 'input',
  goal: 'Summarize unread work', updatedAtUnixMs: 1,
}

function client(list = vi.fn().mockResolvedValue({ tasks: [waiting], nextCursor: '' })): AgentTaskClient {
  return {
    list,
    getTask: vi.fn(),
    provideInput: vi.fn(),
    resolveApproval: vi.fn(),
    cancelTask: vi.fn(),
  }
}

async function mountView(taskClient: AgentTaskClient, query = '/?agent=1&view=tasks') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }],
  })
  await router.push(query)
  await router.isReady()
  return mount(AgentTasksView, {
    props: { client: taskClient },
    global: {
      plugins: [PrimeVue, router],
      stubs: {
        DataTable: { props: ['value'], template: '<div class="dt" />' },
        Column: true,
        Dialog: true,
        Skeleton: { template: '<div class="p-skeleton" />' },
        AgentTaskCreate: true,
        AgentTaskTimeline: true,
        AgentElicitationForm: true,
        AgentApprovalForm: true,
      },
    },
  })
}

describe('AgentTasksView', () => {
  it('shows skeleton rows on first load', async () => {
    const wrapper = await mountView(client(vi.fn(() => new Promise(() => {}))))
    expect(wrapper.findAll('.p-skeleton').length).toBeGreaterThan(0)
  })

  it('renders an empty row instead of a hero state card', async () => {
    const wrapper = await mountView(client(vi.fn().mockResolvedValue({ tasks: [], nextCursor: '' })))
    await flushPromises()
    expect(wrapper.get('.empty-row').text()).toContain('还没有任务')
    expect(wrapper.find('.state-card').exists()).toBe(false)
  })

  it('keeps a banner when the list fails', async () => {
    const wrapper = await mountView(client(vi.fn().mockRejectedValue(new Error('boom'))))
    await flushPromises()
    expect(wrapper.text()).toContain('任务列表读取失败')
    expect(wrapper.find('.banner').exists()).toBe(true)
    expect(wrapper.find('.state-card').exists()).toBe(false)
  })
})
