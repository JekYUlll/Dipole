import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-proto";
import { resourceFromAttributes } from "@opentelemetry/resources";
import {
  BatchSpanProcessor,
  NodeTracerProvider,
  ParentBasedSampler,
  TraceIdRatioBasedSampler
} from "@opentelemetry/sdk-trace-node";

interface AgentObservabilityBaseConfig {
  readonly sampleRatio: number;
  readonly exportTimeoutMs: number;
  readonly serviceName: string;
}

export type AgentObservabilityConfig =
  | (AgentObservabilityBaseConfig & { readonly enabled: false })
  | (AgentObservabilityBaseConfig & { readonly enabled: true; readonly endpoint: string });

interface AgentTelemetrySDK {
  start(): void;
  shutdown(): Promise<void>;
}

type EnabledAgentObservabilityConfig = Extract<AgentObservabilityConfig, { readonly enabled: true }>;
type AgentTelemetrySDKFactory = (config: EnabledAgentObservabilityConfig) => AgentTelemetrySDK;

export interface AgentObservabilityRuntime {
  start(): void;
  stop(): Promise<void>;
}

export function loadAgentObservabilityConfig(env: NodeJS.ProcessEnv): AgentObservabilityConfig {
  const enabled = booleanValue(env.DIPOLE_AGENT_OTEL_ENABLED, false, "DIPOLE_AGENT_OTEL_ENABLED");
  if (!enabled) {
    return { enabled, sampleRatio: 0.1, exportTimeoutMs: 5000, serviceName: "dipole-agent" };
  }
  const serviceName = boundedName(env.OTEL_SERVICE_NAME ?? "dipole-agent", "OTEL_SERVICE_NAME");
  const sampleRatio = decimalValue(env.OTEL_TRACES_SAMPLER_ARG ?? "0.1", 0, 1, "OTEL trace sample ratio");
  const exportTimeoutMs = integerValue(
    env.OTEL_EXPORTER_OTLP_TRACES_TIMEOUT ?? env.OTEL_EXPORTER_OTLP_TIMEOUT ?? "5000",
    100,
    30_000,
    "OTEL trace export timeout"
  );
  const protocol = (env.OTEL_EXPORTER_OTLP_TRACES_PROTOCOL ?? env.OTEL_EXPORTER_OTLP_PROTOCOL ?? "http/protobuf").trim();
  if (protocol !== "http/protobuf") throw new Error("Agent OTEL trace protocol must be http/protobuf");
  const sampler = (env.OTEL_TRACES_SAMPLER ?? "parentbased_traceidratio").trim().toLowerCase();
  if (sampler !== "parentbased_traceidratio") {
    throw new Error("Agent OTEL sampler must be parentbased_traceidratio");
  }
  const tracesEndpoint = env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT?.trim();
  const baseEndpoint = env.OTEL_EXPORTER_OTLP_ENDPOINT?.trim();
  if (tracesEndpoint === undefined && baseEndpoint === undefined) {
    throw new Error("Enabled Agent OTEL requires OTEL_EXPORTER_OTLP_TRACES_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT");
  }
  const endpoint = httpEndpoint(tracesEndpoint ?? appendTracePath(baseEndpoint!));
  return { enabled, endpoint, sampleRatio, exportTimeoutMs, serviceName };
}

export function createAgentObservabilityRuntime(
  config: AgentObservabilityConfig,
  factory: AgentTelemetrySDKFactory = createNodeSDK
): AgentObservabilityRuntime {
  let sdk: AgentTelemetrySDK | undefined;
  let started = false;
  let stopPromise: Promise<void> | undefined;
  return {
    start() {
      if (started) throw new Error("Agent observability Runtime is already started");
      started = true;
      if (!config.enabled) return;
      sdk = factory(config);
      sdk.start();
    },
    stop() {
      stopPromise ??= sdk?.shutdown() ?? Promise.resolve();
      return stopPromise;
    }
  };
}

function createNodeSDK(config: EnabledAgentObservabilityConfig): AgentTelemetrySDK {
  const exporter = new OTLPTraceExporter({ url: config.endpoint, timeoutMillis: config.exportTimeoutMs });
  const provider = new NodeTracerProvider({
    resource: resourceFromAttributes({ "service.name": config.serviceName }),
    sampler: new ParentBasedSampler({ root: new TraceIdRatioBasedSampler(config.sampleRatio) }),
    spanProcessors: [new BatchSpanProcessor(exporter, { exportTimeoutMillis: config.exportTimeoutMs })],
    spanLimits: { attributeCountLimit: 32, attributeValueLengthLimit: 256, eventCountLimit: 8, linkCountLimit: 8 },
    forceFlushTimeoutMillis: config.exportTimeoutMs
  });
  return { start: () => provider.register(), shutdown: () => provider.shutdown() };
}

function appendTracePath(raw: string): string {
  const endpoint = new URL(raw);
  endpoint.pathname = `${endpoint.pathname.replace(/\/$/, "")}/v1/traces`;
  return endpoint.toString();
}

function httpEndpoint(raw: string): string {
  let endpoint: URL;
  try {
    endpoint = new URL(raw);
  } catch {
    throw new Error("Agent OTEL endpoint must be an absolute HTTP URL");
  }
  if ((endpoint.protocol !== "http:" && endpoint.protocol !== "https:") || endpoint.username !== "" || endpoint.password !== "") {
    throw new Error("Agent OTEL endpoint must be an absolute HTTP URL without embedded credentials");
  }
  endpoint.hash = "";
  return endpoint.toString();
}

function booleanValue(raw: string | undefined, fallback: boolean, name: string): boolean {
  if (raw === undefined || raw.trim() === "") return fallback;
  const value = raw.trim().toLowerCase();
  if (value === "true") return true;
  if (value === "false") return false;
  throw new Error(`${name} must be true or false`);
}

function decimalValue(raw: string, minimum: number, maximum: number, name: string): number {
  const value = Number(raw);
  if (!Number.isFinite(value) || value < minimum || value > maximum) throw new Error(`${name} must be between ${minimum} and ${maximum}`);
  return value;
}

function integerValue(raw: string, minimum: number, maximum: number, name: string): number {
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < minimum || value > maximum) throw new Error(`${name} must be an integer between ${minimum} and ${maximum}`);
  return value;
}

function boundedName(raw: string, name: string): string {
  const value = raw.trim();
  if (value.length < 1 || value.length > 64 || !/^[A-Za-z0-9._-]+$/.test(value)) {
    throw new Error(`${name} must contain 1-64 safe characters`);
  }
  return value;
}
