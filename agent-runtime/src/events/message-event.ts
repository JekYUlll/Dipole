import { z } from "zod";

import type { AgentEvent } from "./shadow-processor.js";

const messagePayloadSchema = z.object({
  mutation_type: z.literal("created"),
  revision: z.number().int().min(1),
  actor_uuid: z.string().trim().min(1),
  message_id: z.string().trim().min(1),
  conversation_key: z.string().trim().min(1),
  message_seq: z.number().int().min(1),
  sender_uuid: z.string().trim().min(1),
  target_uuid: z.string().trim().min(1),
  target_type: z.literal(0),
  message_type: z.number().int(),
  content: z.string(),
  sent_at: z.iso.datetime()
}).passthrough();

const messageCreatedEnvelopeSchema = z.object({
  event_id: z.string().trim().min(1),
  request_id: z.string().trim().min(1).optional(),
  trace_id: z.string().trim().min(1).optional(),
  event_type: z.literal("message.direct.created"),
  version: z.string().regex(/^v1(?:\.[0-9]+)*$/, "unsupported version"),
  source: z.literal("dipole"),
  occurred_at: z.iso.datetime(),
  payload: messagePayloadSchema
}).passthrough();

export interface DecodedMessageCreatedEvent {
  readonly event: AgentEvent;
  readonly principalUuid: string;
  readonly targetUuid: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export function decodeMessageCreatedEvent(raw: string): DecodedMessageCreatedEvent {
  const envelope = messageCreatedEnvelopeSchema.parse(JSON.parse(raw) as unknown);
  return {
    event: {
      eventId: envelope.event_id,
      eventType: envelope.event_type,
      aggregateId: envelope.payload.message_id,
      occurredAt: envelope.occurred_at,
      payload: envelope.payload
    },
    principalUuid: envelope.payload.sender_uuid,
    targetUuid: envelope.payload.target_uuid,
    ...(envelope.request_id === undefined ? {} : { requestId: envelope.request_id }),
    ...(envelope.trace_id === undefined ? {} : { traceId: envelope.trace_id })
  };
}
