import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import GroupDirectory from './GroupDirectory.vue'

const groups = [{ uuid: 'G100', name: 'Atlas Product Guild', notice: '', avatar: '', status: 0, member_count: 24, is_hot: true, recent_message_count: 3, me_role: 0 }]

describe('GroupDirectory', () => {
  it('renders only the authenticated group projection', async () => {
    const wrapper = mount(GroupDirectory, { props: { client: { list: async () => groups } } })
    await flushPromises()
    expect(wrapper.text()).toContain('Atlas Product Guild')
    expect(wrapper.text()).toContain('Hot / Pull')
    expect(wrapper.find('[data-group-action]').exists()).toBe(false)
  })

  it('clears the directory when the authoritative read fails', async () => {
    const wrapper = mount(GroupDirectory, { props: { client: { list: async () => { throw new Error('unavailable') } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('群目录暂时不可用')
    expect(wrapper.text()).not.toContain('Atlas Product Guild')
    expect(wrapper.find('[data-group-retry]').exists()).toBe(true)
  })
})
