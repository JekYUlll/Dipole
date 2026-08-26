import { onScopeDispose, ref, watch } from 'vue'
import type { SearchMessageResult } from '@/types'

export type MessageSearch = (query: string, limit: number) => Promise<SearchMessageResult[]>
export type MessageSearchStatus = 'idle' | 'loading' | 'results' | 'empty' | 'error'

export const useMessageSearch = (search: MessageSearch, delay = 300, limit = 20) => {
  const query = ref('')
  const results = ref<SearchMessageResult[]>([])
  const status = ref<MessageSearchStatus>('idle')
  const errorMessage = ref('')
  let timer: ReturnType<typeof setTimeout> | undefined
  let requestVersion = 0

  const clearTimer = () => {
    if (timer !== undefined) {
      clearTimeout(timer)
      timer = undefined
    }
  }

  const reset = () => {
    requestVersion += 1
    results.value = []
    errorMessage.value = ''
    status.value = 'idle'
  }

  const run = async () => {
    clearTimer()
    const normalized = [...query.value.trim()].slice(0, 256).join('')
    if (!normalized) {
      reset()
      return
    }

    const currentVersion = ++requestVersion
    status.value = 'loading'
    errorMessage.value = ''
    try {
      const messages = await search(normalized, limit)
      if (currentVersion !== requestVersion) return
      results.value = messages
      status.value = messages.length > 0 ? 'results' : 'empty'
    } catch {
      if (currentVersion !== requestVersion) return
      results.value = []
      errorMessage.value = '搜索服务暂时不可用'
      status.value = 'error'
    }
  }

  const retry = () => run()

  watch(query, (value) => {
    clearTimer()
    requestVersion += 1
    if (!value.trim()) {
      reset()
      return
    }
    results.value = []
    errorMessage.value = ''
    status.value = 'loading'
    timer = setTimeout(run, delay)
  }, { flush: 'sync' })

  onScopeDispose(() => {
    clearTimer()
    requestVersion += 1
  })

  return { query, results, status, errorMessage, retry, run }
}
