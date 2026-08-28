import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentSubscriptionManager from './AgentSubscriptionManager.vue'
import type { AgentSubscription, AgentSubscriptionClient } from '@/api/agentSubscriptions'

const active: AgentSubscription = {
  subscriptionId: 'SUB-1', definitionId: 'DEF-1', definitionVersion: 7, agentId: 'UAI',
  eventType: 'message.created', resourceType: 'conversation', resourceId: 'group:G123',
  filterKind: 'message_contains_any', filter: { terms: ['事故', '延期'] }, status: 'active',
  createdById: 'U100', createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
}

function service(): AgentSubscriptionClient {
  return {
    list: vi.fn().mockResolvedValue({ subscriptions: [active], nextCursor: '' }),
    revoke: vi.fn().mockImplementation(async (_id: string, reason: string) => ({
      ...active, status: 'revoked', revokedById: 'U100', revokeReason: reason,
      revokedAtUnixMs: 3_000, updatedAtUnixMs: 3_000,
    })),
  }
}

describe('AgentSubscriptionManager', () => {
  it('renders the owner-derived list and keeps creation fail closed', async () => {
    const client = service()
    const wrapper = mount(AgentSubscriptionManager, { props: { client } })
    await flushPromises()

    expect(client.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.text()).toContain('Project Guardian')
    expect(wrapper.text()).toContain('DIRECT_TARGET')
    expect(wrapper.text()).toContain('创建需 Definition 目录')
    expect(wrapper.get('[data-agent-subscription-create]').attributes('disabled')).toBeDefined()
  })

  it('requires an exact reason and replaces the row with authoritative revoke output', async () => {
    const client = service()
    const wrapper = mount(AgentSubscriptionManager, { attachTo: document.body, props: { client } })
    await flushPromises()
    await wrapper.get('[data-agent-subscription-revoke="SUB-1"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[role="dialog"]').attributes('aria-modal')).toBe('true')
    expect(document.activeElement).toBe(wrapper.get('[data-agent-subscription-reason]').element)
    await wrapper.get('[data-agent-subscription-confirm]').trigger('click')
    expect(wrapper.text()).toContain('请输入撤销原因')
    await wrapper.get('[data-agent-subscription-reason]').setValue('项目已归档')
    await wrapper.get('[data-agent-subscription-confirm]').trigger('click')
    await flushPromises()

    expect(client.revoke).toHaveBeenCalledWith('SUB-1', '项目已归档')
    expect(wrapper.text()).toContain('REVOKED')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('clears stale rows and disables actions when the authority API is unavailable', async () => {
    const client = service()
    vi.mocked(client.list).mockRejectedValueOnce(new Error('private upstream detail'))
    const wrapper = mount(AgentSubscriptionManager, { props: { client } })
    await flushPromises()

    expect(wrapper.text()).toContain('订阅控制面暂时不可用')
    expect(wrapper.text()).not.toContain('private upstream detail')
    expect(wrapper.find('[data-agent-subscription-revoke]').exists()).toBe(false)
  })
})
