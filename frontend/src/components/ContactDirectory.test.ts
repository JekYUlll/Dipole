import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import ContactDirectory from './ContactDirectory.vue'

const contacts = [{
  user: { uuid: 'U100', nickname: 'Lin Qiao', avatar: '', signature: 'Project partner', user_type: 0, status: 0 },
  remark: 'Migration review', status: 0,
}]

describe('ContactDirectory', () => {
  it('renders only the authenticated contact projection', async () => {
    const wrapper = mount(ContactDirectory, {
      props: { client: { list: async () => contacts } },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Lin Qiao')
    expect(wrapper.text()).toContain('Migration review')
    expect(wrapper.text()).toContain('只读目录')
    expect(wrapper.find('[data-contact-action]').exists()).toBe(false)
  })

  it('keeps the directory empty until a failed read is retried', async () => {
    let calls = 0
    const wrapper = mount(ContactDirectory, {
      props: { client: { list: async () => {
        calls += 1
        if (calls === 1) throw new Error('unavailable')
        return contacts
      } } },
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()
    expect(wrapper.text()).toContain('联系人目录暂时不可用')
    expect(wrapper.text()).not.toContain('Lin Qiao')
    await wrapper.get('[data-contact-retry]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Lin Qiao')
  })

  it('uses the traced V3 IM asset for the brand surface', () => {
    const source = readFileSync(resolve(import.meta.dirname, 'ContactDirectory.vue'), 'utf8')
    expect(source).toContain("dipole-v3-im-traced.svg")
  })
})
