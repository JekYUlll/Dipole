import { createHash, randomUUID } from "node:crypto";
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

import type {
  AgentArtifactCreateInput,
  AgentArtifactRecord,
  AgentMcpToolCommand,
  AgentMcpToolRoundFinish
} from "../capabilities/agent-capability-rpc.js";
import { createExternalMcpReadCapabilityDefinitions } from "../mcp/external-mcp-read-capability-definitions.js";
import {
  externalMcpDeploymentRouteManifestSchemaVersion,
  loadExternalMcpDeploymentRouteManifest
} from "../mcp/external-mcp-deployment-route-manifest.js";
import type { ExternalMcpConfig } from "../mcp/external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "../mcp/external-mcp-production-io.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import { agentRunId, agentTaskId, type AgentEvent, type AgentIdentity } from "../events/shadow-processor.js";
import { createPersistentAgentTaskLifecycleActivities, type PersistentAgentRunLifecyclePort } from "../temporal/agent-task-lifecycle-activities.js";
import { foundationAgentTaskActivities } from "../temporal/agent-task-activities.js";
import { createTemporalMcpDispatchRuntime, type TemporalMcpDispatchRuntimeCore } from "../temporal/mcp-dispatch-runtime.js";
import { TemporalMcpShadowTaskDispatcher } from "../temporal/mcp-shadow-task-dispatcher.js";
import { TemporalMcpSubscriptionRouteSelector } from "../temporal/mcp-subscription-route-selector.js";
import { agentTaskWorkflowId, TemporalMcpTaskClient } from "../temporal/temporal-task-client.js";
import { TemporalMcpWorkflowExecutionCatalog } from "../temporal/mcp-workflow-envelope.js";
import { createKafkaShadowRuntime, type ShadowRuntimeConfig } from "./shadow-runtime.js";

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
      await databasePool.query(await readFile(new URL(`../../../db/migrations/${migration}`, import.meta.url), "utf8"));
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

    const core = new DrillCore();
    const mcp = localMcpFixture(config, core);
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
    const shadowConfig = runtimeConfig(database, topicPrefix, groupId);
    let runtime = createKafkaShadowRuntime(shadowConfig, dispatcher, core);

    try {
      await runtime.start();
      await waitForConsumerGroup(shadowConfig);
      await publish(shadowConfig, eventEnvelope("EVENT-MCP-DRILL-1", "MESSAGE-MCP-DRILL-1"));
      const firstTaskId = taskId("MESSAGE-MCP-DRILL-1");
      await expect(workflowResult(temporal, firstTaskId))
        .resolves.toMatchObject({ status: "completed" });
      expect(core.toolCalls).toBe(1);
      expect(core.artifacts).toBe(1);

      await runtime.stop();
      runtime = createKafkaShadowRuntime(shadowConfig, dispatcher, core);
      await runtime.start();
      await waitForConsumerGroup(shadowConfig);
      await publish(shadowConfig, eventEnvelope("EVENT-MCP-DRILL-1", "MESSAGE-MCP-DRILL-1"));
      await waitFor(async () => Number((await databasePool.query<Array<{ count: number } & import("mysql2").RowDataPacket>>(
        "SELECT COUNT(*) AS count FROM agent_event_ledger WHERE event_id = 'EVENT-MCP-DRILL-1' AND status = 'completed'"
      ))[0][0]?.count) === 1);
      await delay(750);
      expect(core.toolCalls).toBe(1);

      core.readinessFresh = false;
      await publish(shadowConfig, eventEnvelope("EVENT-MCP-DRILL-2", "MESSAGE-MCP-DRILL-2"));
      const deniedTaskId = taskId("MESSAGE-MCP-DRILL-2");
      await expect(workflowResult(temporal, deniedTaskId))
        .resolves.toMatchObject({ status: "failed" });
      expect(core.toolCalls).toBe(1);
      expect(core.finishedStatuses).toEqual(["completed", "failed"]);

      await writeEvidence({
        schema_version: "dipole.agent.external-mcp-shadow-drill.v1",
        outcome: "passed",
        isolation: "disposable_mysql_kafka_temporal_and_local_mcp",
        event_count: 2,
        ledger_completed_event_count: 2,
        tool_call_count: 1,
        artifact_count: 1,
        restart_duplicate_suppressed: true,
        expired_readiness_denied: true,
        production_authority: false
      });
    } finally {
      await runtime.stop().catch(() => undefined);
      worker.shutdown();
      await workerRun;
      await mcp.close();
    }
  }, 120_000);

  function taskId(messageId: string): string {
    return agentTaskId({ tenantId: "dipole", agentUuid: "UAI-DRILL", triggerType: "message.direct.created", triggerRef: messageId });
  }
});

