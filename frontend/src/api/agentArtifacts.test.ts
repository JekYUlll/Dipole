import { describe, expect, it, vi } from 'vitest'
import { agentArtifactClient, parseAgentArtifactContent, parseAgentArtifactMetadata, parseAgentArtifactPage } from './agentArtifacts'

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

  it('lists owner metadata pages and rejects leaked content or object location', async () => {
    const api = (await import('@/api')).default
    vi.mocked(api.get).mockResolvedValueOnce({ artifacts: [valid], nextCursor: `1725000000000:${'b'.repeat(64)}` } as never)
    await expect(agentArtifactClient.list!('', 20)).resolves.toMatchObject({ artifacts: [valid], nextCursor: `1725000000000:${'b'.repeat(64)}` })
    expect(api.get).toHaveBeenCalledWith('/api/v1/agent/artifacts?limit=20')
    expect(() => parseAgentArtifactPage({ artifacts: [{ ...valid, objectKey: 'private' }], nextCursor: '' })).toThrow('metadata')
    expect(() => parseAgentArtifactPage({ artifacts: [valid], nextCursor: '', content: 'secret' })).toThrow('catalog')
    await expect(agentArtifactClient.list!('bad cursor', 20)).rejects.toThrow(/cursor/i)
    await expect(agentArtifactClient.list!('', 101)).rejects.toThrow(/limit/i)
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
