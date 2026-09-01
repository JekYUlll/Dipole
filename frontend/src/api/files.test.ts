import { describe, expect, it } from 'vitest'
import { parseOwnedFileDirectoryPage } from './files'

const item = { file_id: 'F100', file_name: 'handoff.md', file_size: 42, content_type: 'text/markdown', created_at: '2026-08-30T00:00:00.000Z', download_path: '/api/v1/files/F100/download' }

describe('owned file directory projection', () => {
  it('accepts only the low-sensitivity owner projection', () => {
    expect(parseOwnedFileDirectoryPage({ files: [item], next_cursor: 'F100', has_more: true })).toEqual({ files: [item], next_cursor: 'F100', has_more: true })
  })

  it('rejects storage metadata and mismatched download paths', () => {
    expect(() => parseOwnedFileDirectoryPage({ files: [{ ...item, object_key: 'secret' }], has_more: false })).toThrow('item')
    expect(() => parseOwnedFileDirectoryPage({ files: [{ ...item, download_path: '/api/v1/files/F101/download' }], has_more: false })).toThrow('item')
  })
})
