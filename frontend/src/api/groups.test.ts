import { describe, expect, it } from 'vitest'

import { parseConversationGroupIDs, parseGroupDirectoryItem } from './groups'

const user = { uuid: 'U100', nickname: 'Lin Qiao', avatar: '', signature: 'Project partner', user_type: 0, status: 0 }
const lastMessage = { message_id: 'M100', message_type: 0, preview: 'ship it', sent_at: '2026-09-01T10:00:00Z', sender_uuid: 'U100' }

describe('group directory parser', () => {
  it('extracts only authenticated group conversations and accepts detailed members', () => {
    expect(parseConversationGroupIDs([
      { conversation_key: 'group:G100', target_type: 1, target_group: {}, remark: '', last_message: lastMessage, unread_count: 1, last_message_seq: 3, read_seq: 2 },
      { conversation_key: 'direct:U100:U200', target_type: 0, target_user: user, remark: '', last_message: lastMessage, unread_count: 0, last_message_seq: 1, read_seq: 1 },
    ])).toEqual(['G100'])

    expect(parseGroupDirectoryItem({
      uuid: 'G100', name: 'Atlas Product Guild', notice: '', avatar: '', status: 0,
      member_count: 24, is_hot: true, recent_message_count: 3, owner: user, me_role: 1,
      members: [{ user, role: 1, joined_at: '2026-09-01T10:00:00Z' }], created_at: '2026-09-01T10:00:00Z',
    }).name).toBe('Atlas Product Guild')
  })

  it('rejects malformed or out-of-scope directory data', () => {
    expect(() => parseConversationGroupIDs([{ conversation_key: 'group:bad space', target_type: 1, remark: '', last_message: lastMessage, unread_count: 0, last_message_seq: 0, read_seq: 0 }])).toThrow()
    expect(() => parseGroupDirectoryItem({ uuid: 'G100', name: 'x', notice: '', avatar: '', status: 0, member_count: 1, is_hot: true, recent_message_count: 0, me_role: 1, created_at: 'now', members: [{ user: { ...user, token: 'secret' }, role: 1, joined_at: 'now' }] })).toThrow()
  })
})
