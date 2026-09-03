import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentMemoryManager from './AgentMemoryManager.vue'
import type { AgentMemory, AgentMemoryCandidate, AgentMemoryClient } from '@/api/agentMemories'

const source = readFileSync(resolve(import.meta.dirname, 'AgentMemoryManager.vue'), 'utf8')

const active: AgentMemory = {
  memoryId: 'MEM-1', agentId: 'UAI', memoryType: 'semantic', status: 'active',
  resourceType: 'conversation', resourceId: 'group:G1', content: '项目 A 的数据库迁移负责人是 Alice', compactContent: 'Owner: Alice', priority: 80,
  provenance: { sourceType: 'message', sourceId: 'MSG-1', sequence: '42' },
  validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_100_000,
  memoryRootId: 'MEM-1', memoryVersion: 1,
}

const acceptedCandidate: AgentMemoryCandidate = {
  candidateId: 'CAND-1', candidateSha256: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  summary: 'API v2 Friday', status: 'accepted', reviewId: 'REV-1', observedAtUnixMs: 1_700_000_000_000,
}

function service(): AgentMemoryClient {
  return {
    list: vi.fn().mockResolvedValue({ memories: [active], nextCursor: '' }),
    listCandidates: vi.fn().mockResolvedValue({ candidates: [], nextCursor: '' }),
    promoteCandidate: vi.fn(),
    revoke: vi.fn().mockImplementation(async (_id: string, reason: string) => ({
      ...active, status: 'revoked', revokedAtUnixMs: 1_700_000_200_000, revokedById: 'U100', revokeReason: reason,
    })),
    correct: vi.fn().mockImplementation(async (_id: string, expectedVersion: number, content: string, compactContent: string, reason: string) => ({
      previous: { ...active, status: 'revoked', revokedAtUnixMs: 1_700_000_200_000, revokedById: 'U100', revokeReason: 'superseded by MEM-2' },
      corrected: {
        ...active, memoryId: 'MEM-2', content, compactContent, memoryVersion: expectedVersion + 1,
        supersedesMemoryId: 'MEM-1', correctedById: 'U100', correctionReason: reason,
        provenance: { sourceType: 'owner_correction', sourceId: 'MEM-1', sequence: '2' },
      },
    })),
  }
}

describe('AgentMemoryManager', () => {
  it('overrides the legacy theme with shared Pencil tokens', () => {
    expect(source).toContain('--ink: var(--dp-ink)')
    expect(source).toContain('--panel: var(--dp-surface)')
    expect(source).toContain('font-family: var(--dp-font-body)')
  })

  it('renders owner-visible provenance and trust boundaries', async () => {
    const client = service()
    const wrapper = mount(AgentMemoryManager, { props: { client } })
    await flushPromises()

    expect(client.list).toHaveBeenCalledWith('', 50)
    expect(wrapper.text()).toContain('长期记忆')
    expect(wrapper.text()).toContain('UNTRUSTED MEMORY')
    expect(wrapper.text()).toContain('AUTO WRITE')
    expect(wrapper.text()).toContain('MSG-1')
    expect(wrapper.text()).toContain('版本化纠正默认关闭')
    expect(wrapper.find('[data-agent-memory-correct]').exists()).toBe(false)
  })

  it('appends an authoritative correction version only when explicitly enabled', async () => {
    const client = service()
    const wrapper = mount(AgentMemoryManager, { attachTo: document.body, props: { client, correctionEnabled: true } })
    await flushPromises()
    await wrapper.get('[data-agent-memory-correct="MEM-1"]').trigger('click')
    await flushPromises()
    expect(document.activeElement).toBe(wrapper.get('[data-agent-memory-correction-content]').element)
    await wrapper.get('[data-agent-memory-correction-content]').setValue('项目 A 的数据库迁移负责人是 Bob')
    await wrapper.get('[data-agent-memory-correction-compact]').setValue('Owner: Bob')
    await wrapper.get('[data-agent-memory-correction-confirm]').trigger('click')
    expect(wrapper.text()).toContain('请输入纠正原因')
    await wrapper.get('[data-agent-memory-correction-reason]').setValue('负责人信息已确认')
    await wrapper.get('[data-agent-memory-correction-confirm]').trigger('click')
    await flushPromises()

    expect(client.correct).toHaveBeenCalledWith('MEM-1', 1, '项目 A 的数据库迁移负责人是 Bob', 'Owner: Bob', '负责人信息已确认')
    expect(wrapper.text()).toContain('V2')
    expect(wrapper.text()).toContain('REVOKED')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
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

  it('promotes an accepted reviewed candidate using listed digest and review', async () => {
    const client = service()
    vi.mocked(client.listCandidates).mockResolvedValue({ candidates: [acceptedCandidate], nextCursor: '' })
    vi.mocked(client.promoteCandidate).mockResolvedValue({
      ...active, memoryId: 'MEM-CAND-1', memoryRootId: 'MEM-CAND-1', memoryType: 'observational',
      content: acceptedCandidate.summary, compactContent: acceptedCandidate.summary,
      provenance: { sourceType: 'memory_candidate', sourceId: 'CAND-1', sequence: 'REV-1' },
    })
    const wrapper = mount(AgentMemoryManager, { props: { client } })
    await flushPromises()
    expect(wrapper.text()).toContain('API v2 Friday')
    expect(wrapper.text()).toContain('READY TO PROMOTE')
    await wrapper.get('[data-agent-memory-candidate-promote="CAND-1"]').trigger('click')
    await flushPromises()
    expect(client.promoteCandidate).toHaveBeenCalledWith('CAND-1', acceptedCandidate.candidateSha256, 'REV-1')
    expect(wrapper.text()).toContain('已晋升 MEM-CAND-1')
    expect(wrapper.text()).toContain('MEM-CAND-1')
  })
})
