import { randomUUID } from "node:crypto";
import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { StreamableHTTPClientTransport, type Transport } from "@modelcontextprotocol/client";
import { createMcpHandler, Server } from "@modelcontextprotocol/server";
import { TestWorkflowEnvironment } from "@temporalio/testing";
import { Worker } from "@temporalio/worker";
import { Kafka, Partitioners } from "kafkajs";
import { createPool, type Pool } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { createExternalMcpReadCapabilityDefinitions } from "../mcp/external-mcp-read-capability-definitions.js";
import {
  externalMcpDeploymentRouteManifestSchemaVersion,
  loadExternalMcpDeploymentRouteManifest
} from "../mcp/external-mcp-deployment-route-manifest.js";
import type { ExternalMcpConfig } from "../mcp/external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "../mcp/external-mcp-production-io.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import { agentTaskId } from "../events/shadow-processor.js";
import { createPersistentAgentTaskLifecycleActivities } from "../temporal/agent-task-lifecycle-activities.js";
import { foundationAgentTaskActivities } from "../temporal/agent-task-activities.js";
import { createTemporalMcpDispatchRuntime } from "../temporal/mcp-dispatch-runtime.js";
import { TemporalMcpShadowTaskDispatcher } from "../temporal/mcp-shadow-task-dispatcher.js";
import { TemporalMcpSubscriptionRouteSelector } from "../temporal/mcp-subscription-route-selector.js";
import { agentTaskWorkflowId, TemporalMcpTaskClient } from "../temporal/temporal-task-client.js";
import { TemporalMcpWorkflowExecutionCatalog } from "../temporal/mcp-workflow-envelope.js";
import { createAgentCapabilityRPC, createKafkaShadowRuntime, type ShadowRuntimeConfig } from "./shadow-runtime.js";
import { createExternalMcpShadowDrillEvidence } from "./external-mcp-shadow-drill-evidence.js";

const enabled = process.env.DIPOLE_AGENT_FULL_STACK_DRILL === "true";
const integration = describe.skipIf(!enabled);

