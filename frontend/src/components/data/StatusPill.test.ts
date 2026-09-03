import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import StatusPill from './StatusPill.vue'

describe('StatusPill', () => {
  it('renders label uppercase and tone-specific classes', () => {
    const w = mount(StatusPill, { props: { label: 'running', tone: 'agent' } })
    expect(w.text()).toBe('running')
    expect(w.classes()).toContain('tone-agent')
    expect(w.find('.status-pill__dot').exists()).toBe(true)
  })

  it('hides dot when dot=false', () => {
    const w = mount(StatusPill, { props: { label: 'ok', tone: 'success', dot: false } })
    expect(w.find('.status-pill__dot').exists()).toBe(false)
    expect(w.classes()).toContain('no-dot')
  })

  it('renders in dense mode', () => {
    const w = mount(StatusPill, { props: { label: 'x', tone: 'danger', dense: true } })
    expect(w.classes()).toContain('dense')
  })

  it('defaults to neutral tone', () => {
    const w = mount(StatusPill, { props: { label: 'idle' } })
    expect(w.classes()).toContain('tone-neutral')
  })
})
