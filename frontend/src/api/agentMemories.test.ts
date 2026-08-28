import { describe, expect, it } from 'vitest'
import { parseAgentMemoryPage, parseAgentMemoryResponse } from './agentMemories'

const active = {
  memoryId: 'MEM-1', agentId: 'UAI', memoryType: 'semantic', status: 'active',
  resourceType: 'conversation', resourceId: 'group:G1', content: 'Owner is Alice', compactContent: 'Owner: Alice', priority: 80,
  provenance: { sourceType: 'message', sourceId: 'MSG-1', sequence: '42' },
  validFromUnixMs: 1_000, createdAtUnixMs: 2_000,
}

describe('Agent Memory response parser', () => {
  it('accepts exact active and revoked owner contracts', () => {
    expect(parseAgentMemoryPage({ memories: [active], nextCursor: 'eyJjdXJzb3IiOiJN' })).toMatchObject({ memories: [{ memoryId: 'MEM-1' }] })
    expect(parseAgentMemoryResponse({
      ...active, status: 'revoked', revokedAtUnixMs: 3_000, revokedById: 'U100', revokeReason: 'outdated',
    })).toMatchObject({ status: 'revoked', revokeReason: 'outdated' })
  })

  it('rejects authority injection, source URI and inconsistent audit state', () => {
    expect(() => parseAgentMemoryPage({ memories: [{ ...active, principalUserId: 'U999' }] })).toThrow(/shape/i)
    expect(() => parseAgentMemoryPage({ memories: [{ ...active, provenance: { ...active.provenance, uri: 'mysql://secret' } }] })).toThrow(/provenance/i)
    expect(() => parseAgentMemoryPage({ memories: [{ ...active, revokedById: 'U100' }] })).toThrow(/active audit/i)
    expect(() => parseAgentMemoryPage({ memories: [{ ...active, status: 'revoked' }] })).toThrow(/revoked audit/i)
    expect(() => parseAgentMemoryPage({ memories: [], nextCursor: 'bad=' })).toThrow(/cursor/i)
  })
})
