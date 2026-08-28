import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentTaskTimeline from './AgentTaskTimeline.vue'

const source = readFileSync(resolve(import.meta.dirname, 'AgentTaskTimeline.vue'), 'utf8')

describe('AgentTaskTimeline', () => {
  it('uses the shared Pencil token surface for the timeline shell', () => {
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-line)')
    expect(source).toContain('var(--dp-font-body)')
    expect(source).not.toContain('#fbfaf7')
    expect(source).not.toContain('#b66a43')
  })

  it('renders a low-sensitivity page and follows its cursor', async () => {
    const getTimeline = vi.fn()
      .mockResolvedValueOnce({ schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 2, events: [{ eventSeq: '1', eventId: 'EV-1', taskId: 'TASK-1', runId: 'RUN-1', kind: 'model_call', status: 'completed', occurredAtUnixMs: 1_000 }], nextCursor: '1' })
      .mockResolvedValueOnce({ schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 3, events: [{ eventSeq: '2', eventId: 'EV-2', taskId: 'TASK-1', runId: 'RUN-1', kind: 'terminal', status: 'completed', occurredAtUnixMs: 2_000 }], nextCursor: '' })
    const wrapper = mount(AgentTaskTimeline, { props: { taskId: 'TASK-1', client: { getTimeline, getTask: vi.fn(), provideInput: vi.fn(), resolveApproval: vi.fn(), cancelTask: vi.fn() } } })
    await flushPromises()
    expect(wrapper.find('[data-event-seq="1"]').exists()).toBe(true)
    await wrapper.get('[data-agent-timeline-more]').trigger('click')
    await flushPromises()
    expect(getTimeline).toHaveBeenNthCalledWith(2, 'TASK-1', '1')
    expect(wrapper.find('[data-event-seq="2"]').exists()).toBe(true)
  })

  it('clears events and shows retry when the authoritative endpoint fails', async () => {
    const getTimeline = vi.fn().mockRejectedValue(new Error('unavailable'))
    const wrapper = mount(AgentTaskTimeline, { props: { taskId: 'TASK-1', client: { getTimeline, getTask: vi.fn(), provideInput: vi.fn(), resolveApproval: vi.fn(), cancelTask: vi.fn() } } })
    await flushPromises()
    expect(wrapper.find('[data-agent-timeline-retry]').exists()).toBe(true)
    expect(wrapper.find('[data-event-seq]').exists()).toBe(false)
  })
})
