import { describe, expect, it } from 'vitest'
import { parseAgentSubscriptionPage, parseAgentSubscriptionResponse } from './agentSubscriptions'

const active = {
  subscriptionId: 'SUB-1', definitionId: 'DEF-1', definitionVersion: 7, agentId: 'UAI',
  eventType: 'message.created', resourceType: 'conversation', resourceId: 'group:G123',
  filterKind: 'message_contains_any', filter: { terms: ['事故', '延期'] }, status: 'active',
  createdById: 'U100', createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
}

describe('Agent Subscription response parser', () => {
  it('accepts exact list and revoked audit contracts', () => {
    expect(parseAgentSubscriptionPage({ subscriptions: [active], nextCursor: 'SUB-1' })).toMatchObject({
      nextCursor: 'SUB-1', subscriptions: [{ definitionVersion: 7, filter: { terms: ['事故', '延期'] } }],
    })
    expect(parseAgentSubscriptionResponse({
      ...active, filterKind: 'all', filter: {}, status: 'revoked', revokedById: 'U100',
      revokeReason: 'project archived', revokedAtUnixMs: 3_000, updatedAtUnixMs: 3_000,
    })).toMatchObject({ status: 'revoked', revokeReason: 'project archived' })
  })

  it('rejects malformed filters, authority drift and inconsistent revocation state', () => {
    expect(() => parseAgentSubscriptionPage({ subscriptions: [{ ...active, filter: { terms: [] } }] })).toThrow(/filter/i)
    expect(() => parseAgentSubscriptionPage({ subscriptions: [{ ...active, filter: { terms: [' 延期'] } }] })).toThrow(/filter/i)
    expect(() => parseAgentSubscriptionPage({ subscriptions: [{ ...active, principalUserId: 'U999' }] })).toThrow(/shape/i)
    expect(() => parseAgentSubscriptionPage({ subscriptions: [{ ...active, revokedById: 'U100' }] })).toThrow(/active/i)
    expect(() => parseAgentSubscriptionPage({ subscriptions: [], nextCursor: ' ' })).toThrow(/cursor/i)
  })
})
