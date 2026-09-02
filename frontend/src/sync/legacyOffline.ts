import type { Message } from '@/types'

export async function drainLegacyOffline(
  startID: number,
  list: (afterID: number, limit: number) => Promise<Message[]>,
  commitPage: (messages: Message[], nextID: number) => void,
  options: { pageSize?: number; maxPages?: number } = {},
) {
  const pageSize = Math.min(100, Math.max(1, options.pageSize ?? 100))
  const maxPages = Math.max(1, options.maxPages ?? 1000)
  const result: Message[] = []
  let cursor = startID

  for (let pageNumber = 0; pageNumber < maxPages; pageNumber += 1) {
    const page = await list(cursor, pageSize)
    const messages = Array.isArray(page) ? page : []
    if (messages.length === 0) return result
    const nextID = messages.reduce((highest, message) => Math.max(highest, message.id), cursor)
    if (nextID <= cursor) throw new Error('legacy offline cursor did not advance')
    commitPage(messages, nextID)
    result.push(...messages)
    cursor = nextID
    if (messages.length < pageSize) return result
  }
  throw new Error('legacy offline page limit exceeded')
}
