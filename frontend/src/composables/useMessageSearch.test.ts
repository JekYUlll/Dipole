import { effectScope, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SearchMessageResult } from '@/types'
import { useMessageSearch, type MessageSearch } from './useMessageSearch'

const result = (messageId: string, content = messageId): SearchMessageResult => ({
  message_id: messageId,
  conversation_key: 'group:G1',
  message_seq: 9,
  revision: 1,
  from_uuid: 'U2',
  message_type: 0,
  content,
  sent_at: '2026-08-27T12:30:00Z',
})

const deferred = <T>() => {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((ok, fail) => {
    resolve = ok
    reject = fail
  })
  return { promise, resolve, reject }
}

describe('useMessageSearch', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('debounces non-empty queries and clears an empty query locally', async () => {
    const search = vi.fn<MessageSearch>().mockResolvedValue([result('M1')])
    const scope = effectScope()
    const state = scope.run(() => useMessageSearch(search))!

    state.query.value = '数据库迁移'
    await vi.advanceTimersByTimeAsync(299)
    expect(search).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(search).toHaveBeenCalledWith('数据库迁移', 20)
    await nextTick()
    expect(state.status.value).toBe('results')

    state.query.value = '   '
    await nextTick()
    expect(state.status.value).toBe('idle')
    expect(state.results.value).toEqual([])
    scope.stop()
  })

  it('keeps the newest response when requests resolve out of order', async () => {
    const first = deferred<SearchMessageResult[]>()
    const second = deferred<SearchMessageResult[]>()
    const search = vi.fn<MessageSearch>()
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise)
    const scope = effectScope()
    const state = scope.run(() => useMessageSearch(search))!

    state.query.value = 'first'
    await vi.advanceTimersByTimeAsync(300)
    state.query.value = 'second'
    await vi.advanceTimersByTimeAsync(300)

    second.resolve([result('M2', 'new')])
    await Promise.resolve()
    first.resolve([result('M1', 'old')])
    await Promise.resolve()

    expect(state.results.value.map(item => item.message_id)).toEqual(['M2'])
    expect(state.status.value).toBe('results')
    scope.stop()
  })

  it('exposes a bounded error state and retries the current query', async () => {
    const search = vi.fn<MessageSearch>()
      .mockRejectedValueOnce(new Error('internal dependency detail'))
      .mockResolvedValueOnce([])
    const scope = effectScope()
    const state = scope.run(() => useMessageSearch(search))!

    state.query.value = 'risk'
    await vi.advanceTimersByTimeAsync(300)
    expect(state.status.value).toBe('error')
    expect(state.errorMessage.value).toBe('搜索服务暂时不可用')

    await state.retry()
    expect(search).toHaveBeenLastCalledWith('risk', 20)
    expect(state.status.value).toBe('empty')
    scope.stop()
  })

  it('bounds requests by Unicode code points', async () => {
    const search = vi.fn<MessageSearch>().mockResolvedValue([])
    const scope = effectScope()
    const state = scope.run(() => useMessageSearch(search))!

    state.query.value = '😀'.repeat(300)
    await vi.advanceTimersByTimeAsync(300)

    const sentQuery = search.mock.calls[0][0]
    expect([...sentQuery]).toHaveLength(256)
    scope.stop()
  })
})
