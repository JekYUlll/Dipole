import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentDefinitionsView from './AgentDefinitionsView.vue'
import type { AgentDefinitionCatalogClient } from '@/api/agentDefinitions'

function client(): AgentDefinitionCatalogClient {
  return { list: vi.fn().mockResolvedValue({ definitions: [{
    definitionId: 'DEF-1', version: 7, agentId: 'UAI', conversationScopes: ['group:G123', 'direct:U100:UAI'],
    validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_000_000, updatedAtUnixMs: 1_700_000_100_000,
  }], nextCursor: '' }) }
}

describe('AgentDefinitionsView', () => {
  it('renders the owner-scoped active catalog without edit controls', async () => {
    const catalog = client()
    const wrapper = mount(AgentDefinitionsView, { props: { client: catalog } })
    await flushPromises()

    expect(catalog.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.text()).toContain('Project Guardian')
    expect(wrapper.text()).toContain('group:G123')
    expect(wrapper.text()).toContain('VERSION 7')
    expect(wrapper.find('[data-agent-definition-id="DEF-1"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('编辑定义')
  })

  it('clears stale catalog rows when Core is unavailable', async () => {
    const catalog = client()
    vi.mocked(catalog.list).mockRejectedValueOnce(new Error('private upstream detail'))
    const wrapper = mount(AgentDefinitionsView, { props: { client: catalog } })
    await flushPromises()

    expect(wrapper.text()).toContain('定义目录暂时不可用')
    expect(wrapper.text()).not.toContain('private upstream detail')
    expect(wrapper.find('[data-agent-definition-id]').exists()).toBe(false)
  })
})
