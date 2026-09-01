import Fastify, { type FastifyInstance } from "fastify";
import { timingSafeEqual } from "node:crypto";
import { Readable } from "node:stream";

import { AgentTaskControlError, type AgentTaskControlIdentity, type AgentTaskTimeline } from "./control/agent-task-control.js";

export interface RuntimeReadiness {
  isReady(): boolean;
}

export interface AgentTaskControlAPI {
  startTask?(input: { principalUserId: string; requestId?: string; traceId?: string; body: unknown }): Promise<{ taskId: string; status: "accepted" }>;
  getTask(input: AgentTaskControlIdentity): Promise<unknown>;
  getTimeline?(input: AgentTaskControlIdentity & { afterSeq: bigint; limit: number }): Promise<AgentTaskTimeline>;
  cancelTask(input: AgentTaskControlIdentity & { reason?: string }): Promise<void>;
  resolveApproval(input: AgentTaskControlIdentity & { approvalId: string; decision: "approved" | "denied" }): Promise<void>;
  provideInput(input: AgentTaskControlIdentity & { requestId: string; value: unknown }): Promise<void>;
}

export interface AgentTaskControlHTTPOptions {
  secret: string;
  service: AgentTaskControlAPI;
}

export interface AgentMcpHttpHandler {
  fetch(request: Request, options?: { authInfo?: unknown; parsedBody?: unknown }): Promise<Response>;
}

export interface AgentMcpHTTPOptions {
  secret: string;
  resource: string;
  handler: AgentMcpHttpHandler;
}

export interface OAuthCallbackHandoffNotification {
  readonly handoffId: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface OAuthCallbackHandoffControlAPI {
  notifyHandoff(notification: OAuthCallbackHandoffNotification): Promise<void>;
}

export interface OAuthCallbackHandoffNotificationDeduplicator {
  claim(handoffId: string): boolean;
  release(handoffId: string): void;
}

export interface OAuthCallbackHandoffHTTPOptions {
  secret: string;
  service: OAuthCallbackHandoffControlAPI;
  deduplicator?: OAuthCallbackHandoffNotificationDeduplicator;
}

export class BoundedOAuthCallbackHandoffNotificationDeduplicator implements OAuthCallbackHandoffNotificationDeduplicator {
  private readonly handoffs = new Map<string, true>();

  constructor(private readonly capacity = 1_024) {
    if (!Number.isSafeInteger(capacity) || capacity < 1 || capacity > 10_000) throw new Error("OAuth callback handoff deduplicator capacity is invalid");
  }

  claim(handoffId: string): boolean {
    if (this.handoffs.has(handoffId)) return false;
    if (this.handoffs.size >= this.capacity) this.handoffs.delete(this.handoffs.keys().next().value!);
    this.handoffs.set(handoffId, true);
    return true;
  }

