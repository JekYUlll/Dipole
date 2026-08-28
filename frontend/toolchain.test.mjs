import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { once } from 'node:events'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { createServer as createHTTPServer } from 'node:http'
import { connect } from 'node:net'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import test from 'node:test'
import { build, createServer, resolveConfig } from 'vite'

const projectRoot = new URL('.', import.meta.url).pathname
const configFile = resolve(projectRoot, 'vite.config.ts')

test('Vite config preserves the Dipole base path and output boundary', async () => {
  const config = await resolveConfig({ configFile }, 'build')
  assert.equal(config.base, '/app/')
  assert.equal(config.build.outDir, resolve(projectRoot, '../internal/server/webapp'))
  assert.equal(config.server.proxy['/api'].ws, true)
  assert.equal(config.server.proxy['/api'].changeOrigin, true)
})

test('Vite development proxy forwards HTTP and WebSocket upgrades', async (t) => {
  let httpPath = ''
  let upgradePath = ''
  const backend = createHTTPServer((request, response) => {
    httpPath = request.url ?? ''
    response.end('dipole-proxy-ok')
  })
  backend.on('upgrade', (request, socket) => {
    upgradePath = request.url ?? ''
    const key = request.headers['sec-websocket-key'] ?? ''
    const accept = createHash('sha1')
      .update(`${key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`)
      .digest('base64')
    socket.end([
      'HTTP/1.1 101 Switching Protocols',
      'Connection: Upgrade',
      'Upgrade: websocket',
      `Sec-WebSocket-Accept: ${accept}`,
      '',
      '',
    ].join('\r\n'))
  })
  backend.listen(0, '127.0.0.1')
  await once(backend, 'listening')
  t.after(() => backend.close())
  const backendPort = backend.address().port

  process.env.DIPOLE_WEB_PROXY_TARGET = `http://127.0.0.1:${backendPort}`
  const vite = await createServer({
    configFile,
    logLevel: 'silent',
    server: { host: '127.0.0.1', port: 0 },
  })
  await vite.listen()
  t.after(async () => {
    delete process.env.DIPOLE_WEB_PROXY_TARGET
    await vite.close()
  })
  const vitePort = vite.httpServer.address().port

  const response = await fetch(`http://127.0.0.1:${vitePort}/api/toolchain-probe`)
  assert.equal(await response.text(), 'dipole-proxy-ok')
  assert.equal(httpPath, '/api/toolchain-probe')

  const statusLine = await websocketHandshake(vitePort, '/api/ws-probe')
  assert.equal(statusLine, 'HTTP/1.1 101 Switching Protocols')
  assert.equal(upgradePath, '/api/ws-probe')
})

test('Vite production build emits assets beneath /app/', async (t) => {
  const output = await mkdtemp(resolve(tmpdir(), 'dipole-vite8-'))
  t.after(() => rm(output, { recursive: true, force: true }))
  await build({ configFile, logLevel: 'silent', build: { outDir: output } })
  const index = await readFile(resolve(output, 'index.html'), 'utf8')
  assert.match(index, /(?:src|href)="\/app\/assets\//)
  assert.doesNotMatch(index, /(?:src|href)="\/assets\//)
})

function websocketHandshake(port, path) {
  return new Promise((resolveHandshake, reject) => {
    const socket = connect(port, '127.0.0.1')
    let response = ''
    socket.setTimeout(5_000)
    socket.on('connect', () => {
      socket.write([
        `GET ${path} HTTP/1.1`,
        `Host: 127.0.0.1:${port}`,
        'Connection: Upgrade',
        'Upgrade: websocket',
        'Sec-WebSocket-Version: 13',
        'Sec-WebSocket-Key: ZGlwb2xlLXRvb2xjaGFpbg==',
        '',
        '',
      ].join('\r\n'))
    })
    socket.on('data', (chunk) => {
      response += chunk.toString('utf8')
      if (response.includes('\r\n\r\n')) {
        socket.destroy()
        resolveHandshake(response.split('\r\n', 1)[0])
      }
    })
    socket.on('timeout', () => reject(new Error('WebSocket proxy handshake timed out')))
    socket.on('error', reject)
  })
}