class DrillCore implements TemporalMcpDispatchRuntimeCore, PersistentAgentRunLifecyclePort {
  readonly commands = new Map<string, AgentMcpToolCommand>();
  readonly rounds = new Map<string, AgentMcpToolRoundFinish>();
  readonly finishedStatuses: string[] = [];
  readinessFresh = true;
  toolCalls = 0;
  artifacts = 0;

  async matchEventSubscriptions(event: AgentEvent, identity: AgentIdentity) {
    return [{
      subscriptionId: "SUB-MCP-DRILL", definitionId: "DEF-REPOSITORY-GUARDIAN", definitionVersion: 1,
      tenantId: identity.tenantId, agentId: identity.agentUuid, eventType: event.eventType,
      resourceType: "conversation" as const, resourceId: String(event.payload.conversation_key),
      filterKind: "all" as const, filter: {}
    }];
  }

  async admitRun(input: Parameters<PersistentAgentRunLifecyclePort["admitRun"]>[0]) {
    const taskIdValue = agentTaskId({
      tenantId: input.tenantId, agentUuid: input.agentId, triggerType: input.triggerType, triggerRef: input.triggerRef
    });
    return { taskId: taskIdValue, runId: agentRunId(taskIdValue), runStatus: "running" as const };
  }

  async finish(_taskId: string, _runId: string, status: "completed" | "failed" | "cancelled") {
    this.finishedStatuses.push(status);
  }
  async requestApproval() { throw new Error("read drill cannot request approval"); }
  async resolveApproval() { throw new Error("read drill cannot resolve approval"); }
  async projectTaskWorkflowState() { return {}; }

  async resolveMcpContext(taskId: string, runId: string, principalUserId: string) {
    return {
      tenantId: "dipole", principalUuid: principalUserId, agentUuid: "UAI-DRILL", taskId, runId,
      mode: "shadow" as const, permissions: ["repository.issue.read"],
      resourceScopes: [{ resourceType: "repository_issue", resourceId: "dipole/dipole#1", actions: ["read"] }],
      approvedCapabilities: []
    };
  }

  async beginMcpToolCommand(input: Parameters<TemporalMcpDispatchRuntimeCore["beginMcpToolCommand"]>[0]) {
    if (!this.commands.has(input.invocationId)) {
      this.commands.set(input.invocationId, {
        invocationId: input.invocationId, tenantId: "dipole", principalUserId: "U100", agentId: "UAI-DRILL",
        taskId: input.taskId, runId: input.runId, profileId: input.profileId!, serverId: input.serverId!,
        toolName: input.toolName, capabilityId: input.capabilityId,
        arguments: JSON.parse(input.argumentsJson!) as Record<string, unknown>, argumentsSha256: input.argumentsSha256,
        startedAtUnixMs: Date.now(), status: "running"
      });
    }
    return { invocationId: input.invocationId, status: this.commands.get(input.invocationId)!.status };
  }

  async resolveMcpToolCommand(_taskId: string, _runId: string, invocationId: string) {
    const command = this.commands.get(invocationId);
    if (command === undefined) throw new Error("missing drill command");
    return command;
  }

