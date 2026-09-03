import { describe, expect, it } from 'vitest'
import { agentControlHome, agentSettingsLinks, agentTaskCreatePageEnabled } from './agentFlags'

describe('agentFlags', () => {
  it('keeps default Vite unit tests off the Agent surfaces', () => {
    expect(agentTaskCreatePageEnabled).toBe(false)
    expect(agentSettingsLinks()).toEqual([])
    expect(agentControlHome()).toBeUndefined()
  })
})
