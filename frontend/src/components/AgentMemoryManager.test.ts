import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentMemoryManager from './AgentMemoryManager.vue'
import type { AgentMemory, AgentMemoryClient } from '@/api/agentMemories'

const active: AgentMemory = {
  memoryId: 'MEM-1', agentId: 'UAI', memoryType: 'semantic', status: 'active',
  resourceType: 'conversation', resourceId: 'group:G1', content: '项目 A 的数据库迁移负责人是 Alice', compactContent: 'Owner: Alice', priority: 80,
  provenance: { sourceType: 'message', sourceId: 'MSG-1', sequence: '42' },
  validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_100_000,
}

function service(): AgentMemoryClient {
  return {
    list: vi.fn().mockResolvedValue({ memories: [active], nextCursor: '' }),
    revoke: vi.fn().mockImplementation(async (_id: string, reason: string) => ({
      ...active, status: 'revoked', revokedAtUnixMs: 1_700_000_200_000, revokedById: 'U100', revokeReason: reason,
    })),
  }
}

describe('AgentMemoryManager', () => {
  it('renders owner-visible provenance and trust boundaries', async () => {
    const client = service()
    const wrapper = mount(AgentMemoryManager, { props: { client } })
    await flushPromises()

    expect(client.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.text()).toContain('长期记忆')
    expect(wrapper.text()).toContain('UNTRUSTED MEMORY')
    expect(wrapper.text()).toContain('AUTO WRITE')
    expect(wrapper.text()).toContain('MSG-1')
    expect(wrapper.text()).toContain('纠正将在版本化写入后开放')
  })

  it('requires a reason and replaces the row only with authoritative revoke output', async () => {
    const client = service()
    const wrapper = mount(AgentMemoryManager, { attachTo: document.body, props: { client } })
    await flushPromises()
    await wrapper.get('[data-agent-memory-revoke="MEM-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').attributes('aria-modal')).toBe('true')
    expect(document.activeElement).toBe(wrapper.get('[data-agent-memory-reason]').element)
    await wrapper.get('[data-agent-memory-confirm]').trigger('click')
    expect(wrapper.text()).toContain('请输入撤销原因')
    await wrapper.get('[data-agent-memory-reason]').setValue('信息已经过时')
    await wrapper.get('[data-agent-memory-confirm]').trigger('click')
    await flushPromises()

    expect(client.revoke).toHaveBeenCalledWith('MEM-1', '信息已经过时')
    expect(wrapper.text()).toContain('REVOKED')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('clears stale records and disables controls when authority is unavailable', async () => {
    const client = service()
    vi.mocked(client.list).mockRejectedValueOnce(new Error('private upstream detail'))
    const wrapper = mount(AgentMemoryManager, { props: { client } })
    await flushPromises()

    expect(wrapper.text()).toContain('记忆控制面暂时不可用')
    expect(wrapper.text()).not.toContain('private upstream detail')
    expect(wrapper.find('[data-agent-memory-revoke]').exists()).toBe(false)
  })
})
