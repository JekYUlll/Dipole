import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import AgentDrawer from './AgentDrawer.vue'

const flags = vi.hoisted(() => ({
  elicitation: true,
  approval: true,
  subscriptions: true,
  definitions: true,
  memories: true,
  memoryCorrection: true,
  taskCreate: true,
  timeline: true,
  artifacts: true,
}))

vi.mock('@/config/agentFlags', () => ({ agentFlags: flags }))
vi.mock('@/api/agentTasks', () => ({
  agentTaskClient: { list: vi.fn().mockResolvedValue({ tasks: [], nextCursor: '' }) },
}))
vi.mock('@/api/agentArtifacts', () => ({
  agentArtifactClient: { list: vi.fn().mockResolvedValue({ artifacts: [], nextCursor: '' }) },
}))

const wrappers: ReturnType<typeof mount>[] = []

async function mountDrawer(query = '/?agent=1&view=live') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div />' } }],
  })
  await router.push(query)
  await router.isReady()
  const wrapper = mount(AgentDrawer, {
    global: {
      plugins: [router, createPinia()],
      stubs: { Toast: true },
    },
  })
  wrappers.push(wrapper)
  await flushPromises()
  await new Promise(resolve => setTimeout(resolve, 30))
  await flushPromises()
  return { wrapper, router }
}

describe('AgentDrawer', () => {
  afterEach(async () => {
    while (wrappers.length) wrappers.pop()?.unmount()
    await flushPromises()
  })

  beforeEach(() => {
    setActivePinia(createPinia())
    Object.assign(flags, {
      elicitation: true, approval: true, subscriptions: true, definitions: true,
      memories: true, memoryCorrection: true, taskCreate: true, timeline: true, artifacts: true,
    })
  })

  it('keeps the six tabs in a fixed order', async () => {
    const { wrapper } = await mountDrawer()
    expect(wrapper.findAll('[data-agent-drawer-tab]').map(tab => tab.attributes('data-agent-drawer-tab')))
      .toEqual(['live', 'tasks', 'artifacts', 'definitions', 'subscriptions', 'memories'])
  })

  it('hides flagged tabs without reshuffling the rest', async () => {
    flags.artifacts = false
    flags.subscriptions = false
    flags.memories = false
    flags.definitions = false
    flags.taskCreate = false
    flags.timeline = false
    const { wrapper } = await mountDrawer()
    expect(wrapper.findAll('[data-agent-drawer-tab]').map(tab => tab.attributes('data-agent-drawer-tab')))
      .toEqual(['live'])
  })

  it('closes on Escape without stacking history', async () => {
    const { router } = await mountDrawer('/?agent=1&view=live&task=TASK-1')
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()
    expect(router.currentRoute.value.query.agent).toBeUndefined()
    expect(router.currentRoute.value.query.view).toBeUndefined()
    expect(router.currentRoute.value.query.task).toBeUndefined()
  })

  it('falls back to live when the URL asks for a hidden view', async () => {
    flags.memories = false
    const { wrapper } = await mountDrawer('/?agent=1&view=memories')
    expect(wrapper.attributes('data-agent-drawer-view')).toBe('live')
  })
})
