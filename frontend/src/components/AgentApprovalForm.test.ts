import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentApprovalForm from './AgentApprovalForm.vue'
import type { AgentTaskClient, AgentTaskState } from '@/api/agentTasks'

const approvalTask: AgentTaskState = {
  taskId: 'TASK-1', status: 'waiting_approval', revision: 8, persistentStatus: 'waiting_approval',
  workflowProjection: { outcome: 'match', status: 'waiting_approval', revision: 8 },
  pending: { kind: 'approval', requestId: 'REQ-1', approvalId: 'APPROVAL-1', summary: '向项目群发送延期风险提醒', expiresAtUnixMs: 4_000 },
}
const runningTask: AgentTaskState = {
  taskId: 'TASK-1', status: 'running', revision: 9, persistentStatus: 'running',
  workflowProjection: { outcome: 'match', status: 'running', revision: 9 },
}
const client = (states: AgentTaskState[]): AgentTaskClient => ({
  getTask: vi.fn().mockImplementation(async () => states.shift() ?? runningTask),
  provideInput: vi.fn().mockResolvedValue(undefined),
  resolveApproval: vi.fn().mockResolvedValue(undefined),
  cancelTask: vi.fn().mockResolvedValue(undefined),
})

describe('AgentApprovalForm', () => {
  it('renders a pending approval and resumes the exact task binding', async () => {
    const service = client([approvalTask, runningTask])
    const wrapper = mount(AgentApprovalForm, { props: { taskId: 'TASK-1', client: service, now: () => 1_000 } })
    await flushPromises()
    expect(wrapper.text()).toContain('向项目群发送延期风险提醒')
    await wrapper.get('[data-agent-approve]').trigger('click')
    await flushPromises()
    expect(service.resolveApproval).toHaveBeenCalledWith('TASK-1', 'APPROVAL-1', 'approved')
    expect(wrapper.get('[data-agent-approval-state]').attributes('data-agent-approval-state')).toBe('running')
  })

  it('fails closed when the authoritative Task query is unavailable', async () => {
    const service = client([])
    vi.mocked(service.getTask).mockRejectedValueOnce(new Error('upstream detail'))
    const wrapper = mount(AgentApprovalForm, { props: { taskId: 'TASK-1', client: service, now: () => 1_000 } })
    await flushPromises()
    expect(wrapper.text()).toContain('无法确认当前审批请求')
    expect(wrapper.text()).not.toContain('upstream detail')
    expect(wrapper.find('[data-agent-approve]').exists()).toBe(false)
  })
})
