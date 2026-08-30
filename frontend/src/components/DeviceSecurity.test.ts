import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import DeviceSecurity from './DeviceSecurity.vue'

const sessions = [
  { connection_id: 'C-current', device: 'web', device_id: 'web-current', connected_at: '2026-08-30T00:00:00.000Z', last_seen_at: new Date().toISOString() },
  { connection_id: 'C-other', device: 'desktop', device_id: 'web-other', connected_at: '2026-08-30T00:00:00.000Z', last_seen_at: new Date().toISOString() },
]

describe('DeviceSecurity', () => {
  it('keeps the current device intact and requires confirmation before logging out others', async () => {
    const client = { list: vi.fn(async () => sessions), logout: vi.fn(), logoutOthers: vi.fn(async () => {}) }
    const wrapper = mount(DeviceSecurity, { props: { client, currentDeviceID: 'web-current' } })
    await flushPromises()
    expect(wrapper.text()).toContain('浏览器会话')
    expect(wrapper.text()).toContain('当前会话')
    expect(wrapper.text()).not.toContain('C-current')
    await wrapper.get('[data-device-logout-others]').trigger('click')
    expect(client.logoutOthers).not.toHaveBeenCalled()
    await wrapper.get('[data-device-confirm-logout]').trigger('click')
    await flushPromises()
    expect(client.logoutOthers).toHaveBeenCalledTimes(1)
  })

  it('clears stale device entries when the follow-up read fails', async () => {
    const client = { list: vi.fn().mockResolvedValueOnce(sessions).mockRejectedValueOnce(new Error('unavailable')), logout: vi.fn(async () => {}), logoutOthers: vi.fn() }
    const wrapper = mount(DeviceSecurity, { props: { client, currentDeviceID: 'web-current' } })
    await flushPromises()
    await wrapper.get('.logout-one').trigger('click')
    await wrapper.get('[data-device-confirm-logout]').trigger('click')
    await flushPromises()
    expect(wrapper.attributes('data-device-state')).toBe('unavailable')
    expect(wrapper.text()).not.toContain('桌面设备')
  })
})
