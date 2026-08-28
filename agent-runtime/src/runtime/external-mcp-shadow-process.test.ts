import { describe, expect, it, vi } from "vitest";

import type { AgentTaskWorkerActivities } from "../temporal/agent-task-activities.js";
import type { ExternalMcpTemporalRouteSelectorFactory } from "../temporal/external-mcp-temporal-client-lifecycle.js";
import type { ExternalMcpShadowTemporalRuntime } from "../temporal/external-mcp-shadow-temporal-runtime.js";
import { loadTemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import {
  startExternalMcpShadowProcess,
  type ExternalMcpShadowProcessSeams
} from "./external-mcp-shadow-process.js";
import {
  loadShadowRuntimeConfig,
  type ShadowRuntime,
  type ShadowSubscriptionMatcher
} from "./shadow-runtime.js";

describe("external MCP Shadow process owner", () => {
  it("keeps disabled Kafka configuration free of Temporal and Kafka side effects", async () => {
    const harness = processHarness({ shadowEnabled: false });

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).resolves.toBeUndefined();

    expect(harness.startTemporal).not.toHaveBeenCalled();
    expect(harness.createKafka).not.toHaveBeenCalled();
  });

  it("rejects non-subscription trigger mode before constructing resources", async () => {
    const harness = processHarness({ triggerMode: "direct_target" });

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow process requires subscription trigger mode$/);
    expect(harness.startTemporal).not.toHaveBeenCalled();
    expect(harness.createKafka).not.toHaveBeenCalled();
  });

  it("does not construct Kafka when the external MCP Temporal deployment is disabled", async () => {
    const harness = processHarness();
    harness.startTemporal.mockResolvedValueOnce(undefined);

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).resolves.toBeUndefined();
    expect(harness.createKafka).not.toHaveBeenCalled();
  });

  it("starts Temporal before Kafka and stops Kafka before Temporal once", async () => {
    const harness = processHarness();
    const controller = new AbortController();
    const onFailure = vi.fn();
    const env = { DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true" };

    const process = await startExternalMcpShadowProcess(
      env, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      { signal: controller.signal }, onFailure, harness.seams
    );

    expect(harness.startTemporal).toHaveBeenCalledWith(
      env,
      harness.shadow,
      harness.temporalConfig,
      harness.activities,
      harness.createRoutes,
      { signal: controller.signal },
      onFailure
    );
    expect(harness.createKafka).toHaveBeenCalledWith(
      harness.shadow,
      harness.temporal,
      harness.subscriptionMatcher
    );
    expect(harness.order).toEqual(["temporal-start", "kafka-create", "kafka-start"]);

    await process!.stop();
    await process!.stop();
    expect(harness.order).toEqual([
      "temporal-start", "kafka-create", "kafka-start", "kafka-stop", "temporal-stop"
    ]);
    expect(harness.kafkaStop).toHaveBeenCalledOnce();
    expect(harness.temporalStop).toHaveBeenCalledOnce();
  });

  it("propagates pre-start cancellation without resources", async () => {
    const harness = processHarness();
    const controller = new AbortController();
    controller.abort(new Error("cancelled before Kafka process"));

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow("cancelled before Kafka process");
    expect(harness.startTemporal).not.toHaveBeenCalled();
  });

  it("returns Temporal ownership when Kafka construction fails", async () => {
    const harness = processHarness();
    harness.createKafka.mockImplementationOnce(() => {
      harness.order.push("kafka-create");
      throw new Error("sensitive Kafka brokers");
    });

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow process startup failed$/);
    expect(harness.order).toEqual(["temporal-start", "kafka-create", "temporal-stop"]);
  });

  it("returns Temporal ownership when its borrowed subscription matcher is unavailable", async () => {
    const harness = processHarness({ omitSubscriptionMatcher: true });

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow process startup failed$/);
    expect(harness.createKafka).not.toHaveBeenCalled();
    expect(harness.order).toEqual(["temporal-start", "temporal-stop"]);
  });

  it("stops partially started Kafka before Temporal when Kafka startup fails", async () => {
    const harness = processHarness({ kafkaStartError: new Error("sensitive Kafka target") });

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow process startup failed$/);
    expect(harness.order).toEqual([
      "temporal-start", "kafka-create", "kafka-start", "kafka-stop", "temporal-stop"
    ]);
  });

  it("closes Kafka and Temporal when cancellation wins during Kafka startup", async () => {
    const harness = processHarness();
    const controller = new AbortController();
    harness.kafkaStart.mockImplementationOnce(async () => {
      harness.order.push("kafka-start");
      controller.abort(new Error("cancelled after Kafka startup"));
    });

    await expect(startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow("cancelled after Kafka startup");
    expect(harness.order).toEqual([
      "temporal-start", "kafka-create", "kafka-start", "kafka-stop", "temporal-stop"
    ]);
  });

  it("continues Temporal shutdown after Kafka failure and memoizes rejection", async () => {
    const harness = processHarness({ kafkaStopError: new Error("sensitive Kafka close") });
    const process = await startExternalMcpShadowProcess(
      {}, harness.shadow, harness.temporalConfig, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    );

    await expect(process!.stop()).rejects.toThrow(/^External MCP Shadow process shutdown failed$/);
    await expect(process!.stop()).rejects.toThrow(/^External MCP Shadow process shutdown failed$/);
    expect(harness.kafkaStop).toHaveBeenCalledOnce();
    expect(harness.temporalStop).toHaveBeenCalledOnce();
  });
});

