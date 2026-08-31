import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

for (const file of ['AgentTaskCreateView.vue', 'AgentTaskTimelineView.vue']) {
  const source = readFileSync(resolve(import.meta.dirname, file), 'utf8')

  describe(`${file} brand contract`, () => {
    it('preserves the traced Agent mark aspect ratio', () => {
      expect(source).toContain('const agentMark = `${import.meta.env.BASE_URL}dipole-v3-agent.svg`')
      expect(source).toContain('object-fit: contain')
    })
  })
}
