import { describe, expect, it, vi } from "vitest";

import type { KafkaConsumerFactoryPort, KafkaConsumerPort, KafkaInboundPayload } from "../events/kafka-shadow-consumer.js";
import { agentRunId, agentTaskId, type AgentEvent } from "../events/shadow-processor.js";
import type { AgentEventSubscription } from "../events/event-subscription.js";
import type { ExecutionContext } from "./execution-context.js";
import { buildKafkaShadowRuntime, loadShadowRuntimeConfig } from "./shadow-runtime.js";
import { SubscriptionShadowMetrics } from "../observability/subscription-shadow-metrics.js";

describe("shadow runtime composition", () => {
  it("requires brokers only when Kafka shadow mode is enabled", () => {
    expect(loadShadowRuntimeConfig({})).toMatchObject({
      enabled: false, groupId: "dipole-agent-shadow-v1", ledgerMode: "memory", modelMode: "metadata",
      contextCompilerVersion: "v1", memoryEnabled: false, triggerMode: "direct_target", capabilityRpc: { enabled: false }
    });
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_KAFKA_ENABLED: "true" })).toThrow(/brokers/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true",
      DIPOLE_AGENT_KAFKA_BROKERS: "kafka-1:9092, kafka-2:9092"
    }).brokers).toEqual(["kafka-1:9092", "kafka-2:9092"]);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_LEDGER_MODE: "mysql"
    })).toThrow(/MySQL/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_LEDGER_MODE: "mysql",
      DIPOLE_AGENT_MYSQL_HOST: "mysql", DIPOLE_AGENT_MYSQL_USER: "agent", DIPOLE_AGENT_MYSQL_PASSWORD: "secret",
      DIPOLE_AGENT_MYSQL_DATABASE: "dipole"
    })).toMatchObject({ ledgerMode: "mysql", mysql: { host: "mysql", port: 3306, user: "agent", database: "dipole" } });
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_MODEL_MODE: "ai_sdk" })).toThrow(/model routes/);
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_MEMORY_ENABLED: "true" })).toThrow(/Memory.*AI SDK/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_MODE: "ai_sdk", DIPOLE_AGENT_MODEL_ROUTES: "provider/model"
    })).toThrow(/persistent MySQL model audit/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_MODE: "ai_sdk",
      DIPOLE_AGENT_MODEL_ROUTES: "openai/gpt-5-mini,anthropic/claude-sonnet-4.5",
      DIPOLE_AGENT_LEDGER_MODE: "mysql",
      DIPOLE_AGENT_MYSQL_HOST: "mysql", DIPOLE_AGENT_MYSQL_USER: "agent", DIPOLE_AGENT_MYSQL_PASSWORD: "secret",
      DIPOLE_AGENT_MYSQL_DATABASE: "dipole",
      DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
      DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091",
      DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret",
      DIPOLE_AGENT_MODEL_MAX_CALLS: "2",
      DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS: "12000",
      DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS: "256",
      DIPOLE_AGENT_CONTEXT_COMPILER_VERSION: "v2",
      DIPOLE_AGENT_MEMORY_ENABLED: "true",
      DIPOLE_AGENT_MODEL_CONTEXT_PROFILES: '[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]'
    })).toMatchObject({
      modelMode: "ai_sdk",
      memoryEnabled: true,
      contextCompilerVersion: "v2",
      modelRoutes: ["openai/gpt-5-mini", "anthropic/claude-sonnet-4.5"],
      modelBudget: { maxCalls: 2, totalTimeoutMs: 12000, maxOutputTokensPerCall: 256 },
      modelContextProfiles: [{
        route: "openai/gpt-5-mini", contextWindowTokens: 32_768, utf8BytesPerToken: 3, safetyMarginBps: 1_500
      }]
    });
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_CONTEXT_PROFILES: "not-json"
    })).toThrow(/JSON/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_ROUTES: "provider/model",
      DIPOLE_AGENT_MODEL_CONTEXT_PROFILES: '[{"route":"provider/model","contextWindowTokens":8192,"utf8BytesPerToken":3,"safetyMarginBps":1000}]'
    })).toThrow(/require Context Compiler v2/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_ROUTES: "provider/model",
      DIPOLE_AGENT_CONTEXT_COMPILER_VERSION: "v2",
      DIPOLE_AGENT_MODEL_CONTEXT_PROFILES: '[{"route":"other/model","contextWindowTokens":8192,"utf8BytesPerToken":3,"safetyMarginBps":1000}]'
    })).toThrow(/unknown route/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_MODE: "ai_sdk", DIPOLE_AGENT_MODEL_ROUTES: "provider/model",
      DIPOLE_AGENT_CONTEXT_COMPILER_VERSION: "v2",
      DIPOLE_AGENT_LEDGER_MODE: "mysql", DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
      DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS: "5000"
    })).toThrow(/context window/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
      DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true", DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "core:9091",
      DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret"
    })).toThrow(/loopback/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
      DIPOLE_AGENT_TRIGGER_MODE: "subscription"
    })).toThrow(/subscription.*Capability RPC/i);
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED: "true" })).toThrow(/Kafka/i);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
      DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED: "true"
    })).toThrow(/Capability RPC/i);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
      DIPOLE_AGENT_TRIGGER_MODE: "subscription", DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED: "true",
      DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true", DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091",
      DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret"
    })).toThrow(/direct.target/i);
  });

  it("decodes a Kafka envelope and records a read-only plan", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const factory: KafkaConsumerFactoryPort = { create: () => consumer };
    const planner = { plan: vi.fn(async (_event: AgentEvent, _context: ExecutionContext) => ({ summary: "observe", steps: [] })) };
    const audit = { append: vi.fn(async () => undefined) };
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI"
    });
    const runtime = buildKafkaShadowRuntime(config, factory, planner, audit);
    await runtime.start();
    await eachMessage!(payload(messageEnvelope()));

    expect(planner).toHaveProperty("plan");
    expect(planner.plan.mock.calls[0]?.[1]).toMatchObject({
      principalUuid: "U100", agentUuid: "UAI", mode: "shadow", requestId: "R1", traceId: "T1", eventId: "E1"
    });
    expect(audit.append).toHaveBeenCalledWith(expect.objectContaining({ eventId: "E1", eventType: "message.direct.created" }));
  });

  it("ignores direct messages addressed to another Agent", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const planner = { plan: vi.fn(async () => ({ summary: "observe", steps: [] })) };
    const audit = { append: vi.fn(async () => undefined) };
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI"
    });
    const runtime = buildKafkaShadowRuntime(config, { create: () => consumer }, planner, audit);

    await runtime.start();
    await eachMessage!(payload(messageEnvelope("OTHER")));

    expect(planner.plan).not.toHaveBeenCalled();
    expect(audit.append).not.toHaveBeenCalled();
  });

  it("stops before the ledger and planner when no deterministic subscription matches", async () => {
    const fixture = runtimeFixture();
    const subscriptions = subscriptionAdmission([
      subscription("SUB-1", "message_contains_any", { terms: ["incident"] })
    ]);
    const config = subscriptionConfig();
    const runtime = buildKafkaShadowRuntime(
      config, fixture.factory, fixture.planner, fixture.audit, fixture.ledger, undefined, subscriptions
    );

    await runtime.start();
    await fixture.eachMessage()(payload(messageEnvelope("U200")));

    expect(subscriptions.matchEventSubscriptions).toHaveBeenCalledOnce();
    expect(fixture.ledger.claim).not.toHaveBeenCalled();
    expect(fixture.planner.plan).not.toHaveBeenCalled();
    expect(fixture.audit.append).not.toHaveBeenCalled();
  });

  it("stops before the ledger when the enforced subscription rollout is blocked", async () => {
    const fixture = runtimeFixture();
    const subscriptions = subscriptionAdmission([subscription("SUB-1", "all", {})]);
    const gate = { evaluate: vi.fn(() => ({
      mode: "enforced" as const, outcome: "blocked" as const, taskCreationAllowed: false,
      reason: "subscription_rollout_blocked"
    })) };
    const runtime = buildKafkaShadowRuntime(
      subscriptionConfig(), fixture.factory, fixture.planner, fixture.audit, fixture.ledger,
      undefined, subscriptions, undefined, undefined, undefined, undefined, undefined, gate
    );

    await runtime.start();
    await fixture.eachMessage()(payload(messageEnvelope("U200")));

    expect(gate.evaluate).toHaveBeenCalledOnce();
    expect(subscriptions.matchEventSubscriptions).not.toHaveBeenCalled();
    expect(fixture.ledger.claim).not.toHaveBeenCalled();
  });

  it("pins the stable matching subscription before task admission", async () => {
    const fixture = runtimeFixture();
    const subscriptions = subscriptionAdmission([
      subscription("SUB-B", "all", {}),
      subscription("SUB-A", "message_contains_any", { terms: ["hello"] })
    ]);
    const runtime = buildKafkaShadowRuntime(
      subscriptionConfig(), fixture.factory, fixture.planner, fixture.audit, fixture.ledger, undefined, subscriptions
    );

    await runtime.start();
    await fixture.eachMessage()(payload(messageEnvelope("U200")));

    expect(subscriptions.admit).toHaveBeenCalledWith(
      expect.objectContaining({
        eventId: "E1",
        subscriptionId: "SUB-A",
        subscriptionBinding: {
          subscriptionId: "SUB-A",
          definitionId: "DEF-1",
          definitionVersion: 1,
          tenantId: "dipole",
          agentId: "UAI"
        }
      }),
      expect.objectContaining({ agentUuid: "UAI" })
    );
    expect(fixture.planner.plan).toHaveBeenCalledOnce();
  });

  it("uses a borrowed subscription matcher with an independent dispatcher", async () => {
    const fixture = runtimeFixture();
    const matcher = {
      matchEventSubscriptions: vi.fn(async () => [subscription("SUB-A", "all", {})])
    };
    const dispatcher = { dispatch: vi.fn(async () => undefined) };
    const runtime = buildKafkaShadowRuntime(
      subscriptionConfig(), fixture.factory, fixture.planner, fixture.audit, fixture.ledger,
      undefined, undefined, undefined, undefined, dispatcher, matcher
    );

    await runtime.start();
    await fixture.eachMessage()(payload(messageEnvelope("U200")));

    expect(matcher.matchEventSubscriptions).toHaveBeenCalledOnce();
    expect(dispatcher.dispatch).toHaveBeenCalledWith(
      expect.objectContaining({
        subscriptionId: "SUB-A",
        subscriptionBinding: expect.objectContaining({ subscriptionId: "SUB-A" })
      }),
      expect.objectContaining({ agentUuid: "UAI" }),
      expect.any(String)
    );
    expect(fixture.planner.plan).not.toHaveBeenCalled();
  });

  it("observes subscription match, miss, and errors before the ledger without changing direct-target admission", async () => {
    const fixture = runtimeFixture();
    const matcher = { matchEventSubscriptions: vi.fn()
      .mockResolvedValueOnce([subscription("SUB-A", "all", {}), { ...subscription("SUB-X", "all", {}), resourceId: "group:OTHER" }])
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new Error("Core unavailable")) };
    const metrics = new SubscriptionShadowMetrics();
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI",
      DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED: "true", DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
      DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091", DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret"
    });
    const runtime = buildKafkaShadowRuntime(
      config, fixture.factory, fixture.planner, fixture.audit, fixture.ledger,
      undefined, undefined, undefined, undefined, undefined, matcher, metrics
    );
    await runtime.start();

    await fixture.eachMessage()(payload(messageEnvelope("OTHER", "E1")));
    await fixture.eachMessage()(payload(messageEnvelope("OTHER", "E2")));
    await fixture.eachMessage()(payload(messageEnvelope("UAI", "E3")));

    expect(matcher.matchEventSubscriptions).toHaveBeenCalledTimes(3);
    expect(fixture.ledger.claim).toHaveBeenCalledTimes(1);
    expect(fixture.planner.plan).toHaveBeenCalledTimes(1);
    expect(metrics.render()).toContain('direct_target="ignored",subscription="match"} 1');
    expect(metrics.render()).toContain('direct_target="ignored",subscription="miss"} 1');
    expect(metrics.render()).toContain('direct_target="accepted",subscription="error"} 1');
    expect(metrics.render()).toContain("dipole_agent_subscription_shadow_candidates_total 2");
  });
});

