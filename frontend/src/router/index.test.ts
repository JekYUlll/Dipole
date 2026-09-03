import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'index.ts'), 'utf8')

describe('Agent route security contract', () => {
  it('keeps every Agent page authenticated and independently flag-gated', () => {
    for (const routeName of [
      'agent-task-inbox',
      'agent-task-create',
      'agent-task-input',
      'agent-task-approval',
      'agent-task-timeline',
      'agent-artifact-inbox',
      'agent-artifact',
      'agent-subscriptions',
      'agent-definitions',
      'agent-memories',
    ]) {
      expect(source).toContain(`name: '${routeName}'`)
      expect(source).toMatch(new RegExp(`name: '${routeName}'[\\s\\S]{0,220}?meta: \\{ requiresAuth: true \\}`))
    }

    expect(source).toContain("import.meta.env.VITE_AGENT_ELICITATION_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_TASK_CREATE_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_APPROVAL_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_TIMELINE_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_ARTIFACTS_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_SUBSCRIPTIONS_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_DEFINITIONS_ENABLED === 'true'")
    expect(source).toContain("import.meta.env.VITE_AGENT_MEMORIES_ENABLED === 'true'")
    expect(source).toContain("return { name: 'chat' }")
  })

  it('keeps the Contact directory authenticated without an Agent feature flag', () => {
    expect(source).toContain("name: 'contacts'")
    expect(source).toContain("path: '/contacts'")
  })

  it('keeps the Group directory authenticated without a write feature flag', () => {
    expect(source).toContain("name: 'groups'")
    expect(source).toContain("path: '/groups'")
  })

  it('keeps the owner file directory authenticated without exposing upload controls', () => {
    expect(source).toContain("name: 'files'")
    expect(source).toContain("path: '/files'")
  })

  it('keeps Device Security authenticated and preserves the explicit logout-other-device semantics', () => {
    expect(source).toContain("name: 'devices'")
    expect(source).toContain("path: '/devices'")
  })

  it('keeps unauthenticated access redirected to Login', () => {
    expect(source).toContain("if (to.meta.requiresAuth && !auth.token)")
    expect(source).toContain("return { name: 'login' }")
  })
})
