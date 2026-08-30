import { describe, expect, it, vi } from 'vitest'
import { agentArtifactClient, parseAgentArtifactMetadata } from './agentArtifacts'

const artifactId = 'a'.repeat(64)
const valid = {
  artifactId,
  taskId: 'TASK-1',
  runId: 'RUN-1',
  artifactType: 'analysis-report',
  version: 1,
  title: 'Project digest',
  mediaType: 'application/json',
  contentSha256: artifactId,
  sizeBytes: 18432,
  createdAtUnixMs: 1_725_000_000_000,
}

vi.mock('@/api', () => ({ default: { get: vi.fn(async () => valid) } }))

describe('Agent Artifact metadata API', () => {
  it('accepts the exact low-sensitive metadata shape', () => {
    expect(parseAgentArtifactMetadata(valid)).toEqual(valid)
  })

  it('rejects content-address drift and unexpected body fields', () => {
    expect(() => parseAgentArtifactMetadata({ ...valid, contentSha256: 'b'.repeat(64) })).toThrow('content address')
    expect(() => parseAgentArtifactMetadata({ ...valid, content: 'secret' })).toThrow('metadata')
  })

  it('uses only the content-addressed metadata route', async () => {
    await expect(agentArtifactClient.get(artifactId)).resolves.toEqual(valid)
    const api = (await import('@/api')).default
    expect(api.get).toHaveBeenCalledWith(`/api/v1/agent/artifacts/${artifactId}`)
    await expect(agentArtifactClient.get('short-id')).rejects.toThrow('ID')
  })
})
