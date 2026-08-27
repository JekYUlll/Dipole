import { z } from "zod";
import { createPool, type Pool } from "mysql2/promise";

import { InMemoryEventLedger, type EventLedger } from "../events/event-ledger.js";
import { KafkaFailureRouter, PermanentKafkaEventError } from "../events/kafka-failure-router.js";
import { KafkaJSConsumerFactory, KafkaShadowConsumer, type KafkaConsumerFactoryPort } from "../events/kafka-shadow-consumer.js";
import { decodeMessageCreatedEvent } from "../events/message-event.js";
import { MySQLEventLedger } from "../events/mysql-event-ledger.js";
import { PROBE_AGENT_EVENT_LEDGER } from "../events/mysql-event-ledger-queries.js";
import { ShadowEventProcessor, type ShadowAuditRecord, type ShadowAuditSink, type ShadowPlanner } from "../events/shadow-processor.js";
import { AISDKStructuredModelClient } from "../models/ai-sdk-model-client.js";
import { ModelRouter } from "../models/model-router.js";
import { ModelShadowPlanner } from "../models/model-shadow-planner.js";
import { MySQLModelAuditStore } from "../models/mysql-model-audit-store.js";
import { PROBE_AGENT_MODEL_RUNS } from "../models/mysql-model-audit-queries.js";

const shadowRuntimeConfigSchema = z.object({
  enabled: z.boolean(),
  brokers: z.array(z.string().trim().min(1)),
  clientId: z.string().trim().min(1),
  groupId: z.string().trim().min(1),
  topic: z.string().trim().min(1),
  topicPrefix: z.string().trim(),
  failureMaxAttempts: z.number().int().min(1).max(100),
  topicPartitions: z.number().int().min(1),
  topicReplicationFactor: z.number().int().min(1),
  tenantId: z.string().trim().min(1),
  agentUuid: z.string().trim().min(1),
  ledgerMode: z.enum(["memory", "mysql"]),
  leaseMs: z.number().int().min(1000).max(86_400_000),
  modelMode: z.enum(["metadata", "ai_sdk"]),
  modelRoutes: z.array(z.string().trim().min(1)),
  modelBudget: z.object({
    maxCalls: z.number().int().min(1).max(10),
    totalTimeoutMs: z.number().int().min(100).max(300_000),
    maxOutputTokensPerCall: z.number().int().min(1).max(32_768)
  }).strict(),
  mysql: z.object({
    host: z.string().trim(),
    port: z.number().int().min(1).max(65_535),
    user: z.string().trim(),
    password: z.string(),
    database: z.string().trim()
  }).strict()
}).strict().superRefine((config, refinement) => {
  if (config.enabled && config.brokers.length === 0) {
    refinement.addIssue({ code: "custom", message: "Kafka brokers are required when shadow runtime is enabled", path: ["brokers"] });
  }
  if (config.enabled && config.ledgerMode === "mysql" && (!config.mysql.host || !config.mysql.user || !config.mysql.password || !config.mysql.database)) {
    refinement.addIssue({ code: "custom", message: "Agent MySQL configuration is required for persistent ledger mode", path: ["mysql"] });
  }
  if (config.modelMode === "ai_sdk" && config.modelRoutes.length === 0) {
    refinement.addIssue({ code: "custom", message: "Agent model routes are required in AI SDK mode", path: ["modelRoutes"] });
  }
  if (config.modelMode === "ai_sdk" && config.ledgerMode !== "mysql") {
    refinement.addIssue({ code: "custom", message: "AI SDK mode requires the persistent MySQL model audit Store", path: ["ledgerMode"] });
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
    topicPrefix: env.DIPOLE_AGENT_KAFKA_TOPIC_PREFIX ?? "dipole",
    failureMaxAttempts: Number.parseInt(env.DIPOLE_AGENT_KAFKA_FAILURE_MAX_ATTEMPTS ?? "3", 10),
    topicPartitions: Number.parseInt(env.DIPOLE_AGENT_KAFKA_TOPIC_PARTITIONS ?? "6", 10),
    topicReplicationFactor: Number.parseInt(env.DIPOLE_AGENT_KAFKA_TOPIC_REPLICATION_FACTOR ?? "1", 10),
    tenantId: env.DIPOLE_AGENT_TENANT_ID ?? "dipole",
    agentUuid: env.DIPOLE_AGENT_UUID ?? "UAI000000000000000001",
    ledgerMode: env.DIPOLE_AGENT_LEDGER_MODE?.trim().toLowerCase() || "memory",
    leaseMs: Number.parseInt(env.DIPOLE_AGENT_LEDGER_LEASE_MS ?? "60000", 10),
    modelMode: env.DIPOLE_AGENT_MODEL_MODE?.trim().toLowerCase() || "metadata",
    modelRoutes: (env.DIPOLE_AGENT_MODEL_ROUTES ?? "").split(",").map((route) => route.trim()).filter(Boolean),
    modelBudget: {
      maxCalls: Number.parseInt(env.DIPOLE_AGENT_MODEL_MAX_CALLS ?? "2", 10),
      totalTimeoutMs: Number.parseInt(env.DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS ?? "15000", 10),
      maxOutputTokensPerCall: Number.parseInt(env.DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS ?? "512", 10)
    },
    mysql: {
      host: env.DIPOLE_AGENT_MYSQL_HOST ?? "",
      port: Number.parseInt(env.DIPOLE_AGENT_MYSQL_PORT ?? "3306", 10),
      user: env.DIPOLE_AGENT_MYSQL_USER ?? "",
      password: env.DIPOLE_AGENT_MYSQL_PASSWORD ?? "",
      database: env.DIPOLE_AGENT_MYSQL_DATABASE ?? ""
    }
  });
}

