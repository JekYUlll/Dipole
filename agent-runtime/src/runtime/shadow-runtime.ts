import { z } from "zod";

import { InMemoryEventLedger, type EventLedger } from "../events/event-ledger.js";
import { KafkaJSConsumerFactory, KafkaShadowConsumer, type KafkaConsumerFactoryPort } from "../events/kafka-shadow-consumer.js";
import { decodeMessageCreatedEvent } from "../events/message-event.js";
import { ShadowEventProcessor, type ShadowAuditRecord, type ShadowAuditSink, type ShadowPlanner } from "../events/shadow-processor.js";

const shadowRuntimeConfigSchema = z.object({
  enabled: z.boolean(),
  brokers: z.array(z.string().trim().min(1)),
  clientId: z.string().trim().min(1),
  groupId: z.string().trim().min(1),
  topic: z.string().trim().min(1),
  tenantId: z.string().trim().min(1),
  agentUuid: z.string().trim().min(1)
}).strict().superRefine((config, refinement) => {
  if (config.enabled && config.brokers.length === 0) {
    refinement.addIssue({ code: "custom", message: "Kafka brokers are required when shadow runtime is enabled", path: ["brokers"] });
  }
});

export type ShadowRuntimeConfig = z.infer<typeof shadowRuntimeConfigSchema>;

export function loadShadowRuntimeConfig(env: NodeJS.ProcessEnv): ShadowRuntimeConfig {
  return shadowRuntimeConfigSchema.parse({
    enabled: env.DIPOLE_AGENT_KAFKA_ENABLED?.trim().toLowerCase() === "true",
    brokers: (env.DIPOLE_AGENT_KAFKA_BROKERS ?? "").split(",").map((broker) => broker.trim()).filter(Boolean),
    clientId: env.DIPOLE_AGENT_KAFKA_CLIENT_ID ?? "dipole-agent",
    groupId: env.DIPOLE_AGENT_KAFKA_GROUP_ID ?? "dipole-agent-shadow-v1",
    topic: env.DIPOLE_AGENT_KAFKA_TOPIC ?? "message.direct.created",
    tenantId: env.DIPOLE_AGENT_TENANT_ID ?? "dipole",
    agentUuid: env.DIPOLE_AGENT_UUID ?? "UAI000000000000000001"
  });
}

export class MetadataShadowPlanner implements ShadowPlanner {
  async plan(event: Parameters<ShadowPlanner["plan"]>[0]): ReturnType<ShadowPlanner["plan"]> {
    return {
      summary: `observe ${event.eventType} for ${event.aggregateId}`,
      capabilityIds: []
    };
  }
}

export class ConsoleShadowAuditSink implements ShadowAuditSink {
  async append(record: ShadowAuditRecord): Promise<void> {
    process.stdout.write(`${JSON.stringify({ type: "agent.shadow.plan", ...record })}\n`);
  }
}

export function buildKafkaShadowRuntime(
  config: ShadowRuntimeConfig,
  factory: KafkaConsumerFactoryPort,
  planner: ShadowPlanner = new MetadataShadowPlanner(),
  audit: ShadowAuditSink = new ConsoleShadowAuditSink(),
  ledger: EventLedger = new InMemoryEventLedger()
): KafkaShadowConsumer {
  const processor = new ShadowEventProcessor(planner, audit, ledger);
  return new KafkaShadowConsumer(factory, { groupId: config.groupId, topic: config.topic }, async (raw) => {
    const decoded = decodeMessageCreatedEvent(raw);
    if (decoded.targetUuid !== config.agentUuid) {
      return;
    }
    await processor.process(decoded.event, {
      tenantId: config.tenantId,
      principalUuid: decoded.principalUuid,
      agentUuid: config.agentUuid,
      ...(decoded.requestId === undefined ? {} : { requestId: decoded.requestId }),
      ...(decoded.traceId === undefined ? {} : { traceId: decoded.traceId })
    });
  });
}

export function createKafkaShadowRuntime(config: ShadowRuntimeConfig): KafkaShadowConsumer {
  return buildKafkaShadowRuntime(config, new KafkaJSConsumerFactory(config.clientId, config.brokers));
}
