import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Banner from './Banner.vue'

describe('Banner', () => {
  it('renders message and tone class', () => {
    const w = mount(Banner, { props: { tone: 'warning', message: 'Definition v3 已失效' } })
    expect(w.text()).toContain('Definition v3 已失效')
    expect(w.classes()).toContain('tone-warning')
  })

  it('emits action and close events', async () => {
    const w = mount(Banner, {
      props: { tone: 'danger', message: '订阅列表失败', actionLabel: 'Retry' },
    })
    await w.find('.banner__action').trigger('click')
    await w.find('.banner__close').trigger('click')
    expect(w.emitted('action')).toHaveLength(1)
    expect(w.emitted('close')).toHaveLength(1)
  })

  it('hides close when closable=false', () => {
    const w = mount(Banner, { props: { message: 'x', closable: false } })
    expect(w.find('.banner__close').exists()).toBe(false)
  })

  it('hides action button when actionLabel is omitted', () => {
    const w = mount(Banner, { props: { message: 'x' } })
    expect(w.find('.banner__action').exists()).toBe(false)
  })
})
