import { z } from "zod";
import { readFileSync } from "node:fs";
import * as grpc from "@grpc/grpc-js";
import { createPool, type Pool } from "mysql2/promise";

import { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { ConversationReadCapability } from "../capabilities/conversation-read.js";
import { ConversationSearchCapability } from "../capabilities/conversation-search.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { DeterministicContextCompiler } from "../context/context-compiler.js";
import { createConservativeRouteEstimator, parseRouteContextProfiles, routeContextProfileSchema } from "../context/token-estimator.js";
import { InMemoryEventLedger, type EventLedger } from "../events/event-ledger.js";
import { matchEventSubscriptions, type AgentEventSubscription } from "../events/event-subscription.js";
import { KafkaFailureRouter, PermanentKafkaEventError } from "../events/kafka-failure-router.js";
import { KafkaJSConsumerFactory, KafkaShadowConsumer, type KafkaConsumerFactoryPort } from "../events/kafka-shadow-consumer.js";
import { decodeMessageCreatedEvent } from "../events/message-event.js";
import { MySQLEventLedger } from "../events/mysql-event-ledger.js";
import { PROBE_AGENT_EVENT_LEDGER } from "../events/mysql-event-ledger-queries.js";
import { MySQLShadowAuditSink } from "../events/mysql-shadow-audit-sink.js";
import { PROBE_AGENT_SHADOW_PLANS } from "../events/mysql-shadow-audit-queries.js";
import {
  ShadowEventProcessor,
  type AgentEvent,
  type AgentIdentity,
  type ShadowAuditRecord,
  type ShadowAuditSink,
  type ShadowPlanner,
  type ShadowRunAdmission,
  type ShadowTaskDispatcher
} from "../events/shadow-processor.js";
import { AISDKStructuredModelClient } from "../models/ai-sdk-model-client.js";
import {
  createOpenAICompatibleModelResolver,
  modelProviderCallOptions,
  loadModelProviderConfig,
  modelIDForRoute,
  modelProviderConfigSchema
} from "../models/openai-compatible-model-provider.js";
import { ModelRouter } from "../models/model-router.js";
import { ModelShadowPlanner } from "../models/model-shadow-planner.js";
import { MySQLModelAuditStore } from "../models/mysql-model-audit-store.js";
import type { SubscriptionShadowObserver } from "../observability/subscription-shadow-metrics.js";
import type { SubscriptionRuntimeResult } from "../evals/subscription-runtime-gate.js";
import { PROBE_AGENT_MODEL_RUNS } from "../models/mysql-model-audit-queries.js";
import { AgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import { createTemporalReadStepActivities } from "../temporal/agent-task-read-activities.js";
import type { AgentTaskActivities } from "../temporal/agent-task-activities.js";
import { createReconnectingAgentCapabilityTransport } from "./reconnecting-agent-capability-transport.js";

const shadowRuntimeConfigSchema = z.object({
  enabled: z.boolean(),
  runtimeMode: z.enum(["shadow", "active"]),
  candidateVersion: z.string().trim(),
  releaseManifestPath: z.string().trim(),
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
  triggerMode: z.enum(["direct_target", "subscription"]),
  subscriptionShadowEnabled: z.boolean(),
  ledgerMode: z.enum(["memory", "mysql"]),
  leaseMs: z.number().int().min(1000).max(86_400_000),
  modelMode: z.enum(["metadata", "ai_sdk"]),
  modelProvider: modelProviderConfigSchema,
  modelRoutes: z.array(z.string().trim().min(1)),
  contextCompilerVersion: z.enum(["v1", "v2"]),
  memoryEnabled: z.boolean(),
  retrievalEnabled: z.boolean(),
  retrievalContextEnabled: z.boolean(),
  modelContextProfiles: z.array(routeContextProfileSchema),
  modelBudget: z.object({
    maxCalls: z.number().int().min(1).max(10),
    totalTimeoutMs: z.number().int().min(100).max(300_000),
    maxOutputTokensPerCall: z.number().int().min(1).max(32_768)
  }).strict(),
  capabilityRpc: z.object({
    enabled: z.boolean(),
    target: z.string().trim(),
    secret: z.string(),
    timeoutMs: z.number().int().min(100).max(60_000),
    tls: z.object({
      enabled: z.boolean(),
      caFile: z.string().trim(),
      certFile: z.string().trim(),
      keyFile: z.string().trim(),
      serverName: z.string().trim()
    }).strict()
  }).strict(),
  mysql: z.object({
    host: z.string().trim(),
    port: z.number().int().min(1).max(65_535),
    user: z.string().trim(),
    password: z.string(),
    database: z.string().trim()
  }).strict()
}).strict().superRefine((config, refinement) => {
  if (config.runtimeMode === "active" && !config.enabled) {
    refinement.addIssue({ code: "custom", message: "Active Agent Runtime requires Kafka", path: ["enabled"] });
  }
  if (config.runtimeMode === "active" && config.ledgerMode !== "mysql") {
    refinement.addIssue({ code: "custom", message: "Active Agent Runtime requires the persistent MySQL ledger", path: ["ledgerMode"] });
  }
  if (config.runtimeMode === "active" && config.modelMode !== "ai_sdk") {
    refinement.addIssue({ code: "custom", message: "Active Agent Runtime requires AI SDK model mode", path: ["modelMode"] });
  }
  if (config.runtimeMode === "active" && config.candidateVersion.length === 0) {
    refinement.addIssue({ code: "custom", message: "Active Agent Runtime requires a candidate version", path: ["candidateVersion"] });
  }
  if (config.runtimeMode === "active" && config.releaseManifestPath.length === 0) {
    refinement.addIssue({ code: "custom", message: "Active Agent Runtime requires a release manifest", path: ["releaseManifestPath"] });
  }
  if (config.enabled && !config.groupId.startsWith(`dipole-agent-${config.runtimeMode}-`)) {
    refinement.addIssue({
      code: "custom",
      message: `Kafka ${config.runtimeMode} runtime requires an isolated dipole-agent-${config.runtimeMode}-* group`,
      path: ["groupId"]
    });
  }
  if (config.enabled && config.brokers.length === 0) {
    refinement.addIssue({ code: "custom", message: "Kafka brokers are required when shadow runtime is enabled", path: ["brokers"] });
  }
  if (config.enabled && config.ledgerMode === "mysql" && (!config.mysql.host || !config.mysql.user || !config.mysql.password || !config.mysql.database)) {
    refinement.addIssue({ code: "custom", message: "Agent MySQL configuration is required for persistent ledger mode", path: ["mysql"] });
  }
  if (config.modelMode === "ai_sdk" && config.modelRoutes.length === 0) {
    refinement.addIssue({ code: "custom", message: "Agent model routes are required in AI SDK mode", path: ["modelRoutes"] });
  }
  if (config.modelMode === "ai_sdk" && config.modelProvider.kind !== "openai_compatible") {
    refinement.addIssue({ code: "custom", message: "AI SDK model mode requires an OpenAI-compatible model provider", path: ["modelProvider"] });
  }
  if (config.modelMode === "ai_sdk" && config.modelProvider.kind === "openai_compatible") {
    for (const route of config.modelRoutes) {
      try {
        modelIDForRoute(route, config.modelProvider.name);
      } catch (error) {
        refinement.addIssue({
          code: "custom",
          message: error instanceof Error ? error.message : "Model route does not match the configured provider",
          path: ["modelRoutes"]
        });
      }
    }
  }
  if (config.contextCompilerVersion === "v1" && config.modelContextProfiles.length > 0) {
    refinement.addIssue({
      code: "custom", message: "Model route context profiles require Context Compiler v2", path: ["modelContextProfiles"]
    });
  }
  if (config.contextCompilerVersion === "v2" && config.modelRoutes.length > 0) {
    try {
      const estimator = createConservativeRouteEstimator(config.modelRoutes, config.modelContextProfiles);
      const requiredWindow = 4_096 + config.modelBudget.maxOutputTokensPerCall;
      if (config.modelMode === "ai_sdk" && estimator.contextWindowTokens < requiredWindow) {
        refinement.addIssue({
          code: "custom",
          message: `Model route context window ${estimator.contextWindowTokens} is below required budget ${requiredWindow}`,
          path: ["modelContextProfiles"]
        });
      }
    } catch (error) {
      refinement.addIssue({
        code: "custom",
        message: error instanceof Error ? error.message : "Model route context profiles are invalid",
        path: ["modelContextProfiles"]
      });
    }
  }
  if (config.modelMode === "ai_sdk" && config.ledgerMode !== "mysql") {
    refinement.addIssue({ code: "custom", message: "AI SDK mode requires the persistent MySQL model audit Store", path: ["ledgerMode"] });
  }
  if (config.enabled && config.capabilityRpc.enabled && (!config.capabilityRpc.target || !config.capabilityRpc.secret)) {
    refinement.addIssue({ code: "custom", message: "Agent Capability RPC target and shared secret are required", path: ["capabilityRpc"] });
  }
  if (config.enabled && config.triggerMode === "subscription" && !config.capabilityRpc.enabled) {
    refinement.addIssue({
      code: "custom",
      message: "Subscription trigger mode requires Agent Capability RPC",
      path: ["capabilityRpc", "enabled"]
    });
  }
  if (config.subscriptionShadowEnabled && !config.enabled) {
    refinement.addIssue({ code: "custom", message: "Subscription Shadow observation requires Kafka", path: ["subscriptionShadowEnabled"] });
  }
  if (config.subscriptionShadowEnabled && config.triggerMode !== "direct_target") {
    refinement.addIssue({ code: "custom", message: "Subscription Shadow observation requires direct-target primary mode", path: ["triggerMode"] });
  }
  if (config.subscriptionShadowEnabled && !config.capabilityRpc.enabled) {
    refinement.addIssue({ code: "custom", message: "Subscription Shadow observation requires Agent Capability RPC", path: ["capabilityRpc", "enabled"] });
  }
  if (config.modelMode === "ai_sdk" && !config.capabilityRpc.enabled) {
    refinement.addIssue({ code: "custom", message: "AI SDK mode requires Agent Capability RPC", path: ["capabilityRpc", "enabled"] });
  }
  if (config.memoryEnabled && config.modelMode !== "ai_sdk") {
    refinement.addIssue({ code: "custom", message: "Agent Memory requires AI SDK model mode", path: ["memoryEnabled"] });
  }
  if (config.retrievalEnabled && config.modelMode !== "ai_sdk") {
    refinement.addIssue({ code: "custom", message: "Agent retrieval requires AI SDK model mode", path: ["retrievalEnabled"] });
  }
  if (config.retrievalEnabled && !config.capabilityRpc.enabled) {
    refinement.addIssue({ code: "custom", message: "Agent retrieval requires Agent Capability RPC", path: ["capabilityRpc", "enabled"] });
  }
  if (config.retrievalContextEnabled && !config.retrievalEnabled) {
    refinement.addIssue({ code: "custom", message: "Agent retrieval Context requires retrieval to be enabled", path: ["retrievalContextEnabled"] });
  }
  if (config.capabilityRpc.enabled && config.capabilityRpc.tls.enabled &&
      (!config.capabilityRpc.tls.caFile || !config.capabilityRpc.tls.certFile || !config.capabilityRpc.tls.keyFile || !config.capabilityRpc.tls.serverName)) {
    refinement.addIssue({ code: "custom", message: "Agent Capability RPC mTLS files and server name are required", path: ["capabilityRpc", "tls"] });
  }
  if (config.enabled && config.capabilityRpc.enabled && !config.capabilityRpc.tls.enabled && !isLoopbackTarget(config.capabilityRpc.target)) {
    refinement.addIssue({ code: "custom", message: "Plaintext Agent Capability RPC is accepted only on loopback", path: ["capabilityRpc", "target"] });
  }
});

export type ShadowRuntimeConfig = z.infer<typeof shadowRuntimeConfigSchema>;

export function loadShadowRuntimeConfig(env: NodeJS.ProcessEnv): ShadowRuntimeConfig {
  const configuredRuntimeMode = env.DIPOLE_AGENT_RUNTIME_MODE?.trim().toLowerCase();
  if (configuredRuntimeMode !== undefined && configuredRuntimeMode !== "" && configuredRuntimeMode !== "shadow" && configuredRuntimeMode !== "remote") {
    throw new Error("DIPOLE_AGENT_RUNTIME_MODE must be shadow or remote");
  }
  return shadowRuntimeConfigSchema.parse({
    enabled: env.DIPOLE_AGENT_KAFKA_ENABLED?.trim().toLowerCase() === "true",
    runtimeMode: configuredRuntimeMode === "remote" ? "active" : "shadow",
    candidateVersion: env.DIPOLE_AGENT_CANDIDATE_VERSION ?? "",
    releaseManifestPath: env.DIPOLE_AGENT_RELEASE_MANIFEST ?? "",
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
    triggerMode: env.DIPOLE_AGENT_TRIGGER_MODE?.trim().toLowerCase() || "direct_target",
    subscriptionShadowEnabled: env.DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED?.trim().toLowerCase() === "true",
    ledgerMode: env.DIPOLE_AGENT_LEDGER_MODE?.trim().toLowerCase() || "memory",
    leaseMs: Number.parseInt(env.DIPOLE_AGENT_LEDGER_LEASE_MS ?? "60000", 10),
    modelMode: env.DIPOLE_AGENT_MODEL_MODE?.trim().toLowerCase() || "metadata",
    modelProvider: loadModelProviderConfig(env),
    modelRoutes: (env.DIPOLE_AGENT_MODEL_ROUTES ?? "").split(",").map((route) => route.trim()).filter(Boolean),
    contextCompilerVersion: env.DIPOLE_AGENT_CONTEXT_COMPILER_VERSION?.trim().toLowerCase() || "v1",
    memoryEnabled: env.DIPOLE_AGENT_MEMORY_ENABLED?.trim().toLowerCase() === "true",
    retrievalEnabled: env.DIPOLE_AGENT_RETRIEVAL_ENABLED?.trim().toLowerCase() === "true",
    retrievalContextEnabled: env.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED?.trim().toLowerCase() === "true",
    modelContextProfiles: parseRouteContextProfiles(env.DIPOLE_AGENT_MODEL_CONTEXT_PROFILES ?? ""),
    modelBudget: {
      maxCalls: Number.parseInt(env.DIPOLE_AGENT_MODEL_MAX_CALLS ?? "2", 10),
      totalTimeoutMs: Number.parseInt(env.DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS ?? "15000", 10),
      maxOutputTokensPerCall: Number.parseInt(env.DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS ?? "512", 10)
    },
    capabilityRpc: {
      enabled: (env.DIPOLE_AGENT_CAPABILITY_RPC_ENABLED ?? env.DIPOLE_INTERNAL_RPC_ENABLED)?.trim().toLowerCase() === "true",
      target: env.DIPOLE_AGENT_CAPABILITY_RPC_TARGET ?? env.DIPOLE_INTERNAL_RPC_CORE_TARGET ?? "",
      secret: env.DIPOLE_INTERNAL_RPC_SHARED_SECRET ?? "",
      timeoutMs: Number.parseInt(env.DIPOLE_AGENT_CAPABILITY_RPC_TIMEOUT_MS ?? "2000", 10),
      tls: {
        enabled: env.DIPOLE_INTERNAL_RPC_TLS_ENABLED?.trim().toLowerCase() === "true",
        caFile: env.DIPOLE_INTERNAL_RPC_TLS_CA_FILE ?? "",
        certFile: env.DIPOLE_INTERNAL_RPC_TLS_CERT_FILE ?? "",
        keyFile: env.DIPOLE_INTERNAL_RPC_TLS_KEY_FILE ?? "",
        serverName: env.DIPOLE_INTERNAL_RPC_TLS_SERVER_NAME ?? ""
      }
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

export interface ShadowSubscriptionMatcher {
  matchEventSubscriptions(event: AgentEvent, identity: AgentIdentity): Promise<AgentEventSubscription[]>;
}

export interface SubscriptionRuntimeGate {
  evaluate(): SubscriptionRuntimeResult;
}

interface ShadowSubscriptionAdmission extends ShadowRunAdmission, ShadowSubscriptionMatcher {}

export interface TemporalReadActivityResources {
  readonly activities: AgentTaskActivities;
  readonly client: AgentCapabilityRPCClient;
  start(): Promise<void>;
  stop(): Promise<void>;
}

export class MetadataShadowPlanner implements ShadowPlanner {
  async plan(event: Parameters<ShadowPlanner["plan"]>[0]): ReturnType<ShadowPlanner["plan"]> {
    return {
      summary: `observe ${event.eventType} for ${event.aggregateId}`,
      steps: []
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
  failureRouter?: KafkaFailureRouter,
  admission?: ShadowSubscriptionAdmission,
  registry?: CapabilityRegistry,
  trajectory?: MySQLShadowAuditSink,
  dispatcher?: ShadowTaskDispatcher,
  subscriptionMatcher?: ShadowSubscriptionMatcher,
  subscriptionShadowObserver?: SubscriptionShadowObserver,
  subscriptionRuntimeGate?: SubscriptionRuntimeGate,
  readPermissions?: readonly string[]
): KafkaShadowConsumer {
  const processor = new ShadowEventProcessor(
    planner, audit, ledger, admission, registry, trajectory, config.leaseMs, dispatcher, undefined, readPermissions
  );
  return new KafkaShadowConsumer(factory, {
    groupId: config.groupId,
    topic: physicalTopic(config),
    runtimeMode: config.runtimeMode
  }, async (raw) => {
    let decoded;
    try {
      decoded = decodeMessageCreatedEvent(raw);
    } catch (error) {
      throw new PermanentKafkaEventError(error);
    }
    const directTargetAccepted = decoded.targetUuid === config.agentUuid;
    const identity: AgentIdentity = {
      tenantId: config.tenantId,
      principalUuid: decoded.principalUuid,
      agentUuid: config.agentUuid,
      ...(decoded.requestId === undefined ? {} : { requestId: decoded.requestId }),
      ...(decoded.traceId === undefined ? {} : { traceId: decoded.traceId })
    };
    let event = decoded.event;
    if (config.subscriptionShadowEnabled) {
      if (subscriptionMatcher === undefined || subscriptionShadowObserver === undefined) {
        throw new Error("Subscription Shadow observation dependencies are unavailable");
      }
      let candidateCount = 0;
      try {
        const candidates = await subscriptionMatcher.matchEventSubscriptions(event, identity);
        candidateCount = candidates.length;
        const matches = matchEventSubscriptions(event, candidates);
        subscriptionShadowObserver.observe({
          directTargetAccepted,
          subscriptionOutcome: matches.length === 0 ? "miss" : "match",
          candidateCount
        });
      } catch {
        subscriptionShadowObserver.observe({ directTargetAccepted, subscriptionOutcome: "error", candidateCount });
      }
    }
    if (config.triggerMode === "direct_target" && !directTargetAccepted) return;
    if (config.triggerMode === "subscription") {
      if (subscriptionRuntimeGate !== undefined && !subscriptionRuntimeGate.evaluate().taskCreationAllowed) return;
      const matcher = subscriptionMatcher ?? admission;
      if (matcher === undefined) {
        throw new Error("Subscription trigger mode has no Agent Capability RPC admission client");
      }
      const matches = matchEventSubscriptions(event, await matcher.matchEventSubscriptions(event, identity));
      if (matches.length === 0) return;
      const match = matches[0]!;
      event = {
        ...event,
        subscriptionId: match.subscriptionId,
        subscriptionBinding: {
          subscriptionId: match.subscriptionId,
          definitionId: match.definitionId,
          definitionVersion: match.definitionVersion,
          tenantId: match.tenantId,
          agentId: match.agentId
        }
      };
    }
    await processor.process(event, identity);
  }, failureRouter);
}

export function createKafkaShadowRuntime(
  config: ShadowRuntimeConfig,
  dispatcher?: ShadowTaskDispatcher,
  subscriptionMatcher?: ShadowSubscriptionMatcher,
  subscriptionShadowObserver?: SubscriptionShadowObserver
): ShadowRuntime {
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
  const persistentAudit = pool === undefined ? undefined : new MySQLShadowAuditSink(pool);
  const audit = persistentAudit ?? new ConsoleShadowAuditSink();
  const rpcTransport = config.capabilityRpc.enabled && (
    dispatcher === undefined || (config.triggerMode === "subscription" && subscriptionMatcher === undefined) ||
    (config.subscriptionShadowEnabled && subscriptionMatcher === undefined)
  )
    ? createAgentCapabilityRPC(config)
    : undefined;
  const usesLocalModel = config.modelMode === "ai_sdk" && dispatcher === undefined;
  if (usesLocalModel && persistentAudit === undefined) {
    throw new Error("AI SDK mode requires persistent pre-model Memory lineage");
  }
  let registry: CapabilityRegistry | undefined;
  let trajectory: MySQLShadowAuditSink | undefined;
  if (usesLocalModel) {
    registry = new CapabilityRegistry();
    registry.register(new ConversationListCapability(rpcTransport!.client));
    registry.register(new ConversationReadCapability(rpcTransport!.client));
    if (config.retrievalEnabled) registry.register(new ConversationSearchCapability(rpcTransport!.client));
    trajectory = persistentAudit!;
  }
  const modelCapabilityIds = singlePassModelCapabilityIDs(config);
  const planner = usesLocalModel
    ? new ModelShadowPlanner(new ModelRouter(
      createAISDKModelClient(config), config.modelRoutes, config.modelBudget, undefined, new MySQLModelAuditStore(pool!), undefined, rpcTransport?.client
  ), modelCapabilityIds, routeContextCompiler(config), config.memoryEnabled ? rpcTransport!.client : undefined, undefined, persistentAudit!, rpcTransport!.client, registry!.descriptors(), config.retrievalContextEnabled ? rpcTransport!.client : undefined)
    : new MetadataShadowPlanner();
  const consumer = buildKafkaShadowRuntime(
    config, factory, planner, audit, ledger, failureRouter, rpcTransport?.client, registry, trajectory,
    dispatcher, subscriptionMatcher ?? (config.subscriptionShadowEnabled ? rpcTransport?.client : undefined), subscriptionShadowObserver,
    undefined, readCapabilityPermissions(config)
  );
  const mainTopic = physicalTopic(config);
  return {
    start: async () => {
      if (pool !== undefined) {
        await pool.query(PROBE_AGENT_EVENT_LEDGER);
        await pool.query(PROBE_AGENT_SHADOW_PLANS);
        if (config.modelMode === "ai_sdk" && dispatcher === undefined) {
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
          try {
            if (pool !== undefined) {
              await pool.end();
            }
          } finally {
            rpcTransport?.close();
          }
        }
      }
    }
  };
}

export function createTemporalReadActivityResources(config: ShadowRuntimeConfig): TemporalReadActivityResources {
  if (config.ledgerMode !== "mysql" || config.modelMode !== "ai_sdk" || !config.capabilityRpc.enabled) {
    throw new Error("Temporal read shadow requires MySQL ledger, AI SDK model, and Agent Capability RPC");
  }
  const rpc = createAgentCapabilityRPC(config);
  const pool = createPool({
    host: config.mysql.host, port: config.mysql.port, user: config.mysql.user, password: config.mysql.password,
    database: config.mysql.database, timezone: "Z", connectionLimit: 10
  });
  const audit = new MySQLShadowAuditSink(pool);
  const registry = new CapabilityRegistry();
  registry.register(new ConversationListCapability(rpc.client));
  registry.register(new ConversationReadCapability(rpc.client));
  if (config.retrievalEnabled) registry.register(new ConversationSearchCapability(rpc.client));
  const modelCapabilityIds = singlePassModelCapabilityIDs(config);
  const planner = new ModelShadowPlanner(new ModelRouter(
    createAISDKModelClient(config), config.modelRoutes, config.modelBudget, undefined, new MySQLModelAuditStore(pool), undefined, rpc.client
  ), modelCapabilityIds, routeContextCompiler(config), config.memoryEnabled ? rpc.client : undefined, undefined, audit, rpc.client, registry.descriptors(), config.retrievalContextEnabled ? rpc.client : undefined);
  const temporalStepLeaseMs = Math.min(config.leaseMs, 85_000);
  return {
    activities: createTemporalReadStepActivities({
      planner, audit, registry, trajectory: audit, stepLeaseMs: temporalStepLeaseMs,
      runtimeMode: config.runtimeMode,
      busyStepRetry: { intervalMs: 1000, maxWaitMs: temporalStepLeaseMs + 5000 },
      ...(config.runtimeMode === "shadow" ? { artifacts: rpc.client } : {}),
      ...(config.runtimeMode === "active" ? { contextResolver: rpc.client } : {}),
      readPermissions: readCapabilityPermissions(config)
    }),
    client: rpc.client,
    start: async () => {
      await pool.query(PROBE_AGENT_SHADOW_PLANS);
      await pool.query(PROBE_AGENT_MODEL_RUNS);
    },
    stop: async () => {
      try {
        await pool.end();
      } finally {
        rpc.close();
      }
    }
  };
}

function createAISDKModelClient(config: ShadowRuntimeConfig): AISDKStructuredModelClient {
  return new AISDKStructuredModelClient(
    createOpenAICompatibleModelResolver(config.modelProvider),
    config.modelProvider.outputMode,
    modelProviderCallOptions(config.modelProvider)
  );
}

export function createAgentCapabilityRPC(config: ShadowRuntimeConfig): { client: AgentCapabilityRPCClient; close(): void } {
  const tls = config.capabilityRpc.tls;
  const credentials = tls.enabled
    ? grpc.credentials.createSsl(readFileSync(tls.caFile), readFileSync(tls.keyFile), readFileSync(tls.certFile))
    : grpc.credentials.createInsecure();
  const options: grpc.ClientOptions = tls.enabled ? {
    "grpc.ssl_target_name_override": tls.serverName,
    "grpc.default_authority": tls.serverName
  } : {};
  const transport = createReconnectingAgentCapabilityTransport(
    () => new AgentCapabilityServiceClient(config.capabilityRpc.target, credentials, options)
  );
  return {
    client: new AgentCapabilityRPCClient(transport.client, config.capabilityRpc.secret, config.capabilityRpc.timeoutMs, config.runtimeMode, config.candidateVersion),
    close: () => transport.close()
  };
}

function isLoopbackTarget(target: string): boolean {
  const endpoint = target.trim().replace(/^dns:\/\//, "").toLowerCase();
  if (endpoint === "::1") return true;
  const host = endpoint.startsWith("[")
    ? endpoint.slice(1, endpoint.indexOf("]"))
    : endpoint.slice(0, endpoint.lastIndexOf(":"));
  return host === "127.0.0.1" || host === "localhost" || host === "::1";
}

// A one-shot plan cannot bind a follow-up read to a trusted discovery result.
export function singlePassModelCapabilityIDs(_config: ShadowRuntimeConfig): readonly string[] {
  return ["conversation.list"];
}

function readCapabilityPermissions(config: ShadowRuntimeConfig): readonly string[] {
  return config.retrievalEnabled
    ? ["conversation.list", "conversation.read", "conversation.search"]
    : ["conversation.list", "conversation.read"];
}

function physicalTopic(config: ShadowRuntimeConfig): string {
  return config.topicPrefix ? `${config.topicPrefix}.${config.topic}` : config.topic;
}

function routeContextCompiler(config: ShadowRuntimeConfig): DeterministicContextCompiler {
  if (config.contextCompilerVersion === "v1") return new DeterministicContextCompiler();
  const estimator = createConservativeRouteEstimator(config.modelRoutes, config.modelContextProfiles);
  const requiredWindow = 4_096 + config.modelBudget.maxOutputTokensPerCall;
  if (estimator.contextWindowTokens < requiredWindow) {
    throw new Error(`Model route context window ${estimator.contextWindowTokens} is below required budget ${requiredWindow}`);
  }
  return new DeterministicContextCompiler(estimator.estimate, {
    compilerVersion: "v2",
    estimatorId: estimator.id,
    maxInputTokens: estimator.contextWindowTokens - config.modelBudget.maxOutputTokensPerCall
  });
}
