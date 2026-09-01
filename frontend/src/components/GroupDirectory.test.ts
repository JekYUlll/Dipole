import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import GroupDirectory from './GroupDirectory.vue'

const groups = [{ uuid: 'G100', name: 'Atlas Product Guild', notice: '', avatar: '', status: 0, member_count: 24, is_hot: true, recent_message_count: 3, me_role: 1 }]
const stubs = { RouterLink: { template: '<a><slot /></a>' } }

describe('GroupDirectory', () => {
  it('renders the authenticated read-only group projection', async () => {
    const wrapper = mount(GroupDirectory, { props: { client: { list: async () => groups } }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('Atlas Product Guild')
    expect(wrapper.text()).toContain('Hot / Pull')
    expect(wrapper.text()).toContain('只读目录')
    expect(wrapper.find('[data-group-action]').exists()).toBe(false)
  })

  it('clears stale groups and allows an authoritative retry', async () => {
    let calls = 0
    const wrapper = mount(GroupDirectory, { props: { client: { list: async () => { calls += 1; if (calls === 1) throw new Error('unavailable'); return groups } } }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('群目录暂时不可用')
    expect(wrapper.text()).not.toContain('Atlas Product Guild')
    await wrapper.get('[data-group-retry]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Atlas Product Guild')
  })
})
