import { readFile } from 'node:fs/promises'
import { access } from 'node:fs/promises'
import process from 'node:process'

const root = new URL('..', import.meta.url)
const files = [
  'docs/images/LOGO_V3.png',
  'scripts/trace-brand-assets.sh',
  'docs/images/dipole-v3-im-traced.svg',
  'docs/images/dipole-v3-im-mark-traced.svg',
  'docs/images/dipole-v3-agent-traced.svg',
  'docs/images/dipole-v3-agent-mark-traced.svg',
]

for (const relativePath of files) {
  await access(new URL(relativePath, root))
}

for (const relativePath of files.slice(2)) {
  const svg = await readFile(new URL(relativePath, root), 'utf8')
  if (!svg.includes('<path') || /<image|data:image/i.test(svg)) {
    throw new Error(`brand SVG is not a path-only trace: ${relativePath}`)
  }
}

const source = await readFile(new URL('scripts/trace-brand-assets.sh', root), 'utf8')
if (!source.includes('LOGO_V3.png') || !source.includes('vtracer')) {
  throw new Error('brand trace script must reference the approved PNG and VTracer')
}

const references = {
  'frontend/src/components/ContactDirectory.vue': 'dipole-v3-im-mark-traced.svg',
  'frontend/src/components/GroupDirectory.vue': 'dipole-v3-im-mark-traced.svg',
  'frontend/src/components/DeviceDirectory.vue': 'dipole-v3-im-mark-traced.svg',
  'frontend/src/components/FileDirectory.vue': 'dipole-v3-im-mark-traced.svg',
  'frontend/src/views/SettingsView.vue': 'dipole-v3-im-mark-traced.svg',
  'frontend/src/views/LoginView.vue': 'dipole-v3-im-traced.svg',
  'frontend/src/views/AgentDefinitionsView.vue': 'dipole-v3-agent-mark-traced.svg',
  'frontend/src/components/AgentMemoryManager.vue': 'dipole-v3-agent-mark-traced.svg',
  'frontend/src/components/AgentSubscriptionManager.vue': 'dipole-v3-agent-mark-traced.svg',
}

for (const [relativePath, asset] of Object.entries(references)) {
  const sourceText = await readFile(new URL(relativePath, root), 'utf8')
  if (!sourceText.includes(asset)) {
    throw new Error(`${relativePath} must reference ${asset}`)
  }
}

console.log(`Brand asset gate passed: ${files.length} assets and ${Object.keys(references).length} page references`)
