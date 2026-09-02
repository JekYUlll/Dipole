import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentDefinitionCatalog from './AgentDefinitionCatalog.vue'
import type { AgentDefinitionCatalogClient, AgentDefinitionCatalogItem } from '@/api/agentDefinitions'

const source = readFileSync(resolve(import.meta.dirname, 'AgentDefinitionCatalog.vue'), 'utf8')
const definition: AgentDefinitionCatalogItem = {
  definitionId: 'DEF-1', version: 7, agentId: 'UAI', conversationScopes: ['group:G123', 'direct:U100:U200'],
  validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_000_000, updatedAtUnixMs: 1_700_000_060_000,
}

function client(): AgentDefinitionCatalogClient {
  return { list: vi.fn().mockResolvedValue({ definitions: [definition], nextCursor: '' }) }
}

describe('AgentDefinitionCatalog', () => {
  it('uses shared Pencil tokens and keeps the page read-only', () => {
    expect(source).toContain('--app:var(--dp-canvas)')
    expect(source).toContain('CATALOG ONLY')
    expect(source).toContain('RUNTIME DISABLED')
    expect(source).not.toMatch(/<input|<textarea/)
  })

  it('renders the owner-scoped exact catalog projection', async () => {
    const catalog = client()
    const wrapper = mount(AgentDefinitionCatalog, { props: { client: catalog } })
    await flushPromises()

    expect(catalog.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.get('[data-agent-definition-id="DEF-1"]').text()).toContain('VERSION 07')
    expect(wrapper.text()).toContain('group:G123')
    expect(wrapper.text()).toContain('RUNTIME DISABLED')
  })

  it('appends only a distinct next page', async () => {
    const catalog = client()
    vi.mocked(catalog.list)
      .mockResolvedValueOnce({ definitions: [definition], nextCursor: 'NEXT' })
      .mockResolvedValueOnce({ definitions: [{ ...definition, definitionId: 'DEF-2', version: 8 }], nextCursor: '' })
    const wrapper = mount(AgentDefinitionCatalog, { props: { client: catalog } })
    await flushPromises()
    await wrapper.get('[data-agent-definition-more]').trigger('click')
    await flushPromises()

    expect(catalog.list).toHaveBeenLastCalledWith('NEXT', 50)
    expect(wrapper.findAll('[data-agent-definition-id]').length).toBe(2)
  })

  it('fails closed and removes stale rows when a next page overlaps', async () => {
    const catalog = client()
    vi.mocked(catalog.list)
      .mockResolvedValueOnce({ definitions: [definition], nextCursor: 'NEXT' })
      .mockResolvedValueOnce({ definitions: [definition], nextCursor: '' })
    const wrapper = mount(AgentDefinitionCatalog, { props: { client: catalog } })
    await flushPromises()
    await wrapper.get('[data-agent-definition-more]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Definition 目录暂时不可用')
    expect(wrapper.find('[data-agent-definition-id]').exists()).toBe(false)
  })
})
