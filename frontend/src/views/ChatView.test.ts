import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'ChatView.vue'), 'utf8')

describe('ChatView V3 design contract', () => {
  it('does not reintroduce the retired legacy green token', () => {
    expect(source).not.toContain('#07c160')
    expect(source).toContain('var(--dp-accent)')
  })

  it('renders the restoring sync status once', () => {
    expect(source.match(/<span v-if="chat\.syncStatus === 'restoring'">正在恢复<\/span>/g)).toHaveLength(1)
  })

  it('keeps the main chat surfaces on the V3 token layer', () => {
    expect(source).toContain('--chat-canvas: var(--dp-canvas)')
    expect(source).toContain('--chat-rail: var(--dp-rail)')
    expect(source).toContain('.im-container .session-panel')
    expect(source).toContain('.im-container .msg-item.ai .msg-bubble')
    expect(source).toContain('var(--dp-warning-soft)')
  })
})
