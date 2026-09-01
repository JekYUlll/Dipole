import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentSubscriptionManager from './AgentSubscriptionManager.vue'
import type { AgentSubscription, AgentSubscriptionClient } from '@/api/agentSubscriptions'
import type { AgentDefinitionCatalogClient } from '@/api/agentDefinitions'

const source = readFileSync(resolve(import.meta.dirname, 'AgentSubscriptionManager.vue'), 'utf8')

const active: AgentSubscription = {
  subscriptionId: 'SUB-1', definitionId: 'DEF-1', definitionVersion: 7, agentId: 'UAI',
  eventType: 'message.created', resourceType: 'conversation', resourceId: 'group:G123',
  filterKind: 'message_contains_any', filter: { terms: ['事故', '延期'] }, status: 'active',
  createdById: 'U100', createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
}

function service(): AgentSubscriptionClient {
  return {
	create: vi.fn().mockResolvedValue({ ...active, subscriptionId: 'SUB-CREATED', eventType: 'message.group.created' }),
	listEligibleConversations: vi.fn().mockResolvedValue({ conversations: [{ conversationKey: 'group:G123', eventType: 'message.group.created' }] }),
    list: vi.fn().mockResolvedValue({ subscriptions: [active], nextCursor: '' }),
    revoke: vi.fn().mockImplementation(async (_id: string, reason: string) => ({
      ...active, status: 'revoked', revokedById: 'U100', revokeReason: reason,
      revokedAtUnixMs: 3_000, updatedAtUnixMs: 3_000,
    })),
  }
}

function definitions(): AgentDefinitionCatalogClient {
	return { list: vi.fn().mockResolvedValue({ definitions: [{
		definitionId: 'DEF-1', version: 7, agentId: 'UAI', conversationScopes: ['group:G123'],
		validFromUnixMs: 1_000, createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
	}], nextCursor: '' }) }
}

describe('AgentSubscriptionManager', () => {
  it('uses the shared Pencil token theme', () => {
    expect(source).toContain('--paper:var(--dp-canvas)')
    expect(source).toContain('--green:var(--dp-accent-strong)')
    expect(source).toContain('var(--dp-font-body)')
    expect(source).not.toContain('--paper:#f4f6f8')
  })

  it('uses the PNG-derived Agent mark in the control rail', () => {
    expect(source).toContain("import agentMark from '../../../docs/images/dipole-v3-agent-mark-traced.svg'")
    expect(source).toContain(':src="agentMark"')
    expect(source).toContain('class="brand-mark"')
  })

  it('provides a mobile V3 brand bar when the desktop rail collapses', () => {
    expect(source).toContain('class="mobile-brandbar"')
    expect(source).toContain('DIRECT TARGET')
    expect(source).toContain('@media(max-width:900px)')
  })

  it('renders the owner-derived list and opens creation from trusted catalogs', async () => {
    const client = service()
    const definitionClient = definitions()
    const wrapper = mount(AgentSubscriptionManager, { props: { client, definitionClient } })
    await flushPromises()

    expect(client.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.text()).toContain('Project Guardian')
    expect(wrapper.text()).toContain('DIRECT_TARGET')
	expect(wrapper.text()).toContain('创建事件订阅')
	expect(wrapper.get('[data-agent-subscription-create]').attributes('disabled')).toBeUndefined()
	await wrapper.get('[data-agent-subscription-create]').trigger('click')
	await flushPromises()
	expect(definitionClient.list).toHaveBeenCalledWith('', 100)
	expect(client.listEligibleConversations).toHaveBeenCalledWith('DEF-1', 7)
	expect(wrapper.get('[data-agent-subscription-conversation]').element).toBeTruthy()
	await wrapper.get('[data-agent-subscription-submit]').trigger('click')
	await flushPromises()
	expect(client.create).toHaveBeenCalledWith({
		definitionId: 'DEF-1', definitionVersion: 7, conversationKey: 'group:G123', filterKind: 'all', filter: {},
	})
	expect(wrapper.find('#create-title').exists()).toBe(false)
  })

  it('requires an exact reason and replaces the row with authoritative revoke output', async () => {
    const client = service()
    const wrapper = mount(AgentSubscriptionManager, { attachTo: document.body, props: { client, definitionClient: definitions() } })
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

  it('rejects an oversized keyword set without silently truncating user intent', async () => {
    const client = service()
    const wrapper = mount(AgentSubscriptionManager, { props: { client, definitionClient: definitions() } })
    await flushPromises()
    await wrapper.get('[data-agent-subscription-create]').trigger('click')
    await flushPromises()
    await wrapper.get('input[value="message_contains_any"]').setValue(true)
    await wrapper.get('[data-agent-subscription-terms]').setValue(Array.from({ length: 33 }, (_, index) => `关键词${index}`).join(','))

    expect(wrapper.text()).toContain('关键词最多 32 项')
    expect(wrapper.get('[data-agent-subscription-terms]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('[data-agent-subscription-submit]').attributes('disabled')).toBeDefined()
    expect(client.create).not.toHaveBeenCalled()
  })

  it('fails closed instead of hiding additional Definition pages', async () => {
    const definitionClient = definitions()
    vi.mocked(definitionClient.list).mockResolvedValueOnce({ definitions: [], nextCursor: 'NEXT' })
    const wrapper = mount(AgentSubscriptionManager, { props: { client: service(), definitionClient } })
    await flushPromises()
    await wrapper.get('[data-agent-subscription-create]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Active Definition 超过当前选择器上限')
    expect(wrapper.get('[data-agent-subscription-submit]').attributes('disabled')).toBeDefined()
  })

  it('clears stale rows and disables actions when the authority API is unavailable', async () => {
    const client = service()
    vi.mocked(client.list).mockRejectedValueOnce(new Error('private upstream detail'))
    const wrapper = mount(AgentSubscriptionManager, { props: { client, definitionClient: definitions() } })
    await flushPromises()

    expect(wrapper.text()).toContain('订阅控制面暂时不可用')
    expect(wrapper.text()).not.toContain('private upstream detail')
    expect(wrapper.find('[data-agent-subscription-revoke]').exists()).toBe(false)
  })
})