integration("external MCP isolated full-stack Shadow drill", () => {
  const database = `dipole_mcp_drill_${randomUUID().replaceAll("-", "")}`;
  const topicPrefix = `drill_${randomUUID().replaceAll("-", "")}`;
  const groupId = `dipole-agent-shadow-drill-${randomUUID()}`;
  let admin: Pool;
  let databasePool: Pool;
  let temporal: TestWorkflowEnvironment;
  let fixtureDirectory: string;

  beforeAll(async () => {
    const mysqlUrl = requiredEnv("DIPOLE_TEST_AGENT_MYSQL_URL");
    admin = createPool({ uri: mysqlUrl, timezone: "Z", connectionLimit: 4 });
    await admin.query(`CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`);
    const parsed = new URL(mysqlUrl);
    parsed.pathname = `/${database}`;
    databasePool = createPool({ uri: parsed.toString(), timezone: "Z", connectionLimit: 4, multipleStatements: true });
    for (const migration of ["000018_agent_event_ledger.up.sql", "000020_agent_shadow_trajectory.up.sql"]) {
      await databasePool.query(await readFile(new URL(`../../../../db/migrations/${migration}`, import.meta.url), "utf8"));
    }
    temporal = await TestWorkflowEnvironment.createLocal();
    fixtureDirectory = await mkdtemp(join(tmpdir(), "dipole-mcp-full-stack-"));
  }, 120_000);

  afterAll(async () => {
    await temporal?.teardown();
    await databasePool?.end();
    if (admin !== undefined) {
      await admin.query(`DROP DATABASE IF EXISTS \`${database}\``);
      await admin.end();
    }
    if (fixtureDirectory !== undefined) {
      await rm(fixtureDirectory, { recursive: true, force: true });
    }
  });

  it("converges one event, restart replay, and expired-readiness denial", async () => {
    const config = externalConfig();
    const io = externalIo(fixtureDirectory);
    const manifestPath = join(fixtureDirectory, "routes.json");
    await writeFile(manifestPath, JSON.stringify(routeManifest()), { mode: 0o600 });
    await chmod(manifestPath, 0o600);
    const loaded = await loadExternalMcpDeploymentRouteManifest(
      config,
      createExternalMcpReadCapabilityDefinitions(),
      { DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST: manifestPath },
      { expectedOwnerUid: process.getuid!() }
    );
    if (loaded === undefined) throw new Error("route manifest unavailable");

    const shadowConfig = runtimeConfig(database, topicPrefix, groupId);
    const rpc = createAgentCapabilityRPC(shadowConfig);
    const core = rpc.client;
    const observation = { toolCalls: 0 };
    const mcp = localMcpFixture(config, observation);
    const route = loaded.routes[0]!;
    const dispatch = createTemporalMcpDispatchRuntime(route, {
      routes: loaded.registry,
      core,
      artifacts: core,
      externalMcp: {
        config,
        io,
        registry: mcp.registry,
        readinessBindingOptions: readinessOptions()
      },
      ownerTokenSha256: () => "e".repeat(64)
    });
    const activities = {
      ...foundationAgentTaskActivities,
      ...createPersistentAgentTaskLifecycleActivities(core),
      ...dispatch.activities
    };
    const taskQueue = `dipole-mcp-drill-${randomUUID()}`;
    const worker = await Worker.create({
      connection: temporal.nativeConnection,
      namespace: temporal.namespace ?? "default",
      taskQueue,
      workflowsPath: new URL("../temporal/agent-task-workflow.ts", import.meta.url).pathname,
      activities
    });
    const workerRun = worker.run();
    const dispatcher = new TemporalMcpShadowTaskDispatcher(
      new TemporalMcpTaskClient(
        temporal.client.workflow,
        taskQueue,
        new TemporalMcpWorkflowExecutionCatalog([dispatch.routeBinding])
      ),
      new TemporalMcpSubscriptionRouteSelector(loaded.subscriptionRoutes)
    );
    let runtime = createKafkaShadowRuntime(shadowConfig, dispatcher, core);

    try {
      await runtime.start();
      await waitForConsumerGroup(shadowConfig);
      await publish(shadowConfig, eventEnvelope("EVENT-MCP-DRILL-1", "MESSAGE-MCP-DRILL-1"));
      const firstTaskId = taskId("MESSAGE-MCP-DRILL-1");
      await expect(workflowResult(temporal, firstTaskId))
        .resolves.toMatchObject({ status: "completed" });
      expect(observation.toolCalls).toBe(1);
      expect(await readRPCFixtureState()).toMatchObject({
        rpc_type: "go_internal_grpc_mtls", rpc_authenticated: true, artifact_count: 1
      });

      await runtime.stop();
      runtime = createKafkaShadowRuntime(shadowConfig, dispatcher, core);
      await runtime.start();
      await waitForConsumerGroup(shadowConfig);
      await publish(shadowConfig, eventEnvelope("EVENT-MCP-DRILL-1", "MESSAGE-MCP-DRILL-1"));
      await waitFor(async () => Number((await databasePool.query<Array<{ count: number } & import("mysql2").RowDataPacket>>(
        "SELECT COUNT(*) AS count FROM agent_event_ledger WHERE event_id = 'EVENT-MCP-DRILL-1' AND status = 'completed'"
      ))[0][0]?.count) === 1);
      await delay(750);
      expect(observation.toolCalls).toBe(1);

      await writeFile(requiredEnv("DIPOLE_TEST_AGENT_RPC_STALE_PATH"), "stale\n", { mode: 0o600 });
      await publish(shadowConfig, eventEnvelope("EVENT-MCP-DRILL-2", "MESSAGE-MCP-DRILL-2"));
      const deniedTaskId = taskId("MESSAGE-MCP-DRILL-2");
      await expect(workflowResult(temporal, deniedTaskId))
        .resolves.toMatchObject({ status: "failed" });
      expect(observation.toolCalls).toBe(1);
      expect(await readRPCFixtureState()).toMatchObject({
        rpc_type: "go_internal_grpc_mtls", rpc_authenticated: true, artifact_count: 1,
        finished_statuses: ["completed", "failed"]
      });
      expect(requiredEnv("DIPOLE_TEST_AGENT_RPC_IDENTITY_DENIALS_VERIFIED")).toBe("true");

      await writeEvidence(createExternalMcpShadowDrillEvidence({
        event_count: 2,
        ledger_completed_event_count: 2,
        tool_call_count: 1,
        artifact_count: 1,
        restart_duplicate_suppressed: true,
        expired_readiness_denied: true,
        core_rpc_type: "go_internal_grpc_mtls",
        core_rpc_authenticated: true,
        core_rpc_identity_denials_verified: true
      }));
    } finally {
      await runtime.stop().catch(() => undefined);
      worker.shutdown();
      await workerRun;
      await mcp.close();
      rpc.close();
    }
  }, 120_000);

  function taskId(messageId: string): string {
    return agentTaskId({ tenantId: "dipole", agentUuid: "UAI-DRILL", triggerType: "message.direct.created", triggerRef: messageId });
  }
});

