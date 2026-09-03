import { describe, expect, it } from 'vitest'
import { parseAgentMemoryCandidatePage, parseAgentMemoryPage, parseAgentMemoryResponse } from './agentMemories'

const active = {
  memoryId: 'MEM-1', agentId: 'UAI', memoryType: 'semantic', status: 'active',
  resourceType: 'conversation', resourceId: 'group:G1', content: 'Owner is Alice', compactContent: 'Owner: Alice', priority: 80,
  provenance: { sourceType: 'message', sourceId: 'MSG-1', sequence: '42' },
  validFromUnixMs: 1_000, createdAtUnixMs: 2_000,
  memoryRootId: 'MEM-1', memoryVersion: 1,
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

  it('accepts canonical correction lineage and rejects forged predecessors', () => {
    const corrected = {
      ...active, memoryId: 'MEM-2', memoryVersion: 2, memoryRootId: 'MEM-1', supersedesMemoryId: 'MEM-1',
      correctedById: 'U100', correctionReason: 'fix owner', content: 'Owner is Bob', compactContent: 'Owner: Bob',
      provenance: { sourceType: 'owner_correction', sourceId: 'MEM-1', sequence: '2' },
    }
    expect(parseAgentMemoryResponse(corrected)).toMatchObject({ memoryVersion: 2, supersedesMemoryId: 'MEM-1' })
    expect(() => parseAgentMemoryResponse({ ...corrected, supersedesMemoryId: 'MEM-X' })).toThrow(/lineage/i)
    expect(() => parseAgentMemoryResponse({ ...active, memoryVersion: 2 })).toThrow(/lineage/i)
  })

  it('accepts owned candidates with optional review and rejects injected fields', () => {
    const candidate = {
      candidateId: 'CAND-1', candidateSha256: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
      summary: 'API v2 Friday', status: 'accepted', reviewId: 'REV-1', observedAtUnixMs: 1_000,
    }
    expect(parseAgentMemoryCandidatePage({ candidates: [candidate], nextCursor: '' })).toMatchObject({
      candidates: [{ candidateId: 'CAND-1', reviewId: 'REV-1' }],
    })
    expect(parseAgentMemoryCandidatePage({
      candidates: [{ candidateId: 'CAND-2', candidateSha256: candidate.candidateSha256, summary: 'later', status: 'pending', observedAtUnixMs: 2_000 }],
      nextCursor: '',
    }).candidates[0].status).toBe('pending')
    expect(() => parseAgentMemoryCandidatePage({ candidates: [{ ...candidate, principalUserId: 'U999' }], nextCursor: '' })).toThrow(/shape/i)
    expect(() => parseAgentMemoryCandidatePage({ candidates: [{ ...candidate, candidateSha256: 'ZZ' }], nextCursor: '' })).toThrow(/digest/i)
    expect(() => parseAgentMemoryCandidatePage({ candidates: [{ ...candidate, reviewId: undefined }], nextCursor: '' })).toThrow(/review/i)
  })
})
