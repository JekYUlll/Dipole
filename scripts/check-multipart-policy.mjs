import { createHash } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const root = resolve(new URL('..', import.meta.url).pathname)
const policyPath = resolve(root, 'contracts/multipart-upload/v1/default-policy.json')
const manifestPath = resolve(root, 'contracts/multipart-upload/v1/release-manifest.json')
const policy = JSON.parse(await readFile(policyPath, 'utf8'))
const manifest = JSON.parse(await readFile(manifestPath, 'utf8'))

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
process.stdout.write(`multipart policy ${policy.policy_version} valid sha256=${digest} mode=${policy.mode} fallback=${policy.fallback_mode}\n`)
