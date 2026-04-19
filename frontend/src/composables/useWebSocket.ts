import { ref, onUnmounted } from 'vue'
import type { WsPacket } from '@/types'

interface UseWsOptions {
  onMessage?: (packet: WsPacket) => void
  onConnected?: () => void
  onDisconnected?: () => void
}

export function useWebSocket(options: UseWsOptions = {}) {
  const isConnected = ref(false)
  let ws: WebSocket | null = null
  let token = ''
  let reconnectAttempts = 0
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let manualClose = false

  const connect = (authToken: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) return
    token = authToken
    manualClose = false
    _open()
  }

  const _open = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/api/v1/ws?token=${token}&device=web`
    ws = new WebSocket(url)

    ws.onopen = () => {
      isConnected.value = true
      reconnectAttempts = 0
      _startHeartbeat()
      options.onConnected?.()
    }

    ws.onmessage = (event) => {
      try {
        const packet: WsPacket = JSON.parse(event.data as string)
        options.onMessage?.(packet)
      } catch {
        // ignore malformed frames
      }
    }

    ws.onclose = () => {
      isConnected.value = false
      _stopHeartbeat()
      ws = null
      options.onDisconnected?.()
      if (!manualClose) {
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000)
        reconnectAttempts++
        reconnectTimer = setTimeout(_open, delay)
      }
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  const send = (type: string, data: Record<string, unknown>) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type, data }))
    }
  }

  const close = () => {
    manualClose = true
    _stopHeartbeat()
    ws?.close()
    ws = null
    isConnected.value = false
  }

  const _startHeartbeat = () => {
    heartbeatTimer = setInterval(() => send('ping', {}), 30000)
  }

  const _stopHeartbeat = () => {
    if (heartbeatTimer) { clearInterval(heartbeatTimer); heartbeatTimer = null }
    if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null }
  }

  onUnmounted(close)

  return { connect, send, close, isConnected }
}