  async claimMcpToolRound(input: Parameters<TemporalMcpDispatchRuntimeCore["claimMcpToolRound"]>[0]) {
    const receipt = this.rounds.get(input.roundId);
    if (receipt === undefined) return { outcome: "claimed" as const };
    if (receipt.status === "failed") return { outcome: "replay_failed" as const, errorCode: receipt.errorCode };
    return { outcome: "replay_completed" as const, result: JSON.parse(receipt.resultJSON) as unknown,
      resultJSON: receipt.resultJSON, resultSha256: receipt.resultSha256 };
  }

  async finishMcpToolRound(input: AgentMcpToolRoundFinish) {
    this.rounds.set(input.roundId, input);
  }

  async finishMcpToolInvocationFromRound(input: { taskId: string; runId: string; invocationId: string; roundId: string }) {
    const command = await this.resolveMcpToolCommand(input.taskId, input.runId, input.invocationId);
    const round = this.rounds.get(input.roundId);
    const status = round?.status === "failed" ? "failed" as const : "completed" as const;
    this.commands.set(input.invocationId, { ...command, status });
    return { invocationId: input.invocationId, status };
  }

  async resolveFreshMcpReadinessEvidence(_tenantId: string, profileBindingSha256: string, runtimeBindingSha256: string) {
    const now = Date.now();
    const expiresAt = this.readinessFresh ? now + 60_000 : now - 1;
    return {
      evidenceId: "e".repeat(64), profileBindingSha256, runtimeBindingSha256, contentSha256: "c".repeat(64),
      collectedAt: new Date(now - 1_000).toISOString(), expiresAt: new Date(expiresAt).toISOString()
    };
  }

  async createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord> {
    this.artifacts += 1;
    const contentSha256 = sha(input.content);
    return {
      schemaVersion: "dipole.agent.artifact.v1",
      artifactId: sha(Buffer.from(["dipole.agent.artifact.v1", input.taskId, input.runId, input.artifactType, input.version, contentSha256].join("\n"))),
      taskId: input.taskId, runId: input.runId, artifactType: input.artifactType, version: input.version,
      title: input.title, mediaType: input.mediaType, contentSha256, sizeBytes: input.content.byteLength,
      metadata: input.metadata
    };
  }
}

function localMcpFixture(config: Extract<ExternalMcpConfig, { enabled: true }>, core: DrillCore) {
  const server = new Server({ name: "github-mcp", version: "1.0.0" }, { capabilities: { tools: {} } });
  server.setRequestHandler("tools/list", async () => ({
    tools: [{ name: "read_issue", inputSchema: { type: "object" as const } }]
  }));
  server.setRequestHandler("tools/call", async request => {
    core.toolCalls += 1;
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
    enabled: true, brokers: requiredEnv("DIPOLE_TEST_AGENT_KAFKA_BROKERS").split(","), clientId: `dipole-agent-drill-${randomUUID()}`,
    groupId, topic: "message.direct.created", topicPrefix, failureMaxAttempts: 2, topicPartitions: 1,
    topicReplicationFactor: 1, tenantId: "dipole", agentUuid: "UAI-DRILL", triggerMode: "subscription",
    ledgerMode: "mysql", leaseMs: 5_000, modelMode: "metadata", modelRoutes: [], contextCompilerVersion: "v1",
    memoryEnabled: false, modelContextProfiles: [], modelBudget: { maxCalls: 1, totalTimeoutMs: 1_000, maxOutputTokensPerCall: 128 },
    capabilityRpc: { enabled: true, target: "127.0.0.1:1", secret: "drill", timeoutMs: 500, tls: {
      enabled: false, caFile: "", certFile: "", keyFile: "", serverName: ""
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

function sha(value: Uint8Array): string {
  return createHash("sha256").update(value).digest("hex");
}
