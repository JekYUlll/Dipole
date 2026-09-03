import { describe, expect, it, vi } from 'vitest'
import { agentDefinitionCatalogClient, parseAgentDefinitionCatalogPage } from './agentDefinitions'

const definition = {
  definitionId: 'DEF-1', version: 7, agentId: 'UAI', conversationScopes: ['*', 'group:G123'],
  validFromUnixMs: 1000, createdAtUnixMs: 1000, updatedAtUnixMs: 2000,
}

describe('Agent Definition catalog parser', () => {
  it('accepts the bounded public projection', () => {
    expect(parseAgentDefinitionCatalogPage({ definitions: [definition], nextCursor: 'eyJ2ZXJzaW9uIjo3fQ' })).toEqual({
      definitions: [definition], nextCursor: 'eyJ2ZXJzaW9uIjo3fQ',
    })
  })

  it('rejects an unsupported Definition profile before calling the API', async () => {
    await expect((agentDefinitionCatalogClient.create as (profile: string) => Promise<unknown>)('interactive')).rejects.toThrow('unsupported')
  })

  it('rejects authority drift and extra fields', () => {
    expect(() => parseAgentDefinitionCatalogPage({ definitions: [{ ...definition, ownerId: 'U100' }] })).toThrow()
    expect(() => parseAgentDefinitionCatalogPage({ definitions: [{ ...definition, conversationScopes: [] }] })).toThrow()
    expect(() => parseAgentDefinitionCatalogPage({ definitions: [{ ...definition, validFromUnixMs: 0 }] })).toThrow()
  })
})