export interface ShadowRuntime {
  start(): Promise<void>;
  stop(): Promise<void>;
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
  ledger: EventLedger = new InMemoryEventLedger(),
  failureRouter?: KafkaFailureRouter
): KafkaShadowConsumer {
  const processor = new ShadowEventProcessor(planner, audit, ledger);
  return new KafkaShadowConsumer(factory, { groupId: config.groupId, topic: physicalTopic(config) }, async (raw) => {
    let decoded;
    try {
      decoded = decodeMessageCreatedEvent(raw);
    } catch (error) {
      throw new PermanentKafkaEventError(error);
    }
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
  }, failureRouter);
}

export function createKafkaShadowRuntime(config: ShadowRuntimeConfig): ShadowRuntime {
  let pool: Pool | undefined;
  let ledger: EventLedger;
  if (config.ledgerMode === "mysql") {
    pool = createPool({
      host: config.mysql.host, port: config.mysql.port, user: config.mysql.user, password: config.mysql.password,
      database: config.mysql.database, timezone: "Z", connectionLimit: 10
    });
    ledger = new MySQLEventLedger(pool, config.leaseMs);
  } else {
    ledger = new InMemoryEventLedger();
  }
  const factory = new KafkaJSConsumerFactory(config.clientId, config.brokers);
  const failurePublisher = factory.createFailurePublisher();
  const failureRouter = new KafkaFailureRouter(failurePublisher, config.failureMaxAttempts);
  const planner = config.modelMode === "ai_sdk"
    ? new ModelShadowPlanner(new ModelRouter(
      new AISDKStructuredModelClient(), config.modelRoutes, config.modelBudget, undefined, new MySQLModelAuditStore(pool!)
    ), ["conversation.read"])
    : new MetadataShadowPlanner();
  const consumer = buildKafkaShadowRuntime(config, factory, planner, undefined, ledger, failureRouter);
  const mainTopic = physicalTopic(config);
  return {
    start: async () => {
      if (pool !== undefined) {
        await pool.query(PROBE_AGENT_EVENT_LEDGER);
        if (config.modelMode === "ai_sdk") {
          await pool.query(PROBE_AGENT_MODEL_RUNS);
        }
      }
      await factory.ensureTopics(
        [mainTopic, `${mainTopic}.retry`, `${mainTopic}.dead`],
        config.topicPartitions,
        config.topicReplicationFactor
      );
      await failurePublisher.connect();
      await consumer.start();
    },
    stop: async () => {
      try {
        await consumer.stop();
      } finally {
        try {
          await failurePublisher.disconnect();
        } finally {
          if (pool !== undefined) {
            await pool.end();
          }
        }
      }
    }
  };
}

function physicalTopic(config: ShadowRuntimeConfig): string {
  return config.topicPrefix ? `${config.topicPrefix}.${config.topic}` : config.topic;
}
