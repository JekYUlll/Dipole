import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const root = resolve(new URL('..', import.meta.url).pathname)
const policyPath = resolve(root, 'contracts/multipart-upload/v1/default-policy.json')
const manifestPath = resolve(root, 'contracts/multipart-upload/v1/release-manifest.json')
const configPath = resolve(root, 'configs/config.dist.yaml')
const goConfigPath = resolve(root, 'internal/config/config.go')
const chatViewPath = resolve(root, 'frontend/src/views/ChatView.vue')
const policy = JSON.parse(await readFile(policyPath, 'utf8'))
const manifest = JSON.parse(await readFile(manifestPath, 'utf8'))
const [configSource, goConfigSource, chatViewSource] = await Promise.all([
  readFile(configPath, 'utf8'),
  readFile(goConfigPath, 'utf8'),
  readFile(chatViewPath, 'utf8'),
])

const required = [
  'schema_version', 'policy_version', 'mode', 'fallback_mode',
  'direct_upload_threshold_bytes', 'max_file_size_bytes', 'chunk_size_bytes',
  'max_concurrency', 'max_retries', 'retry_delay_ms', 'presign_url_ttl_seconds',
]
if (policy.schema_version !== 'dipole.multipart-upload.policy.v1') throw new Error('invalid Multipart policy schema version')
if (manifest.schema_version !== 'dipole.multipart-upload.release.v1') throw new Error('invalid Multipart release schema version')
if (manifest.policy_file !== 'contracts/multipart-upload/v1/default-policy.json') throw new Error('release manifest policy path is not canonical')
if (manifest.policy_version !== policy.policy_version) throw new Error('release manifest policy version drift')
if (manifest.default_mode !== policy.mode || manifest.fallback_mode !== policy.fallback_mode) throw new Error('release manifest mode drift')
if (Object.keys(policy).sort().join(',') !== required.slice().sort().join(',')) throw new Error('Multipart policy fields are not closed')
if (policy.fallback_mode !== 'relay') throw new Error('Multipart policy must retain relay fallback')
if (policy.direct_upload_threshold_bytes >= policy.max_file_size_bytes) throw new Error('direct upload threshold must be below max file size')
if (policy.chunk_size_bytes < 5 * 1024 * 1024) throw new Error('Multipart chunk size is below S3 minimum')

const canonical = JSON.stringify(policy) + '\n'
const digest = createHash('sha256').update(canonical, 'utf8').digest('hex')
if (digest !== manifest.policy_sha256) throw new Error(`Multipart policy hash mismatch: ${digest}`)

const storageDefaults = {
  multipart_policy_version: policy.policy_version,
  multipart_mode: policy.mode,
  file_max_size_mb: policy.max_file_size_bytes / (1024 * 1024),
  multipart_chunk_size_mb: policy.chunk_size_bytes / (1024 * 1024),
  multipart_direct_threshold_mb: policy.direct_upload_threshold_bytes / (1024 * 1024),
  multipart_max_concurrency: policy.max_concurrency,
  multipart_max_retries: policy.max_retries,
  multipart_retry_delay_ms: policy.retry_delay_ms,
  multipart_presign_url_ttl_seconds: policy.presign_url_ttl_seconds,
}

for (const [key, value] of Object.entries(storageDefaults)) {
  const configMatch = configSource.match(new RegExp(`^  ${key}:\\s*(.+)$`, 'm'))
  if (!configMatch || configMatch[1].trim() !== String(value)) {
    throw new Error(`config.dist Multipart default drift for storage.${key}`)
  }

  const goMatch = goConfigSource.match(new RegExp(`v\\.SetDefault\\("storage\\.${key}",\\s*([^)]*)\\)`))
  const goLiteral = typeof value === 'string' ? `"${value}"` : String(value)
  if (!goMatch || goMatch[1].trim() !== goLiteral) {
    throw new Error(`Go Multipart default drift for storage.${key}`)
  }
}

const frontendFallbackFragments = [
  `schema_version: '${policy.schema_version}'`,
  `policy_version: '${policy.policy_version}'`,
  `mode: '${policy.mode}'`,
  `fallback_mode: '${policy.fallback_mode}'`,
  `max_file_size_bytes: ${policy.max_file_size_bytes / (1024 * 1024)} * 1024 * 1024`,
  `chunk_size_bytes: ${policy.chunk_size_bytes / (1024 * 1024)} * 1024 * 1024`,
  `max_concurrency: ${policy.max_concurrency}`,
  `max_retries: ${policy.max_retries}`,
  `retry_delay_ms: ${policy.retry_delay_ms}`,
  `presign_url_ttl_seconds: ${policy.presign_url_ttl_seconds}`,
]
for (const fragment of frontendFallbackFragments) {
  if (!chatViewSource.includes(fragment)) {
    throw new Error(`frontend Multipart fallback drift: missing ${fragment}`)
  }
}
const directThresholdExpression = `${policy.direct_upload_threshold_bytes / (1024 * 1024)} * 1024 * 1024`
if (!chatViewSource.includes(`const directUploadThresholdBytes = ${directThresholdExpression}`) ||
    !chatViewSource.includes('direct_upload_threshold_bytes: directUploadThresholdBytes')) {
  throw new Error('frontend Multipart direct upload threshold drift')
}

process.stdout.write(`multipart policy ${policy.policy_version} valid sha256=${digest} mode=${policy.mode} fallback=${policy.fallback_mode} runtime-defaults=aligned\n`)
