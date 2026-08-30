import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentArtifactMetadata from './AgentArtifactMetadata.vue'

const artifactId = 'a'.repeat(64)
const metadata = {
  artifactId, taskId: 'TASK-1', runId: 'RUN-1', artifactType: 'analysis-report', version: 1,
  title: 'Project digest', mediaType: 'application/json', contentSha256: artifactId,
  sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000,
}

describe('AgentArtifactMetadata', () => {
  it('renders only metadata and the closed disclosure boundary', async () => {
    const wrapper = mount(AgentArtifactMetadata, { props: { artifactId, client: { get: vi.fn().mockResolvedValue(metadata) } } })
    await flushPromises()
    expect(wrapper.attributes('data-agent-artifact-state')).toBe('ready')
    expect(wrapper.text()).toContain('Project digest')
    expect(wrapper.text()).toContain('内容与下载保持关闭')
    expect(wrapper.text()).not.toContain('secret body')
  })

  it('clears stale metadata and offers a retry on failure', async () => {
    const get = vi.fn().mockRejectedValue(new Error('unavailable'))
    const wrapper = mount(AgentArtifactMetadata, { props: { artifactId, client: { get } } })
    await flushPromises()
    expect(wrapper.attributes('data-agent-artifact-state')).toBe('unavailable')
    expect(wrapper.find('[data-agent-artifact-retry]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Project digest')
  })
})