function localMcpFixture(config: Extract<ExternalMcpConfig, { enabled: true }>, observation: { toolCalls: number }) {
  const server = new Server({ name: "github-mcp", version: "1.0.0" }, { capabilities: { tools: {} } });
  server.setRequestHandler("tools/list", async () => ({
    tools: [{ name: "read_issue", inputSchema: { type: "object" as const } }]
  }));
  server.setRequestHandler("tools/call", async request => {
    observation.toolCalls += 1;
    return { content: [{ type: "text" as const, text: `issue:${String(request.params.arguments?.issue_number)}` }] };
  });
  const handler = createMcpHandler(() => server);
  return {
    registry: {
      describe: () => config.profiles[0]!,
      connect: async () => new StreamableHTTPClientTransport(new URL("https://drill.invalid/mcp"), {
        fetch: (url, init) => handler.fetch(new Request(url, init))
      }) as Transport
    },
    close: () => handler.close()
  };
}

function externalConfig(): Extract<ExternalMcpConfig, { enabled: true }> {
  return { enabled: true, profiles: [{
    profileId: "github-drill", tenantId: "dipole", serverId: "github-mcp", endpoint: "https://github.invalid/mcp",
    credentialRef: "CRED-0123456789ABCDEF", credentialVersion: 1, allowedHosts: ["github.invalid"], allowedPorts: [443],
    dnsResolution: "public_only", tlsServerName: "github.invalid", caBundleRef: "CA-0123456789ABCDEF",
    allowedTools: ["read_issue"]
  }] };
}

function externalIo(root: string): ExternalMcpProductionIoConfig {
  return {
    credentialCatalogPath: join(root, "catalog.json"),
    secretProvider: {
      providerId: "local-aes-gcm", keys: { "KEY-0123456789ABCDEF": join(root, "key.bin") },
      secrets: { "SECRET-0123456789ABCDEF": { keyRef: "KEY-0123456789ABCDEF", path: join(root, "secret.bin") } }
    },
    caBundles: { "CA-0123456789ABCDEF": join(root, "ca.pem") }
  };
}

function routeManifest() {
  return { schema_version: externalMcpDeploymentRouteManifestSchemaVersion, routes: [{
    route_id: "repository-issue-read", route_version: 1, capability_id: "repository.issue.read",
    workflow_step: 1, ordinal: 1, profile_id: "github-drill", server_id: "github-mcp", tool_name: "read_issue",
    egress_policy: { allowed_argument_names: ["owner", "repo", "issue_number"], maximum_bytes: 512 },
    subscription_trigger: {
      definition_id: "DEF-REPOSITORY-GUARDIAN", definition_version: 1,
      arguments: { owner: "dipole", repo: "dipole", issue_number: 1 }
    }
  }] };
}

function runtimeConfig(database: string, topicPrefix: string, groupId: string): ShadowRuntimeConfig {
  const mysqlUrl = new URL(requiredEnv("DIPOLE_TEST_AGENT_MYSQL_URL"));
  return {
    enabled: true, runtimeMode: "shadow", candidateVersion: "", releaseManifestPath: "", brokers: requiredEnv("DIPOLE_TEST_AGENT_KAFKA_BROKERS").split(","), clientId: `dipole-agent-drill-${randomUUID()}`,
    groupId, topic: "message.direct.created", topicPrefix, failureMaxAttempts: 2, topicPartitions: 1,
    topicReplicationFactor: 1, tenantId: "dipole", agentUuid: "UAI-DRILL", triggerMode: "subscription",
    subscriptionShadowEnabled: false,
    ledgerMode: "mysql", leaseMs: 5_000, modelMode: "metadata", modelProvider: { kind: "disabled", name: "", baseURL: "", apiKey: "", supportsStructuredOutputs: false }, modelRoutes: [], contextCompilerVersion: "v1",
    memoryEnabled: false, retrievalEnabled: false, retrievalContextEnabled: false, modelContextProfiles: [], modelBudget: { maxCalls: 1, totalTimeoutMs: 1_000, maxOutputTokensPerCall: 128 },
    capabilityRpc: { enabled: true, target: requiredEnv("DIPOLE_TEST_AGENT_RPC_TARGET"),
      secret: requiredEnv("DIPOLE_TEST_AGENT_RPC_SECRET"), timeoutMs: 2_000, tls: {
      enabled: true, caFile: requiredEnv("DIPOLE_TEST_AGENT_RPC_CA_FILE"),
      certFile: requiredEnv("DIPOLE_TEST_AGENT_RPC_CERT_FILE"), keyFile: requiredEnv("DIPOLE_TEST_AGENT_RPC_KEY_FILE"),
      serverName: requiredEnv("DIPOLE_TEST_AGENT_RPC_SERVER_NAME")
    } },
    mysql: { host: mysqlUrl.hostname, port: Number(mysqlUrl.port), user: mysqlUrl.username,
      password: mysqlUrl.password, database }
  };
}