function processHarness(options: {
  readonly shadowEnabled?: boolean;
  readonly triggerMode?: "direct_target" | "subscription";
  readonly kafkaStartError?: Error;
  readonly kafkaStopError?: Error;
  readonly temporalStopError?: Error;
  readonly omitSubscriptionMatcher?: boolean;
} = {}) {
  const order: string[] = [];
  const shadow = loadShadowRuntimeConfig(options.shadowEnabled === false ? {} : {
    DIPOLE_AGENT_KAFKA_ENABLED: "true",
    DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
    DIPOLE_AGENT_TRIGGER_MODE: options.triggerMode ?? "subscription",
    DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
    DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091",
    DIPOLE_INTERNAL_RPC_SHARED_SECRET: "test-secret"
  });
  const temporalConfig = loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" });
  const activities = {} as AgentTaskWorkerActivities;
  const createRoutes = vi.fn(() => ({})) as unknown as ExternalMcpTemporalRouteSelectorFactory;
  const subscriptionMatcher = {
    matchEventSubscriptions: vi.fn(async () => [])
  } satisfies ShadowSubscriptionMatcher;
  const temporalStop = vi.fn(async () => {
    order.push("temporal-stop");
    if (options.temporalStopError !== undefined) throw options.temporalStopError;
  });
  const temporal = {
    deployment: {}, worker: {}, temporal: temporalConfig,
    ...(options.omitSubscriptionMatcher === true ? {} : { subscriptionMatcher }),
    dispatch: vi.fn(), stop: temporalStop
  } as unknown as ExternalMcpShadowTemporalRuntime;
  const startTemporal = vi.fn<ExternalMcpShadowProcessSeams["startTemporal"]>(async () => {
    order.push("temporal-start");
    return temporal;
  });
  const kafkaStart = vi.fn(async () => {
    order.push("kafka-start");
    if (options.kafkaStartError !== undefined) throw options.kafkaStartError;
  });
  const kafkaStop = vi.fn(async () => {
    order.push("kafka-stop");
    if (options.kafkaStopError !== undefined) throw options.kafkaStopError;
  });
  const kafka = { start: kafkaStart, stop: kafkaStop } satisfies ShadowRuntime;
  const createKafka = vi.fn<ExternalMcpShadowProcessSeams["createKafka"]>(() => {
    order.push("kafka-create");
    return kafka;
  });
  const seams = { startTemporal, createKafka } satisfies ExternalMcpShadowProcessSeams;
  return {
    order, shadow, temporalConfig, activities, createRoutes, temporal, subscriptionMatcher, temporalStop,
    startTemporal, kafka, kafkaStart, kafkaStop, createKafka, seams
  };
}
