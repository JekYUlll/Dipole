import { ref, onUnmounted } from 'vue'
import type { WsPacket } from '@/types'
import { getDeviceID } from '@/device'
import { claimBrowserDelivery } from '@/sync/browserSync'

interface UseWsOptions {
  onMessage?: (packet: WsPacket) => void | Promise<void>
  onConnected?: () => void
  onDisconnected?: () => void
}

type DeliveryClaim = (userUUID: string, deliveryID: string) => Promise<boolean>

const deliveryReplayCapacity = 4096

export class DeliveryDeduplicator {
  private readonly seen = new Set<string>()
  private readonly order: string[] = []

  constructor(private readonly capacity: number) {
    if (!Number.isSafeInteger(capacity) || capacity < 1) throw new Error('delivery replay capacity must be positive')
  }

  accept(packet: WsPacket): boolean {
    const deliveryID = packet.delivery_id?.trim()
    if (!deliveryID) return true
    if (this.seen.has(deliveryID)) return false
    this.seen.add(deliveryID)
    this.order.push(deliveryID)
    if (this.order.length > this.capacity) {
      this.seen.delete(this.order.shift()!)
    }
    return true
  }
}

export async function acceptDeliveryPacket(
  deduplicator: DeliveryDeduplicator,
  packet: WsPacket,
  userUUID: string,
  claim: DeliveryClaim = claimBrowserDelivery,
) {
  if (!deduplicator.accept(packet)) return false
  const deliveryID = packet.delivery_id?.trim()
  if (!deliveryID || !userUUID) return true
  try {
    return await claim(userUUID, deliveryID)
  } catch {
    // Keep realtime delivery available while in-memory dedup remains active.
    return true
  }
}

export function useWebSocket(options: UseWsOptions = {}) {
  const isConnected = ref(false)
  let ws: WebSocket | null = null
  let token = ''
  let userUUID = ''
  let reconnectAttempts = 0
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let manualClose = false
  const deliveryDeduplicator = new DeliveryDeduplicator(deliveryReplayCapacity)
  let inboundQueue = Promise.resolve()

  const connect = (authToken: string, authenticatedUserUUID: string) => {
    if (ws && ws.readyState === WebSocket.OPEN) return
    token = authToken
    userUUID = authenticatedUserUUID
    manualClose = false
    _open()
  }

  const _open = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const deviceID = encodeURIComponent(getDeviceID())
    const url = `${protocol}//${window.location.host}/api/v1/ws?token=${token}&device=web&device_id=${deviceID}`
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
        inboundQueue = inboundQueue.then(async () => {
          if (!await acceptDeliveryPacket(deliveryDeduplicator, packet, userUUID)) return
          await options.onMessage?.(packet)
        }).catch(() => {
          // A failed handler cannot stop later WebSocket frames.
        })
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
