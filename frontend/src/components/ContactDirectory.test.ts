import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ContactDirectory from './ContactDirectory.vue'

const contacts = [{
  user: { uuid: 'U100', nickname: 'Lin Qiao', avatar: '', signature: 'Project partner', user_type: 0, status: 0 },
  remark: 'Migration review', status: 0,
}]

describe('ContactDirectory', () => {
  it('renders only the authenticated contact projection', async () => {
    const wrapper = mount(ContactDirectory, { props: { client: { list: async () => contacts } } })
    await flushPromises()

    expect(wrapper.text()).toContain('Lin Qiao')
    expect(wrapper.text()).toContain('Migration review')
    expect(wrapper.text()).toContain('只读目录')
    expect(wrapper.find('[data-contact-action]').exists()).toBe(false)
  })

  it('keeps the directory empty when the authoritative read fails', async () => {
    const wrapper = mount(ContactDirectory, { props: { client: { list: async () => { throw new Error('unavailable') } } } })
    await flushPromises()

    expect(wrapper.text()).toContain('联系人目录暂时不可用')
    expect(wrapper.text()).not.toContain('Lin Qiao')
    expect(wrapper.get('[data-contact-retry]').exists()).toBe(true)
  })
})
