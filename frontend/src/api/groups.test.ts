import { describe, expect, it } from 'vitest'

import { parseConversationGroupIDs, parseGroupDirectoryItem } from './groups'

const group = {
  uuid: 'G100', name: 'Atlas Product Guild', notice: '', avatar: '', status: 0,
  member_count: 24, is_hot: false, recent_message_count: 3,
  owner: { uuid: 'U100', nickname: 'Lin', avatar: '', signature: '', user_type: 0, status: 0 },
  me_role: 0, created_at: '2026-08-30T00:00:00Z',
}

const lastMessage = {
  message_id: 'M100', message_type: 0, preview: 'hello', sent_at: '2026-08-30T00:00:00Z', sender_uuid: 'U100',
}

describe('group directory parser', () => {
  it('derives unique group IDs from the authenticated conversation projection', () => {
    expect(parseConversationGroupIDs([
      { conversation_key: 'group:G100', target_type: 1, remark: '', last_message: lastMessage, unread_count: 0, last_message_seq: 2, read_seq: 2 },
      { conversation_key: 'direct:U100:U200', target_type: 0, target_user: group.owner, remark: '', last_message: lastMessage, unread_count: 0, last_message_seq: 2, read_seq: 2 },
      { conversation_key: 'group:G100', target_type: 1, remark: '', last_message: lastMessage, unread_count: 0, last_message_seq: 2, read_seq: 2 },
    ])).toEqual(['G100'])
  })

  it('accepts the bounded group projection and rejects unsafe fields', () => {
    expect(parseGroupDirectoryItem(group)).toMatchObject({ uuid: 'G100', name: 'Atlas Product Guild', member_count: 24 })
    expect(() => parseGroupDirectoryItem({ ...group, secret: 'nope' })).toThrow()
    expect(() => parseGroupDirectoryItem({ ...group, status: 2 })).toThrow()
    expect(() => parseConversationGroupIDs([{ conversation_key: 'group:../../x', target_type: 1, remark: '', last_message: lastMessage, unread_count: 0, last_message_seq: 2, read_seq: 2 }])).toThrow()
  })
})
