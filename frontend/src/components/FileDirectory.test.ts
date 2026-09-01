import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import FileDirectory from './FileDirectory.vue'

const item = { file_id: 'F100', file_name: 'handoff.md', file_size: 42, content_type: 'text/markdown', created_at: '2026-08-30T00:00:00.000Z', download_path: '/api/v1/files/F100/download' }
const stubs = { RouterLink: { template: '<a><slot /></a>' } }

describe('FileDirectory', () => {
  it('renders the owner-safe projection and opens a newly authorized download', async () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null)
    const client = { list: vi.fn(async () => ({ files: [item], has_more: false })), download: vi.fn(async () => 'https://signed.example/F100') }
    const wrapper = mount(FileDirectory, { props: { client }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('handoff.md')
    expect(wrapper.text()).toContain('对象键、存储 URL、校验值')
    await wrapper.get('.download').trigger('click')
    await flushPromises()
    expect(client.download).toHaveBeenCalledWith('F100')
    expect(open).toHaveBeenCalledWith('https://signed.example/F100', '_blank', 'noopener,noreferrer')
    open.mockRestore()
  })

  it('clears stale entries after a subsequent page failure', async () => {
    const client = { list: vi.fn().mockResolvedValueOnce({ files: [item], next_cursor: 'F100', has_more: true }).mockRejectedValueOnce(new Error('unavailable')), download: vi.fn() }
    const wrapper = mount(FileDirectory, { props: { client }, global: { stubs } })
    await flushPromises()
    await wrapper.get('.load-more').trigger('click')
    await flushPromises()
    expect(wrapper.attributes('data-file-state')).toBe('unavailable')
    expect(wrapper.text()).not.toContain('handoff.md')
  })
})
