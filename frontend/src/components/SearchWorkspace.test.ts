import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Conversation, SearchMessageResult } from '@/types'
import SearchWorkspace from './SearchWorkspace.vue'

const conversations: Conversation[] = [{
  conversation_key: 'group:G1',
  target_type: 1,
  target_group: {
    uuid: 'G1', name: 'Dipole 开发群', notice: '', avatar: '', status: 0,
    me_role: 1, member_count: 3,
  },
  remark: '',
  last_message: { message_id: 'M8', message_type: 0, preview: 'ok', sent_at: '2026-08-27T12:00:00Z' },
  unread_count: 0,
  last_message_seq: 8,
  read_seq: 8,
}]

const message: SearchMessageResult = {
  message_id: 'M9', conversation_key: 'group:G1', message_seq: 9, revision: 1,
  from_uuid: 'U2', message_type: 0, content: '数据库迁移本周五完成', sent_at: '2026-08-27T12:30:00Z',
}

describe('SearchWorkspace', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('renders results and emits the selected stable locator', async () => {
    const search = vi.fn().mockResolvedValue([message])
    const wrapper = mount(SearchWorkspace, { props: { conversations, searcher: search } })

    await wrapper.get('input[type="search"]').setValue('数据库迁移')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('Dipole 开发群')
    expect(wrapper.text()).toContain('#9')
    expect(wrapper.text()).toContain('数据库迁移本周五完成')

    await wrapper.get('[data-search-result="M9"]').trigger('click')
    expect(wrapper.emitted('select')).toEqual([[message]])
  })

  it('shows a local failure with a retry action', async () => {
    const search = vi.fn()
      .mockRejectedValueOnce(new Error('secret'))
      .mockResolvedValueOnce([])
    const wrapper = mount(SearchWorkspace, { props: { conversations, searcher: search } })

    await wrapper.get('input[type="search"]').setValue('risk')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('搜索服务未响应')
    expect(wrapper.text()).not.toContain('secret')

    await wrapper.get('[data-search-retry]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('没有找到相关消息')
  })
})
