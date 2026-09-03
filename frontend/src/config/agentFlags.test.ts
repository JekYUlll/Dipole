import { describe, expect, it, vi } from 'vitest'

describe('agentFlags', () => {
  it('keeps default Vite unit tests off the Agent surfaces', async () => {
    vi.resetModules()
    delete (window as Record<string, unknown>).__DIPOLE_FLAGS__
    const { agentTaskCreatePageEnabled, agentSettingsLinks, agentControlHome } = await import('./agentFlags')
    expect(agentTaskCreatePageEnabled).toBe(false)
    expect(agentSettingsLinks()).toEqual([])
    expect(agentControlHome()).toBeUndefined()
  })

  it('reads runtime flags from window.__DIPOLE_FLAGS__ when present', async () => {
    vi.resetModules()
    ;(window as Record<string, unknown>).__DIPOLE_FLAGS__ = {
      taskCreate: true, timeline: true, definitions: true, memories: true,
      subscriptions: true, artifacts: true,
    }
    const { agentFlags, agentTaskCreatePageEnabled, agentSettingsLinks } = await import('./agentFlags')
    expect(agentFlags.taskCreate).toBe(true)
    expect(agentFlags.timeline).toBe(true)
    expect(agentFlags.definitions).toBe(true)
    expect(agentFlags.memories).toBe(true)
    expect(agentTaskCreatePageEnabled).toBe(true)
    expect(agentSettingsLinks().map(l => l.id)).toContain('create')
    delete (window as Record<string, unknown>).__DIPOLE_FLAGS__
  })
})
