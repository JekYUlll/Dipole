import type { Message } from '@/types'

export interface TimelineNotification {
  schema_version: string
  event_id: string
  message_uuid: string
  conversation_key: string
  message_seq: number
  target_type: number
  target_uuid: string
}

export type TimelineNotifyShadowOutcome = 'match' | 'missing' | 'mismatch' | 'error' | 'invalid'

export interface TimelineNotifyShadowTransport {
  list(notification: TimelineNotification, afterSeq: number, limit: number): Promise<Message[]>
}

type OutcomeReporter = (outcome: TimelineNotifyShadowOutcome) => void | Promise<void>

export class TimelineNotifyShadowVerifier {
  private readonly pageSize: number
  private readonly maxPages: number
  private readonly maxSeenEvents: number
  private readonly verifiedSequences = new Map<string, number>()
  private readonly chains = new Map<string, Promise<void>>()
  private readonly seenEvents = new Set<string>()
  private readonly seenEventOrder: string[] = []

  constructor(
    private readonly transport: TimelineNotifyShadowTransport,
    private readonly report: OutcomeReporter,
    options: { pageSize?: number; maxPages?: number; maxSeenEvents?: number } = {},
  ) {
    this.pageSize = boundedInteger(options.pageSize, 100, 1, 200)
    this.maxPages = boundedInteger(options.maxPages, 100, 1, 1_000)
    this.maxSeenEvents = boundedInteger(options.maxSeenEvents, 2_048, 16, 10_000)
  }

  observe(raw: unknown): Promise<void> {
    const notification = normalizeNotification(raw)
    if (!notification) {
      this.reportSafely('invalid')
      return Promise.resolve()
    }
    if (this.seenEvents.has(notification.event_id)) return Promise.resolve()
    this.rememberEvent(notification.event_id)

    const previous = this.chains.get(notification.conversation_key) ?? Promise.resolve()
    const current = previous.catch(() => {}).then(() => this.verify(notification))
    this.chains.set(notification.conversation_key, current)
    return current.finally(() => {
      if (this.chains.get(notification.conversation_key) === current) {
        this.chains.delete(notification.conversation_key)
      }
    })
  }

  private async verify(notification: TimelineNotification) {
    const verified = this.verifiedSequences.get(notification.conversation_key)
    if (verified !== undefined && notification.message_seq <= verified) return

    let cursor = verified ?? notification.message_seq - 1
    try {
      for (let pageNumber = 0; pageNumber < this.maxPages && cursor < notification.message_seq; pageNumber += 1) {
        const page = await this.transport.list(notification, cursor, this.pageSize)
        if (!Array.isArray(page) || page.length === 0) {
          this.reportSafely('missing')
          return
        }

        let previous = cursor
        for (const message of page) {
          const sequence = message?.message_seq
          if (!message?.message_id || !Number.isSafeInteger(sequence) || sequence !== previous + 1) {
            this.reportSafely('missing')
            return
          }
          previous = sequence
          if (sequence === notification.message_seq) {
            if (message.message_id !== notification.message_uuid) {
              this.reportSafely('mismatch')
              return
            }
            this.verifiedSequences.set(notification.conversation_key, notification.message_seq)
            this.reportSafely('match')
            return
          }
          if (sequence > notification.message_seq) {
            this.reportSafely('missing')
            return
          }
        }
        cursor = previous
      }
      this.reportSafely('missing')
    } catch {
      this.reportSafely('error')
    }
  }

  private rememberEvent(eventID: string) {
    this.seenEvents.add(eventID)
    this.seenEventOrder.push(eventID)
    while (this.seenEventOrder.length > this.maxSeenEvents) {
      this.seenEvents.delete(this.seenEventOrder.shift()!)
    }
  }

  private reportSafely(outcome: TimelineNotifyShadowOutcome) {
    try {
      void Promise.resolve(this.report(outcome)).catch(() => {})
    } catch {
      // Shadow telemetry cannot affect realtime message delivery.
    }
  }
}

function normalizeNotification(raw: unknown): TimelineNotification | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const value = raw as Partial<TimelineNotification>
  if (value.schema_version !== 'v1') return undefined
  if (!nonEmpty(value.event_id) || !nonEmpty(value.message_uuid) || !nonEmpty(value.conversation_key) || !nonEmpty(value.target_uuid)) return undefined
  if (!Number.isSafeInteger(value.message_seq) || value.message_seq! <= 0) return undefined
  if (value.target_type !== 0 && value.target_type !== 1) return undefined
  return value as TimelineNotification
}

function nonEmpty(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0
}

function boundedInteger(value: number | undefined, fallback: number, minimum: number, maximum: number) {
  const candidate = Number.isSafeInteger(value) ? value as number : fallback
  return Math.min(maximum, Math.max(minimum, candidate))
}
