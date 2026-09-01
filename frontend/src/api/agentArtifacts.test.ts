import { describe, expect, it, vi } from 'vitest'
import { agentArtifactClient, parseAgentArtifactContent, parseAgentArtifactMetadata } from './agentArtifacts'

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

const digest = { artifactId, mediaType: 'text/markdown', content: '# Project digest\n- Ship the gateway' }

vi.mock('@/api', () => ({ default: { get: vi.fn(async (path: string) => path.endsWith('/content') ? digest : valid) } }))

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

  it('accepts the constrained digest content shape and uses its dedicated route', async () => {
    expect(parseAgentArtifactContent(digest)).toEqual(digest)
    expect(() => parseAgentArtifactContent({ ...digest, objectKey: 'private' })).toThrow('content')
    await expect(agentArtifactClient.getContent(artifactId)).resolves.toEqual(digest)
    const api = (await import('@/api')).default
    expect(api.get).toHaveBeenCalledWith(`/api/v1/agent/artifacts/${artifactId}/content`)
  })
})
