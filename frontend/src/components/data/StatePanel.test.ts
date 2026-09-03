import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import StatePanel from './StatePanel.vue'

describe('StatePanel', () => {
  it('renders spinner in loading state', () => {
    const w = mount(StatePanel, { props: { state: 'loading', title: '读取中', hint: '请稍候' } })
    expect(w.classes()).toContain('state-loading')
    expect(w.find('.state-panel__spinner').exists()).toBe(true)
    expect(w.text()).toContain('读取中')
  })

  it('emits action when clicked', async () => {
    const w = mount(StatePanel, { props: { state: 'unavailable', title: '不可用', actionLabel: '重试' } })
    await w.find('.state-panel__action').trigger('click')
    expect(w.emitted('action')).toHaveLength(1)
    expect(w.classes()).toContain('state-unavailable')
  })

  it('hides action when actionLabel omitted', () => {
    const w = mount(StatePanel, { props: { state: 'empty', title: '暂无' } })
    expect(w.find('.state-panel__action').exists()).toBe(false)
  })
})
