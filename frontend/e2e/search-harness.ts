import { createApp } from 'vue'
import SearchWorkspace from '../src/components/SearchWorkspace.vue'
import type { Conversation, SearchMessageResult } from '../src/types'

const message: SearchMessageResult = {
  message_id: 'M9', conversation_key: 'group:G1', message_seq: 9, revision: 1,
  from_uuid: 'U2', message_type: 0, content: '数据库迁移本周五完成', sent_at: '2026-08-27T12:30:00Z',
}

const conversations: Conversation[] = [{
  conversation_key: 'group:G1', target_type: 1,
  target_group: { uuid: 'G1', name: 'Dipole 开发群', notice: '', avatar: '', status: 0, me_role: 1, member_count: 3 },
  remark: '', last_message: { message_id: 'M8', message_type: 0, preview: 'ok', sent_at: '2026-08-27T12:00:00Z' },
  unread_count: 0, last_message_seq: 8, read_seq: 8,
}]

const state = new URLSearchParams(window.location.search).get('state') ?? 'idle'
const searcher = async (): Promise<SearchMessageResult[]> => {
  if (state === 'loading') return new Promise(() => undefined)
  if (state === 'error') throw new Error('private search detail')
  return state === 'results' ? [message] : []
}

createApp(SearchWorkspace, {
  conversations,
  currentUser: { uuid: 'U1', nickname: 'Owner', avatar: '' },
  initialQuery: state === 'idle' ? '' : '数据库迁移',
  searcher,
  onClose: () => undefined,
  onSelect: () => undefined,
}).mount('#app')
