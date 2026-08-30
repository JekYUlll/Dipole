import { describe, expect, it } from 'vitest'

import { parseContactList } from './contacts'

const user = {
  uuid: 'U100', nickname: 'Lin Qiao', avatar: '', signature: 'Project partner', user_type: 0, status: 0,
}

describe('contact directory parser', () => {
  it('accepts the bounded authenticated contact projection', () => {
    expect(parseContactList([{ user, remark: 'Migration review', status: 0 }])).toEqual([
      { user, remark: 'Migration review', status: 0 },
    ])
  })

  it('rejects unexpected or unsafe contact data', () => {
    expect(() => parseContactList([{ user, remark: 'x', status: 2 }])).toThrow()
    expect(() => parseContactList([{ user: { ...user, password: 'nope' }, remark: '', status: 0 }])).toThrow()
    expect(() => parseContactList([{ user, remark: 99, status: 0 }])).toThrow()
  })
})
