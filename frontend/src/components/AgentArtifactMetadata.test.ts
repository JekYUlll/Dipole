import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AgentArtifactMetadata from './AgentArtifactMetadata.vue'

const artifactId = 'a'.repeat(64)
const metadata = {
  artifactId, taskId: 'TASK-1', runId: 'RUN-1', artifactType: 'conversation_digest', version: 1,
  title: 'Project digest', mediaType: 'text/markdown', contentSha256: artifactId,
  sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000,
}
const digest = { artifactId, mediaType: 'text/markdown', content: '# Project digest\n- Ship the gateway' }

describe('AgentArtifactMetadata', () => {
  it('renders the verified digest while retaining the closed download boundary', async () => {
    const wrapper = mount(AgentArtifactMetadata, { props: { artifactId, client: { get: vi.fn().mockResolvedValue(metadata), getContent: vi.fn().mockResolvedValue(digest) } } })
    await flushPromises()
    expect(wrapper.attributes('data-agent-artifact-state')).toBe('ready')
    expect(wrapper.text()).toContain('Project digest')
    expect(wrapper.text()).toContain('Ship the gateway')
    expect(wrapper.text()).toContain('下载保持关闭')
    expect(wrapper.find('[data-agent-artifact-content-state="ready"]').exists()).toBe(true)
  })

  it('clears stale metadata and offers a retry on failure', async () => {
    const get = vi.fn().mockRejectedValue(new Error('unavailable'))
    const wrapper = mount(AgentArtifactMetadata, { props: { artifactId, client: { get, getContent: vi.fn() } } })
    await flushPromises()
    expect(wrapper.attributes('data-agent-artifact-state')).toBe('unavailable')
    expect(wrapper.find('[data-agent-artifact-retry]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Project digest')
  })

  it('retains metadata and offers a bounded retry when digest content is unavailable', async () => {
    const getContent = vi.fn().mockRejectedValue(new Error('unavailable'))
    const wrapper = mount(AgentArtifactMetadata, { props: { artifactId, client: { get: vi.fn().mockResolvedValue(metadata), getContent } } })
    await flushPromises()
    expect(wrapper.attributes('data-agent-artifact-state')).toBe('ready')
    expect(wrapper.find('[data-agent-artifact-content-state="unavailable"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Project digest')
    await wrapper.find('[data-agent-artifact-content-retry]').trigger('click')
    expect(getContent).toHaveBeenCalledTimes(2)
  })
})
