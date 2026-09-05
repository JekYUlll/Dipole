import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { IconContacts, IconGroups, IconSearch, IconUsers } from './index'

describe('nav icons', () => {
  it('keeps contacts and groups as distinct glyphs', () => {
    const contacts = mount(IconContacts, { props: { size: 22 } })
    const groups = mount(IconGroups, { props: { size: 22 } })
    expect(contacts.html()).not.toBe(groups.html())
    expect(contacts.find('circle').exists()).toBe(true)
    expect(groups.findAll('circle').length).toBeGreaterThan(contacts.findAll('circle').length)
  })

  it('pins search icon to a square so CSS cannot stretch the magnifier', () => {
    const search = mount(IconSearch, { props: { size: 14 } })
    const svg = search.get('svg')
    expect(svg.attributes('width')).toBe('14')
    expect(svg.attributes('height')).toBe('14')
    expect(svg.attributes('style')).toContain('width: 14px')
    expect(svg.attributes('style')).toContain('height: 14px')
    expect(svg.attributes('style')).toContain('overflow: visible')
  })

  it('keeps IconUsers aligned with the group glyph', () => {
    const users = mount(IconUsers, { props: { size: 20 } })
    const groups = mount(IconGroups, { props: { size: 20 } })
    expect(users.html()).toBe(groups.html())
  })
})
