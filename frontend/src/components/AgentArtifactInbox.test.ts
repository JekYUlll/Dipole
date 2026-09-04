import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import AgentArtifactInbox from './AgentArtifactInbox.vue'
import type { AgentArtifactClient, AgentArtifactMetadata } from '@/api/agentArtifacts'

const source = readFileSync(resolve(import.meta.dirname, 'AgentArtifactInbox.vue'), 'utf8')
const artifactId = 'a'.repeat(64)
const item: AgentArtifactMetadata = {
  artifactId, taskId: 'TASK-1', runId: 'RUN-1', artifactType: 'conversation_digest', version: 1,
  title: 'Project digest', mediaType: 'text/markdown', contentSha256: artifactId,
  sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000,
}

function client(artifacts = [item], nextCursor = ''): AgentArtifactClient {
  return {
    list: vi.fn().mockResolvedValue({ artifacts, nextCursor }),
    get: vi.fn(),
    getContent: vi.fn(),
  }
}

function mountInbox(artifactClient: AgentArtifactClient) {
  return mount(AgentArtifactInbox, {
    props: { client: artifactClient },
    global: { stubs: { RouterLink: { props: ['to'], template: '<a :data-route-name="to.name"><slot /></a>' } } },
  })
}

describe('AgentArtifactInbox', () => {
  it('uses the shared Pencil token surface for the inbox shell', () => {
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-font-body)')
    expect(source).toContain('OWNER INBOX')
    expect(source).not.toContain('getContent')
    expect(source).not.toContain('objectKey')
  })

  it('renders owner metadata and links to the digest page without reading content', async () => {
    const artifactClient = client()
    const wrapper = mountInbox(artifactClient)
    await flushPromises()

    expect(artifactClient.list).toHaveBeenCalledWith('', 50)
    expect(artifactClient.getContent).not.toHaveBeenCalled()
    expect(wrapper.get(`[data-agent-artifact-id="${artifactId}"]`).text()).toContain('Project digest')
    expect(wrapper.get('[data-agent-artifact-inbox-open]').attributes('data-route-name')).toBe('agent-artifact')
    expect(wrapper.text()).not.toContain('Ship the gateway')
  })

  it('appends only a distinct next page', async () => {
    const second = { ...item, artifactId: 'b'.repeat(64), contentSha256: 'b'.repeat(64), title: 'Second digest' }
    const artifactClient = client([item], '1725000000000:' + 'b'.repeat(64))
    vi.mocked(artifactClient.list!)
      .mockResolvedValueOnce({ artifacts: [item], nextCursor: '1725000000000:' + 'b'.repeat(64) })
      .mockResolvedValueOnce({ artifacts: [second], nextCursor: '' })
    const wrapper = mountInbox(artifactClient)
    await flushPromises()
    await wrapper.get('[data-agent-artifact-inbox-more]').trigger('click')
    await flushPromises()

    expect(artifactClient.list).toHaveBeenLastCalledWith('1725000000000:' + 'b'.repeat(64), 50)
    expect(wrapper.findAll('[data-agent-artifact-id]').length).toBe(2)
  })

  it('hides the control rail when embedded', async () => {
    const wrapper = mount(AgentArtifactInbox, {
      props: { client: client(), embedded: true },
      global: { stubs: { RouterLink: { props: ['to'], template: '<a><slot /></a>' } } },
    })
    await flushPromises()
    expect(wrapper.find('.control-rail').exists()).toBe(false)
  })
})