  release(handoffId: string): void {
    this.handoffs.delete(handoffId);
  }
}

export interface AgentMetrics {
  render(): string;
}

export function buildServer(
  readiness: RuntimeReadiness,
  control?: AgentTaskControlHTTPOptions,
  mcp?: AgentMcpHTTPOptions,
  metrics?: AgentMetrics,
  oauthCallbackHandoff?: OAuthCallbackHandoffHTTPOptions
): FastifyInstance {
  const server = Fastify({ logger: false });

  server.get("/livez", async () => ({ status: "ok", service: "dipole-agent" }));
  server.get("/readyz", async (_request, reply) => {
    if (!readiness.isReady()) {
      return reply.code(503).send({ status: "not_ready", service: "dipole-agent" });
    }
    return { status: "ready", service: "dipole-agent" };
  });
  if (metrics !== undefined) {
    server.get("/metrics", async (_request, reply) => reply.type("text/plain; version=0.0.4; charset=utf-8").send(metrics.render()));
  }

  if (control !== undefined) {
    if (control.secret.trim().length === 0) {
      throw new Error("Agent Task control HTTP secret is required");
    }
    const startTask = control.service.startTask;
    if (startTask !== undefined) {
      server.post<{ Body?: unknown }>("/internal/v1/agent/tasks", async (request, reply) => {
        const identity = trustedControlRequestIdentity(request.headers, control.secret);
        if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent Task control authentication failed" });
        try {
          const result = await startTask({ ...identity, body: request.body });
          return reply.code(202).send({ taskId: result.taskId, status: result.status });
        } catch (error) {
          return sendControlError(reply, error);
        }
      });
    }
    server.get<{ Params: { taskId: string } }>("/internal/v1/agent/tasks/:taskId", async (request, reply) => {
      const identity = trustedControlIdentity(request.headers, request.params.taskId, control.secret);
      if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent Task control authentication failed" });
      try {
        return await control.service.getTask(identity);
      } catch (error) {
        return sendControlError(reply, error);
      }
    });
    server.get<{ Params: { taskId: string }; Querystring: { after?: string; limit?: string } }>("/internal/v1/agent/tasks/:taskId/timeline", async (request, reply) => {
      const identity = trustedControlIdentity(request.headers, request.params.taskId, control.secret);
      if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent Task control authentication failed" });
      let afterSeq: bigint;
      try { afterSeq = request.query.after === undefined ? 0n : BigInt(request.query.after); }
      catch { return reply.code(400).send({ code: 400, message: "Agent Task Timeline cursor is invalid" }); }
      const limit = request.query.limit === undefined ? 50 : Number(request.query.limit);
      if (afterSeq < 0n || !Number.isSafeInteger(Number(afterSeq)) || !Number.isInteger(limit) || limit < 1 || limit > 100) {
        return reply.code(400).send({ code: 400, message: "Agent Task Timeline pagination is invalid" });
      }
      if (control.service.getTimeline === undefined) {
        return reply.code(503).send({ code: 503, message: "Agent Task Timeline is unavailable" });
      }
      try {
        return serializeAgentTaskTimeline(await control.service.getTimeline({ ...identity, afterSeq, limit }));
      } catch (error) {
        return sendControlError(reply, error);
      }
    });
    server.post<{ Params: { taskId: string }; Body?: { reason?: unknown } }>("/internal/v1/agent/tasks/:taskId/cancel", async (request, reply) => {
      const identity = trustedControlIdentity(request.headers, request.params.taskId, control.secret);
      if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent Task control authentication failed" });
      const reason = typeof request.body?.reason === "string" ? request.body.reason : undefined;
      try {
        await control.service.cancelTask({ ...identity, ...(reason === undefined ? {} : { reason }) });
        return reply.code(202).send({ taskId: identity.taskId, status: "cancellation_requested" });
      } catch (error) {
        return sendControlError(reply, error);
      }
    });
    server.post<{ Params: { taskId: string; approvalId: string }; Body?: { decision?: unknown } }>("/internal/v1/agent/tasks/:taskId/approvals/:approvalId", async (request, reply) => {
      const identity = trustedControlIdentity(request.headers, request.params.taskId, control.secret);
      if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent Task control authentication failed" });
      const decision = request.body?.decision;
      if (decision !== "approved" && decision !== "denied") {
        return reply.code(400).send({ code: 400, message: "approval decision must be approved or denied" });
      }
      try {
        await control.service.resolveApproval({ ...identity, approvalId: request.params.approvalId, decision });
        return reply.code(202).send({ taskId: identity.taskId, approvalId: request.params.approvalId, status: "resolution_requested" });
      } catch (error) {
        return sendControlError(reply, error);
      }
    });
    server.post<{ Params: { taskId: string; requestId: string }; Body?: { value?: unknown } }>("/internal/v1/agent/tasks/:taskId/inputs/:requestId", async (request, reply) => {
      const identity = trustedControlIdentity(request.headers, request.params.taskId, control.secret);
      if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent Task control authentication failed" });
      if (!validIdentifier(request.params.requestId)) return reply.code(400).send({ code: 400, message: "Agent Task input request ID is invalid" });
      if (request.body?.value === undefined) return reply.code(400).send({ code: 400, message: "Agent Task input value is required" });
      try {
        await control.service.provideInput({ ...identity, requestId: request.params.requestId, value: request.body.value });
        return reply.code(202).send({ taskId: identity.taskId, requestId: request.params.requestId, status: "input_accepted" });
      } catch (error) {
        return sendControlError(reply, error);
      }
    });
  }

  if (mcp !== undefined) {
    if (mcp.secret.trim().length === 0) {
      throw new Error("Agent MCP HTTP secret is required");
    }
    server.route<{
      Params: { taskId: string; runId: string };
      Body?: unknown;
    }>({
      method: ["GET", "POST", "DELETE"],
      url: "/internal/v1/agent/tasks/:taskId/runs/:runId/mcp",
      bodyLimit: 256 * 1024,
      handler: async (request, reply) => {
        const identity = trustedMcpIdentity(request.headers, request.params.taskId, request.params.runId, mcp.secret, mcp.resource);
        if (identity === undefined) return reply.code(401).send({ code: 401, message: "Agent MCP authentication failed" });

        const headers = new Headers();
        for (const [name, value] of Object.entries(request.headers)) {
          if (value !== undefined && !privateForwardHeader(name)) headers.set(name, Array.isArray(value) ? value.join(",") : value);
        }
        const body = request.method === "POST" && request.body !== undefined ? JSON.stringify(request.body) : undefined;
        const cancellation = new AbortController();
        const abort = (): void => cancellation.abort(new Error("MCP client disconnected"));
        request.raw.once("aborted", abort);
        reply.raw.once("close", abort);
        const mcpRequest = new Request(`http://dipole-agent.local${request.raw.url}`, {
          method: request.method,
          headers,
          signal: cancellation.signal,
          ...(body === undefined ? {} : { body })
        });
        const response = await mcp.handler.fetch(mcpRequest, {
          authInfo: {
            token: "gateway-authenticated",
            clientId: identity.principalUserId,
            scopes: [identity.scope],
            extra: {
              resource: identity.resource,
              taskId: identity.taskId,
              runId: identity.runId,
              ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
              ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
            }
          },
          ...(request.body === undefined ? {} : { parsedBody: request.body })
        });
        for (const [name, value] of response.headers.entries()) reply.header(name, value);
        reply.code(response.status);
        if (response.body === null) return reply.send();
        return reply.send(Readable.from(response.body));
      }
    });
  }

  if (oauthCallbackHandoff !== undefined) {
    if (oauthCallbackHandoff.secret.trim().length === 0) {
      throw new Error("OAuth callback handoff HTTP secret is required");
    }
    const deduplicator = oauthCallbackHandoff.deduplicator ?? new BoundedOAuthCallbackHandoffNotificationDeduplicator();
    server.post<{ Body?: unknown }>("/internal/v1/agent/oauth/callback-handoffs", async (request, reply) => {
      const correlation = trustedOAuthCallbackHandoffCorrelation(request.headers, oauthCallbackHandoff.secret);
      if (correlation === undefined) return reply.code(401).send({ code: 401, message: "OAuth callback handoff authentication failed" });
      const handoffId = oauthCallbackHandoffID(request.body);
      if (handoffId === undefined) return reply.code(400).send({ code: 400, message: "OAuth callback handoff is invalid" });
      if (!deduplicator.claim(handoffId)) return reply.code(202).send();
      try {
        await oauthCallbackHandoff.service.notifyHandoff({ handoffId, ...correlation });
        return reply.code(202).send();
      } catch {
        deduplicator.release(handoffId);
        return reply.code(503).send({ code: 503, message: "OAuth callback handoff is unavailable" });
      }
    });
  }

  return server;
}

function serializeAgentTaskTimeline(timeline: AgentTaskTimeline) {
  return {
    schemaVersion: timeline.schemaVersion,
    taskId: timeline.taskId,
    revision: safeTimelineNumber(timeline.revision, "revision"),
    events: timeline.events.map((event) => ({
      eventSeq: event.eventSeq.toString(),
      eventId: event.eventId,
      taskId: event.taskId,
      runId: event.runId,
      kind: event.kind,
      status: event.status,
      capabilityId: event.capabilityId,
      approvalId: event.approvalId,
      artifactId: event.artifactId,
      occurredAtUnixMs: safeTimelineNumber(event.occurredAtUnixMs, "event timestamp")
    })),
    nextCursor: timeline.nextCursor
  };
}

function safeTimelineNumber(value: bigint, field: string): number {
  if (value < 0n || value > BigInt(Number.MAX_SAFE_INTEGER)) {
    throw new Error(`Agent Task Timeline ${field} exceeds the HTTP number range`);
  }
  return Number(value);
}

interface TrustedMcpIdentity extends AgentTaskControlIdentity {
  runId: string;
  resource: string;
  scope: string;
}

const AGENT_MCP_READ_SCOPE = "dipole.agent.mcp.read";

function trustedMcpIdentity(
  headers: Record<string, string | string[] | undefined>,
  taskId: string,
  runId: string,
  secret: string,
  expectedResource: string
): TrustedMcpIdentity | undefined {
  const base = trustedControlIdentity(headers, taskId, secret);
  const normalizedRunId = runId.trim();
  const resource = header(headers, "x-dipole-oauth-resource");
  const scope = header(headers, "x-dipole-oauth-scope");
  if (base === undefined || !validIdentifier(normalizedRunId) || resource !== expectedResource || scope !== AGENT_MCP_READ_SCOPE) return undefined;
  return { ...base, runId: normalizedRunId, resource, scope };
}

function privateForwardHeader(name: string): boolean {
  const normalized = name.toLowerCase();
  return normalized === "authorization" || normalized === "cookie" || normalized === "x-dipole-caller-service" ||
    normalized === "x-dipole-service-token" || normalized === "x-dipole-principal-user-id" ||
    normalized === "x-dipole-oauth-resource" || normalized === "x-dipole-oauth-scope" ||
    normalized === "content-length" || normalized === "transfer-encoding" || normalized === "connection" || normalized === "host";
}

function trustedControlIdentity(headers: Record<string, string | string[] | undefined>, taskId: string, secret: string): AgentTaskControlIdentity | undefined {
  const requestIdentity = trustedControlRequestIdentity(headers, secret);
  const normalizedTaskId = taskId.trim();
  if (requestIdentity === undefined || !validIdentifier(normalizedTaskId)) return undefined;
  return { taskId: normalizedTaskId, ...requestIdentity };
}

function trustedControlRequestIdentity(headers: Record<string, string | string[] | undefined>, secret: string): Omit<AgentTaskControlIdentity, "taskId"> | undefined {
  const caller = header(headers, "x-dipole-caller-service");
  const providedSecret = header(headers, "x-dipole-service-token");
  const principalUserId = header(headers, "x-dipole-principal-user-id");
  if (caller !== "dipole-gateway" || !safeEqual(providedSecret, secret) || !validIdentifier(principalUserId)) {
    return undefined;
  }
  const requestId = header(headers, "x-request-id");
  const traceId = header(headers, "x-trace-id");
  return {
    principalUserId,
    ...(requestId === "" ? {} : { requestId: requestId.slice(0, 128) }),
    ...(traceId === "" ? {} : { traceId: traceId.slice(0, 128) })
  };
}

function header(headers: Record<string, string | string[] | undefined>, name: string): string {
  const value = headers[name];
  return (Array.isArray(value) ? value[0] : value)?.trim() ?? "";
}

function safeEqual(left: string, right: string): boolean {
  const leftBuffer = Buffer.from(left);
  const rightBuffer = Buffer.from(right);
  return leftBuffer.length === rightBuffer.length && timingSafeEqual(leftBuffer, rightBuffer);
}

function validIdentifier(value: string): boolean {
  return value.length > 0 && value.length <= 128 && /^[A-Za-z0-9._:-]+$/.test(value);
}

function trustedOAuthCallbackHandoffCorrelation(headers: Record<string, string | string[] | undefined>, secret: string): { requestId?: string; traceId?: string } | undefined {
  if (header(headers, "x-dipole-caller-service") !== "dipole-gateway" || !safeEqual(header(headers, "x-dipole-service-token"), secret) ||
      header(headers, "x-dipole-principal-user-id") !== "") return undefined;
  const requestId = header(headers, "x-request-id");
  const traceId = header(headers, "x-trace-id");
  if ((requestId !== "" && !validIdentifier(requestId)) || (traceId !== "" && !validIdentifier(traceId))) return undefined;
  return { ...(requestId === "" ? {} : { requestId }), ...(traceId === "" ? {} : { traceId }) };
}

function oauthCallbackHandoffID(body: unknown): string | undefined {
  if (body === null || typeof body !== "object" || Array.isArray(body)) return undefined;
  const fields = Object.keys(body);
  if (fields.length !== 1 || fields[0] !== "handoff_id") return undefined;
  const handoffId = (body as { handoff_id?: unknown }).handoff_id;
  return typeof handoffId === "string" && /^[A-Za-z0-9_-]{16,64}$/.test(handoffId) ? handoffId : undefined;
}

function sendControlError(reply: { code(statusCode: number): { send(payload: unknown): unknown } }, error: unknown): unknown {
  if (error instanceof AgentTaskControlError) {
    const status = error.code === "invalid_argument" ? 400 : error.code === "not_found" ? 404 : 409;
    return reply.code(status).send({ code: status, message: error.message });
  }
  const externalCode = typeof error === "object" && error !== null && "code" in error ? Number(error.code) : -1;
  const name = error instanceof Error ? error.name : "";
  if (externalCode === 5 || name === "WorkflowNotFoundError") {
    return reply.code(404).send({ code: 404, message: "Agent Task unavailable" });
  }
  return reply.code(502).send({ code: 502, message: "Agent Task control dependency unavailable" });
}
