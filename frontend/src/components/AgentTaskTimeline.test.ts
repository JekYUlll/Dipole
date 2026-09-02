import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentTaskTimeline from './AgentTaskTimeline.vue'

const source = readFileSync(resolve(import.meta.dirname, 'AgentTaskTimeline.vue'), 'utf8')
const defaultClient = () => ({
  getTimeline: vi.fn(),
  getTask: vi.fn(),
  provideInput: vi.fn(),
  resolveApproval: vi.fn(),
  cancelTask: vi.fn(),
})

function mountTimeline(client: ReturnType<typeof defaultClient>) {
  return mount(AgentTaskTimeline, {
    props: { taskId: 'TASK-1', client },
    global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-route-name="to.name"><slot /></a>' } } },
  })
}

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
    const wrapper = mountTimeline({ ...defaultClient(), getTimeline })
    await flushPromises()
    expect(wrapper.find('[data-event-seq="1"]').exists()).toBe(true)
    await wrapper.get('[data-agent-timeline-more]').trigger('click')
    await flushPromises()
    expect(getTimeline).toHaveBeenNthCalledWith(2, 'TASK-1', '1')
    expect(wrapper.find('[data-event-seq="2"]').exists()).toBe(true)
  })

  it('clears events and shows retry when the authoritative endpoint fails', async () => {
    vi.useFakeTimers()
    const getTimeline = vi.fn().mockRejectedValue(new Error('unavailable'))
    const wrapper = mountTimeline({ ...defaultClient(), getTimeline })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1_000)
    await flushPromises()
    expect(wrapper.find('[data-agent-timeline-retry]').exists()).toBe(true)
    expect(wrapper.find('[data-event-seq]').exists()).toBe(false)
    vi.useRealTimers()
  })

  it('retries the initial Timeline read once while a newly admitted Task is projected', async () => {
    vi.useFakeTimers()
    const getTimeline = vi.fn()
      .mockRejectedValueOnce(new Error('not found yet'))
      .mockResolvedValueOnce({ schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 1, events: [], nextCursor: '' })
    const wrapper = mountTimeline({ ...defaultClient(), getTimeline })

    await flushPromises()
    expect(getTimeline).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1_000)
    await flushPromises()

    expect(getTimeline).toHaveBeenCalledTimes(2)
    expect(wrapper.attributes('data-state')).toBe('ready')
    vi.useRealTimers()
  })

  it('links only validated Artifact events to the metadata surface', async () => {
    const getTimeline = vi.fn().mockResolvedValue({ schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 2, events: [{ eventSeq: '1', eventId: 'EV-1', taskId: 'TASK-1', runId: 'RUN-1', kind: 'artifact', status: 'created', artifactId: 'a'.repeat(64), occurredAtUnixMs: 1_000 }], nextCursor: '' })
    const wrapper = mountTimeline({ ...defaultClient(), getTimeline })
    await flushPromises()
    expect(wrapper.get('.artifact-link').text()).toBe('查看 Artifact metadata')
  })

  it('links a waiting approval event to its owner-scoped approval surface', async () => {
    const getTimeline = vi.fn().mockResolvedValue({
      schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 2,
      events: [{ eventSeq: '1', eventId: 'EV-1', taskId: 'TASK-1', runId: 'RUN-1', kind: 'approval', status: 'waiting_approval', approvalId: 'APPROVAL-1', occurredAtUnixMs: 1_000 }], nextCursor: '',
    })
    const wrapper = mountTimeline({ ...defaultClient(), getTimeline })
    await flushPromises()
    expect(wrapper.get('.approval-link').text()).toBe('处理审批请求')
    expect(wrapper.get('.approval-link').attributes('data-route-name')).toBe('agent-task-approval')
  })
})
