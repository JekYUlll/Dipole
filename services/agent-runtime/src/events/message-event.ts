import { z } from "zod";

import type { AgentEvent } from "./shadow-processor.js";

const lineageIdentifier = /^[A-Za-z0-9][A-Za-z0-9._:/-]*$/;

const messagePayloadBaseSchema = z.object({
  mutation_type: z.literal("created"),
  revision: z.number().int().min(1),
  actor_uuid: z.string().trim().min(1),
  message_id: z.string().trim().min(1),
  conversation_key: z.string().trim().min(1),
  message_seq: z.number().int().min(1),
  sender_uuid: z.string().trim().min(1),
  target_uuid: z.string().trim().min(1),
  message_type: z.number().int(),
  content: z.string(),
  sent_at: z.iso.datetime()
}).passthrough();

const messagePayloadSchema = messagePayloadBaseSchema.extend({
  target_type: z.literal(0)
});

const groupMessagePayloadSchema = messagePayloadBaseSchema.extend({
  target_type: z.literal(1)
});

const wireLineageSchema = z.object({
  origin: z.object({
    type: z.enum(["agent", "service", "system"]),
    id: z.string().trim().min(1).max(128).regex(lineageIdentifier)
  }).strict(),
  causation_event_id: z.string().trim().min(1).max(128).regex(lineageIdentifier).optional(),
  agent_task_id: z.string().trim().min(1).max(128).regex(lineageIdentifier).optional()
}).strict().superRefine((lineage, context) => {
  if (lineage.origin.type === "agent" && lineage.agent_task_id === undefined) {
    context.addIssue({ code: "custom", path: ["agent_task_id"], message: "agent_task_id is required for Agent origin" });
  }
});

const messageCreatedEnvelopeSchema = z.object({
  event_id: z.string().trim().min(1),
  request_id: z.string().trim().min(1).optional(),
  trace_id: z.string().trim().min(1).optional(),
  lineage: wireLineageSchema.optional(),
  event_type: z.literal("message.direct.created"),
  version: z.string().regex(/^v1(?:\.[0-9]+)*$/, "unsupported version"),
  source: z.literal("dipole"),
  occurred_at: z.iso.datetime(),
  payload: messagePayloadSchema
}).passthrough();

const groupMessageCreatedEnvelopeSchema = z.object({
  event_id: z.string().trim().min(1),
  request_id: z.string().trim().min(1).optional(),
  trace_id: z.string().trim().min(1).optional(),
  lineage: wireLineageSchema.optional(),
  event_type: z.literal("message.group.created"),
  version: z.string().regex(/^v1(?:\.[0-9]+)*$/, "unsupported version"),
  source: z.literal("dipole"),
  occurred_at: z.iso.datetime(),
  payload: groupMessagePayloadSchema
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
      payload: envelope.payload,
      ...(envelope.lineage === undefined ? {} : {
        lineage: {
          origin: envelope.lineage.origin,
          ...(envelope.lineage.causation_event_id === undefined ? {} : { causationEventId: envelope.lineage.causation_event_id }),
          ...(envelope.lineage.agent_task_id === undefined ? {} : { agentTaskId: envelope.lineage.agent_task_id })
        }
      })
    },
    principalUuid: envelope.payload.sender_uuid,
    targetUuid: envelope.payload.target_uuid,
    ...(envelope.request_id === undefined ? {} : { requestId: envelope.request_id }),
    ...(envelope.trace_id === undefined ? {} : { traceId: envelope.trace_id })
  };
}

// Route B/B2: a group message targets a group (target_type=1) rather than a
// direct user. The decoded event keeps the same shape as a direct message so
// the runtime can route it through the same interactive-trigger pipeline; the
// caller distinguishes the two by event_type / target_type on the payload.
export function decodeGroupMessageCreatedEvent(raw: string): DecodedMessageCreatedEvent {
  const envelope = groupMessageCreatedEnvelopeSchema.parse(JSON.parse(raw) as unknown);
  return {
    event: {
      eventId: envelope.event_id,
      eventType: envelope.event_type,
      aggregateId: envelope.payload.message_id,
      occurredAt: envelope.occurred_at,
      payload: envelope.payload,
      ...(envelope.lineage === undefined ? {} : {
        lineage: {
          origin: envelope.lineage.origin,
          ...(envelope.lineage.causation_event_id === undefined ? {} : { causationEventId: envelope.lineage.causation_event_id }),
          ...(envelope.lineage.agent_task_id === undefined ? {} : { agentTaskId: envelope.lineage.agent_task_id })
        }
      })
    },
    principalUuid: envelope.payload.sender_uuid,
    targetUuid: envelope.payload.target_uuid,
    ...(envelope.request_id === undefined ? {} : { requestId: envelope.request_id }),
    ...(envelope.trace_id === undefined ? {} : { traceId: envelope.trace_id })
  };
}
