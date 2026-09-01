import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import DeviceDirectory from './DeviceDirectory.vue'

const sessions = [{ connection_id: 'C100', device: 'Chrome on Linux', device_id: 'D100', user_agent: 'Chrome 140', remote_addr: '10.0.0.1', node_id: 'N1', connected_at: '2026-09-01T10:00:00Z', last_seen_at: '2026-09-01T10:01:00Z' }]
const stubs = { RouterLink: { template: '<a><slot /></a>' } }

describe('DeviceDirectory', () => {
  it('renders owner-scoped sessions and supports single logout', async () => {
    const logout = vi.fn(async () => {})
    const wrapper = mount(DeviceDirectory, { props: { client: { list: async () => sessions, logout, logoutAll: async () => {} } }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('Chrome on Linux')
    expect(wrapper.text()).toContain('仅限本人')
    await wrapper.get('.session-card button').trigger('click')
    await flushPromises()
    expect(logout).toHaveBeenCalledWith('C100')
    expect(wrapper.text()).toContain('当前没有在线设备')
  })

  it('clears stale sessions when the authority is unavailable', async () => {
    const wrapper = mount(DeviceDirectory, { props: { client: { list: async () => { throw new Error('unavailable') }, logout: async () => {}, logoutAll: async () => {} } }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('设备会话暂时不可用')
    expect(wrapper.text()).not.toContain('Chrome on Linux')
  })
})
