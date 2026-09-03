import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'ChatView.vue'), 'utf8')

describe('ChatView Agent waiting locator', () => {
  it('consumes the low-sensitivity waiting event and links to the owner inbox', () => {
    expect(source).toContain("case 'agent_task_waiting'")
    expect(source).toContain('taskWaiting.acceptLocator(data)')
    expect(source).toContain("refreshFromInbox('replace')")
    expect(source).toContain('data-agent-task-waiting')
    expect(source).toContain("router.push({ name: 'agent-task-inbox' })")
    expect(source).not.toContain('pending.summary')
    expect(source).not.toContain('form.fields')
  })
})
