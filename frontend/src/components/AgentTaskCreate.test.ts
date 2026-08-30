import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AgentTaskCreate from './AgentTaskCreate.vue'

const replace = vi.fn()
const source = readFileSync(resolve(import.meta.dirname, 'AgentTaskCreate.vue'), 'utf8')
vi.mock('vue-router', () => ({ useRouter: () => ({ replace }) }))

describe('AgentTaskCreate', () => {
  beforeEach(() => replace.mockReset())

  it('uses established design tokens without inventing an inverse color', () => {
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-canvas)')
    expect(source).toContain('var(--dp-accent)')
    expect(source).not.toContain('--dp-ink-inverse')
  })

  it('rejects an empty goal without calling the API', async () => {
    const startTask = vi.fn()
    const wrapper = mount(AgentTaskCreate, { props: { client: { startTask, getTask: vi.fn(), provideInput: vi.fn(), resolveApproval: vi.fn(), cancelTask: vi.fn() } } })
    await wrapper.get('[data-agent-task-create-form]').trigger('submit')
    expect(startTask).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('1 到 4000')
  })

  it('submits a local idempotency key and redirects only after acceptance', async () => {
    const startTask = vi.fn().mockResolvedValue({ taskId: 'TASK-1', status: 'accepted' })
    const wrapper = mount(AgentTaskCreate, { props: { requestId: () => 'local:001', client: { startTask, getTask: vi.fn(), provideInput: vi.fn(), resolveApproval: vi.fn(), cancelTask: vi.fn() } } })
    await wrapper.get('[data-agent-task-goal]').setValue(' Summarize unread work ')
    await wrapper.get('[data-agent-task-create-form]').trigger('submit')
    await flushPromises()
    expect(startTask).toHaveBeenCalledWith({ clientRequestId: 'local:001', goal: 'Summarize unread work' })
    expect(replace).toHaveBeenCalledWith({ name: 'agent-task-timeline', params: { taskId: 'TASK-1' } })
  })

  it('does not expose a fallback when task creation is unavailable', async () => {
    const wrapper = mount(AgentTaskCreate, { props: { client: { getTask: vi.fn(), provideInput: vi.fn(), resolveApproval: vi.fn(), cancelTask: vi.fn() } } })
    await wrapper.get('[data-agent-task-goal]').setValue('Summarize unread work')
    await wrapper.get('[data-agent-task-create-form]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('暂不可用')
    expect(replace).not.toHaveBeenCalled()
  })
})
