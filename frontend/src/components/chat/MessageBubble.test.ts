import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MessageBubble from './MessageBubble.vue'

describe('MessageBubble', () => {
  it('renders a self bubble with the outgoing corner', () => {
    const wrapper = mount(MessageBubble, {
      props: { variant: 'self', initials: 'EJ' },
      slots: { default: '我让 agent 先跑一次' },
    })
    expect(wrapper.attributes('data-message-variant')).toBe('self')
    expect(wrapper.text()).toContain('我让 agent 先跑一次')
    expect(wrapper.find('.msg-bubble').exists()).toBe(true)
    expect(wrapper.find('.msg-system').exists()).toBe(false)
  })

  it('renders an incoming other bubble with avatar fallback', () => {
    const wrapper = mount(MessageBubble, {
      props: { variant: 'other', initials: 'A', senderName: 'Alice', showSender: true },
      slots: { default: '看一下这个 PR' },
    })
    expect(wrapper.find('.msg-sender-name').text()).toBe('Alice')
    expect(wrapper.find('.msg-avatar-fallback').text()).toBe('A')
  })

  it('renders an AI bubble with the agent accent stripe class', () => {
    const wrapper = mount(MessageBubble, {
      props: { variant: 'ai', initials: 'AI' },
      slots: { default: 'diff-summary 已生成' },
    })
    expect(wrapper.classes()).toContain('ai')
    expect(wrapper.find('.msg-bubble').exists()).toBe(true)
  })

  it('renders a system strip without a bubble', () => {
    const wrapper = mount(MessageBubble, {
      props: { variant: 'system' },
      slots: { default: 'agent-uai 已接收任务' },
    })
    expect(wrapper.find('.msg-system').text()).toContain('agent-uai 已接收任务')
    expect(wrapper.find('.msg-bubble').exists()).toBe(false)
  })

  it('emits avatar-click and sender-click', async () => {
    const wrapper = mount(MessageBubble, {
      props: { variant: 'other', senderName: 'Alice', showSender: true, initials: 'A' },
    })
    await wrapper.find('.msg-avatar').trigger('click')
    await wrapper.find('.msg-sender-name').trigger('click')
    expect(wrapper.emitted('avatar-click')).toHaveLength(1)
    expect(wrapper.emitted('sender-click')).toHaveLength(1)
  })

  it('marks media bubbles so the slot can drop padding', () => {
    const wrapper = mount(MessageBubble, {
      props: { variant: 'self', media: true, initials: 'EJ' },
      slots: { default: '<img alt="pic" />' },
    })
    expect(wrapper.find('.msg-bubble').classes()).toContain('media')
  })
})
