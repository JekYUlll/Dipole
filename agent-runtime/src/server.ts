import Fastify, { type FastifyInstance } from "fastify";
import { timingSafeEqual } from "node:crypto";

import { AgentTaskControlError, type AgentTaskControlIdentity } from "./control/agent-task-control.js";

export interface RuntimeReadiness {
  isReady(): boolean;
}

export interface AgentTaskControlAPI {
  getTask(input: AgentTaskControlIdentity): Promise<unknown>;
  cancelTask(input: AgentTaskControlIdentity & { reason?: string }): Promise<void>;
  resolveApproval(input: AgentTaskControlIdentity & { approvalId: string; decision: "approved" | "denied" }): Promise<void>;
  provideInput(input: AgentTaskControlIdentity & { requestId: string; value: unknown }): Promise<void>;
}

export interface AgentTaskControlHTTPOptions {
  secret: string;
  service: AgentTaskControlAPI;
}

export function buildServer(readiness: RuntimeReadiness, control?: AgentTaskControlHTTPOptions): FastifyInstance {
  const server = Fastify({ logger: false });

  server.get("/livez", async () => ({ status: "ok", service: "dipole-agent" }));
  server.get("/readyz", async (_request, reply) => {
    if (!readiness.isReady()) {
      return reply.code(503).send({ status: "not_ready", service: "dipole-agent" });
    }
    return { status: "ready", service: "dipole-agent" };
  });

  if (control !== undefined) {
    if (control.secret.trim().length === 0) {
      throw new Error("Agent Task control HTTP secret is required");
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

  return server;
}

function trustedControlIdentity(headers: Record<string, string | string[] | undefined>, taskId: string, secret: string): AgentTaskControlIdentity | undefined {
  const caller = header(headers, "x-dipole-caller-service");
  const providedSecret = header(headers, "x-dipole-service-token");
  const principalUserId = header(headers, "x-dipole-principal-user-id");
  const normalizedTaskId = taskId.trim();
  if (caller !== "dipole-gateway" || !safeEqual(providedSecret, secret) || !validIdentifier(normalizedTaskId) || !validIdentifier(principalUserId)) {
    return undefined;
  }
  const requestId = header(headers, "x-request-id");
  const traceId = header(headers, "x-trace-id");
  return {
    taskId: normalizedTaskId,
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
