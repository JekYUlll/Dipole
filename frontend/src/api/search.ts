import api from '@/api'
import type { SearchMessageResult } from '@/types'

export const searchMessages = async (query: string, limit = 20): Promise<SearchMessageResult[]> => {
  const data = await api.get('/api/v1/messages/search', {
    params: { q: query, limit },
  }) as SearchMessageResult[]
  return Array.isArray(data) ? data : []
}