function subscriptionConfig() {
  return loadShadowRuntimeConfig({
    DIPOLE_AGENT_KAFKA_ENABLED: "true",
    DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
    DIPOLE_AGENT_UUID: "UAI",
    DIPOLE_AGENT_TRIGGER_MODE: "subscription",
    DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
    DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091",
    DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret"
  });
}

function runtimeFixture() {
  let handler: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
  const consumer: KafkaConsumerPort = {
    connect: async () => undefined,
    subscribe: async () => undefined,
    run: async (config) => { handler = config.eachMessage; },
    disconnect: async () => undefined
  };
  return {
    factory: { create: () => consumer } satisfies KafkaConsumerFactoryPort,
    eachMessage: () => handler!,
    planner: { plan: vi.fn(async () => ({ summary: "observe", steps: [] })) },
    audit: { append: vi.fn(async () => undefined) },
    ledger: {
      claim: vi.fn(async (eventId: string, taskId: string) => ({ eventId, taskId, token: "lease" })),
      complete: vi.fn(async () => undefined),
      release: vi.fn(async () => undefined)
    }
  };
}

function subscriptionAdmission(candidates: readonly AgentEventSubscription[]) {
  return {
    matchEventSubscriptions: vi.fn(async () => [...candidates]),
    admit: vi.fn(async (event: AgentEvent, identity: { tenantId: string; agentUuid: string }) => {
      const taskId = agentTaskId({
        tenantId: identity.tenantId,
        agentUuid: identity.agentUuid,
        triggerType: event.eventType,
        triggerRef: event.aggregateId
      });
      return { taskId, runId: agentRunId(taskId), runStatus: "running" as const };
    }),
    complete: vi.fn(async () => undefined)
  };
}

function subscription(subscriptionId: string, filterKind: "all" | "message_contains_any", filter: unknown): AgentEventSubscription {
  return {
    subscriptionId, definitionId: "DEF-1", definitionVersion: 1,
    tenantId: "dipole", agentId: "UAI", eventType: "message.direct.created",
    resourceType: "conversation", resourceId: "direct:U100:UAI", filterKind, filter
  };
}

function messageEnvelope(targetUuid = "UAI", eventId = "E1"): object {
  return {
    event_id: eventId, request_id: "R1", trace_id: "T1",
    event_type: "message.direct.created", version: "v1", source: "dipole",
    occurred_at: "2026-08-27T08:00:00.000Z",
    payload: {
      mutation_type: "created", revision: 1, actor_uuid: "U100", message_id: "M100",
      conversation_key: "direct:U100:UAI", message_seq: 1, sender_uuid: "U100", target_uuid: targetUuid,
      target_type: 0, message_type: 0, content: "hello", sent_at: "2026-08-27T08:00:00.000Z"
    }
  };
}

function payload(envelope: object): KafkaInboundPayload {
  return {
    topic: "message.direct.created",
    message: { key: Buffer.from("M100"), value: Buffer.from(JSON.stringify(envelope)) }
  };
}
