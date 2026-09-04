import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'
import AgentSubscriptionsView from './AgentSubscriptionsView.vue'
import type { AgentSubscription, AgentSubscriptionClient } from '@/api/agentSubscriptions'
import type { AgentDefinitionCatalogClient } from '@/api/agentDefinitions'

const active: AgentSubscription = {
  subscriptionId: 'SUB-1', definitionId: 'DEF-1', definitionVersion: 7, agentId: 'UAI',
  eventType: 'message.created', resourceType: 'conversation', resourceId: 'group:G123',
  filterKind: 'message_contains_any', filter: { terms: ['事故'] }, status: 'active',
  createdById: 'U100', createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
}

function client(list = vi.fn().mockResolvedValue({ subscriptions: [active], nextCursor: '' })): AgentSubscriptionClient {
  return {
    list,
    create: vi.fn(),
    revoke: vi.fn(),
    listEligibleConversations: vi.fn().mockResolvedValue({ conversations: [] }),
  }
}

function definitions(): AgentDefinitionCatalogClient {
  return { list: vi.fn().mockResolvedValue({ definitions: [], nextCursor: '' }) }
}

function mountView(subscriptionClient: AgentSubscriptionClient) {
  return mount(AgentSubscriptionsView, {
    props: { client: subscriptionClient, definitionClient: definitions() },
    global: {
      plugins: [PrimeVue, ConfirmationService, ToastService],
      stubs: { DataTable: { props: ['value'], template: '<div class="dt"><slot /></div>' }, Column: true, Dialog: true, ConfirmDialog: true, Select: true, RadioButton: true, InputText: true, Skeleton: { template: '<div class="p-skeleton" />' } },
    },
  })
}

describe('AgentSubscriptionsView', () => {
  it('shows skeleton rows while the first page loads', () => {
    const wrapper = mountView(client(vi.fn(() => new Promise(() => {}))))
    expect(wrapper.findAll('.p-skeleton').length).toBeGreaterThan(0)
    expect(wrapper.find('.state-card').exists()).toBe(false)
  })

  it('renders an empty row plus create CTA instead of a hero card', async () => {
    const wrapper = mountView(client(vi.fn().mockResolvedValue({ subscriptions: [], nextCursor: '' })))
    await flushPromises()
    expect(wrapper.get('.empty-row').text()).toContain('还没有事件订阅')
    expect(wrapper.find('[data-agent-subscription-create-empty]').exists()).toBe(true)
    expect(wrapper.find('.state-card').exists()).toBe(false)
  })

  it('keeps a banner on failure instead of a full-page state card', async () => {
    const wrapper = mountView(client(vi.fn().mockRejectedValue(new Error('private upstream detail'))))
    await flushPromises()
    expect(wrapper.text()).toContain('订阅列表读取失败')
    expect(wrapper.text()).not.toContain('private upstream detail')
    expect(wrapper.find('.state-card').exists()).toBe(false)
    expect(wrapper.find('.banner').exists()).toBe(true)
  })
})
