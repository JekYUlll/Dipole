import { z } from "zod";

import type { AgentEvent } from "./shadow-processor.js";

const identifier = (maximum: number) => z.string().trim().min(1).max(maximum);
const termsFilterSchema = z.object({
  terms: z.array(z.string().trim().min(1).max(64).refine((term) => !/[\u0000-\u001f\u007f]/u.test(term), "terms contain control characters")).min(1).max(32)
}).strict();

const subscriptionSchema = z.object({
  subscriptionId: identifier(64),
  definitionId: identifier(64),
  definitionVersion: z.number().int().positive(),
  tenantId: identifier(64),
  agentId: identifier(24),
  eventType: identifier(64),
  resourceType: z.literal("conversation"),
  resourceId: identifier(128),
  filterKind: z.enum(["all", "message_contains_any"]),
  filter: z.unknown()
}).strict().superRefine((subscription, context) => {
  const result = subscription.filterKind === "all"
    ? z.object({}).strict().safeParse(subscription.filter)
    : termsFilterSchema.safeParse(subscription.filter);
  if (!result.success) {
    context.addIssue({ code: "custom", path: ["filter"], message: result.error.message });
  }
});

export type AgentEventSubscription = z.infer<typeof subscriptionSchema>;

export function parseAgentEventSubscription(value: unknown): AgentEventSubscription {
  return subscriptionSchema.parse(value);
}

export function matchEventSubscriptions(event: AgentEvent, rawSubscriptions: readonly unknown[]): AgentEventSubscription[] {
  const subscriptions = rawSubscriptions.map((item) => subscriptionSchema.parse(item));
  const resourceId = typeof event.payload.conversation_key === "string" ? event.payload.conversation_key.trim() : "";
  const content = typeof event.payload.content === "string" ? event.payload.content.toLowerCase() : "";
  return subscriptions
    .filter((subscription) => subscription.eventType === event.eventType)
    .filter((subscription) => subscription.resourceId === "*" || subscription.resourceId === resourceId)
    .filter((subscription) => {
      if (subscription.filterKind === "all") return true;
      const filter = termsFilterSchema.parse(subscription.filter);
      return filter.terms.some((term) => content.includes(term.toLowerCase()));
    })
    .sort((left, right) => left.subscriptionId.localeCompare(right.subscriptionId));
}
