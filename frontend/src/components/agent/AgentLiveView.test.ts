import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AgentLiveView from './AgentLiveView.vue'
import { useChatStore } from '@/stores/chat'

const list = vi.fn()
const artifactList = vi.fn()

vi.mock('@/config/agentFlags', () => ({
  agentFlags: { timeline: true, artifacts: true },
}))
vi.mock('@/api/agentTasks', () => ({
  agentTaskClient: { list: (...args: unknown[]) => list(...args) },
}))
vi.mock('@/api/agentArtifacts', () => ({
  agentArtifactClient: { list: (...args: unknown[]) => artifactList(...args) },
}))

describe('AgentLiveView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    list.mockReset().mockResolvedValue({
      tasks: [{
        taskId: 'TASK-1', status: 'waiting_input', revision: 1, pendingKind: 'input',
        goal: 'Summarize unread work', updatedAtUnixMs: 1,
      }],
      nextCursor: '',
    })
    artifactList.mockReset().mockResolvedValue({
      artifacts: [{ artifactId: 'a'.repeat(64), title: 'Digest', artifactType: 'conversation_digest' }],
      nextCursor: '',
    })
  })

  it('loads owner tasks and recent artifacts for the live board', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    await router.push('/?agent=1&view=live')
    await router.isReady()
    useChatStore().activeKey = ''
    const wrapper = mount(AgentLiveView, { global: { plugins: [router] } })
    await flushPromises()
    expect(list).toHaveBeenCalled()
    expect(wrapper.get('[data-live-current-task]').text()).toContain('TASK-1')
    expect(wrapper.text()).toContain('Digest')
    expect(wrapper.text()).toContain('待我处理')
  })
})