function eventEnvelope(eventId: string, messageId: string): string {
  return JSON.stringify({
    event_id: eventId, request_id: `REQ-${eventId}`, trace_id: `TRACE-${eventId}`,
    event_type: "message.direct.created", version: "v1", source: "dipole", occurred_at: new Date().toISOString(),
    payload: {
      mutation_type: "created", revision: 1, actor_uuid: "U100", message_id: messageId,
      conversation_key: "direct:U100:U200", message_seq: 1, sender_uuid: "U100", target_uuid: "U200",
      target_type: 0, message_type: 1, content: "redacted drill event", sent_at: new Date().toISOString()
    }
  });
}

async function publish(config: ShadowRuntimeConfig, value: string): Promise<void> {
  const kafka = new Kafka({ clientId: "dipole-agent-drill-producer", brokers: [...config.brokers] });
  const producer = kafka.producer({ createPartitioner: Partitioners.DefaultPartitioner });
  await producer.connect();
  try {
    await producer.send({ topic: `${config.topicPrefix}.${config.topic}`, messages: [{ value }] });
  } finally {
    await producer.disconnect();
  }
}

async function waitForConsumerGroup(config: ShadowRuntimeConfig): Promise<void> {
  const kafka = new Kafka({ clientId: "dipole-agent-drill-observer", brokers: [...config.brokers] });
  const admin = kafka.admin();
  await admin.connect();
  try {
    await waitFor(async () => {
      const result = await admin.describeGroups([config.groupId]);
      const group = result.groups[0];
      return group?.state === "Stable" && group.members.length > 0 &&
        group.members.some(member => member.memberAssignment.byteLength > 0);
    });
  } finally {
    await admin.disconnect();
  }
}

function readinessOptions() {
  return { expectedOwnerUid: process.getuid!(), trustedTransportBuilder: true };
}

async function waitFor(predicate: () => Promise<boolean>): Promise<void> {
  for (let attempt = 0; attempt < 300; attempt += 1) {
    if (await predicate()) return;
    await delay(100);
  }
  throw new Error("drill condition timed out");
}

async function workflowResult(temporal: TestWorkflowEnvironment, taskId: string): Promise<unknown> {
  const handle = temporal.client.workflow.getHandle(agentTaskWorkflowId(taskId));
  await waitFor(async () => {
    try {
      await handle.describe();
      return true;
    } catch {
      return false;
    }
  });
  return handle.result();
}

function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function writeEvidence(value: Readonly<Record<string, unknown>>): Promise<void> {
  const path = requiredEnv("DIPOLE_AGENT_MCP_DRILL_EVIDENCE");
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, `${canonicalMcpJSON(value)}\n`, { mode: 0o600 });
}

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}

interface AgentRPCFixtureState {
  rpc_type: string;
  rpc_authenticated: boolean;
  rpc_call_count: number;
  artifact_count: number;
  finished_statuses: string[];
}

async function readRPCFixtureState(): Promise<AgentRPCFixtureState> {
  const value = JSON.parse(await readFile(requiredEnv("DIPOLE_TEST_AGENT_RPC_STATE_PATH"), "utf8")) as Record<string, unknown>;
  if (value.schema_version !== "dipole.agent.mcp-rpc-drill-state.v1" || value.rpc_type !== "go_internal_grpc_mtls" ||
      value.rpc_authenticated !== true || !Number.isSafeInteger(value.rpc_call_count) || Number(value.rpc_call_count) < 1 ||
      !Number.isSafeInteger(value.artifact_count) || !Array.isArray(value.finished_statuses) ||
      value.finished_statuses.some(item => typeof item !== "string")) {
    throw new Error("Agent RPC drill fixture state is invalid");
  }
  return value as unknown as AgentRPCFixtureState;
}
