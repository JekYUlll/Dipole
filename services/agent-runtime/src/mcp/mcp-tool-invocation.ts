import { createHash, randomUUID } from "node:crypto";
import { SpanStatusCode, trace, type Span, type Tracer } from "@opentelemetry/api";

import type { ExecutionContext } from "../runtime/execution-context.js";
import { canonicalMcpJSON } from "./canonical-json.js";

export interface McpToolInvocationBegin {
  invocationId: string;
  taskId: string;
  runId: string;
  toolName: string;
  capabilityId: string;
  argumentsSha256: string;
  profileId?: string;
  serverId?: string;
  argumentsJson?: string;
  requestId?: string;
  traceId?: string;
  approvalId?: string;
}

export interface McpToolActionReference {
  readonly resourceType: "message";
  readonly resourceId: string;
  readonly commandKind: "assistant_reply" | "system_message";
  readonly commandId: string;
}

export type McpToolInvocationFinish = {
  invocationId: string;
  taskId: string;
  runId: string;
  latencyMs: number;
} & ({ status: "completed"; resultSha256: string; resultBytes: number; actionReference?: McpToolActionReference } | { status: "failed"; errorCode: string });

export interface McpToolInvocationAuditPort {
  begin(input: McpToolInvocationBegin): Promise<void>;
  finish(input: McpToolInvocationFinish): Promise<void>;
}

export class McpToolInvocationRunner {
  constructor(
    private readonly audit: McpToolInvocationAuditPort,
    private readonly tracer: Tracer = trace.getTracer("dipole-agent-runtime"),
    private readonly idGenerator: () => string = randomUUID,
    private readonly now: () => number = performance.now.bind(performance),
    private readonly timeoutMs: number = 5_000,
    private readonly preserveOpenInvocationOnFailure: (error: unknown) => boolean = () => false
  ) {
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
      throw new Error("MCP Tool timeout must be between 100 and 60000 milliseconds");
    }
  }

  execute(
    tool: { name: string; capabilityId: string; approvalId?: string; profileId?: string; serverId?: string },
    rawArguments: unknown,
    context: ExecutionContext,
    operation: (signal: AbortSignal, invocationId: string) => Promise<unknown>,
    actionReference?: (result: unknown) => McpToolActionReference
  ): Promise<string> {
    if ((tool.approvalId === undefined) !== (actionReference === undefined)) {
      throw new Error("MCP write audit requires both Approval and action reference binding");
    }
    return this.tracer.startActiveSpan("agent.tool.call", {}, async (span) => {
      const invocationId = this.idGenerator();
      const startedAt = this.now();
      this.decorateSpan(span, invocationId, tool, context);
      try {
        const canonicalArguments = canonicalMcpJSON(rawArguments);
        if ((tool.profileId === undefined) !== (tool.serverId === undefined)) throw new Error("MCP external Tool binding is incomplete");
        await this.audit.begin({
          invocationId, taskId: context.taskId, runId: context.runId, toolName: tool.name,
          capabilityId: tool.capabilityId, argumentsSha256: sha256(canonicalArguments),
          ...(tool.profileId === undefined ? {} : { profileId: tool.profileId, serverId: tool.serverId, argumentsJson: canonicalArguments }),
          ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
          ...(context.traceId === undefined ? {} : { traceId: context.traceId }),
          ...(tool.approvalId === undefined ? {} : { approvalId: tool.approvalId })
        });
      } catch (error) {
        this.failSpan(span, error);
        span.end();
        throw new Error("Tool audit unavailable");
      }

      try {
        let rawResult: unknown;
        try {
          rawResult = await operationWithTimeout(signal => operation(signal, invocationId), this.timeoutMs);
        } catch (error) {
          console.error("Agent Tool operation failed", {
            taskId: context.taskId,
            runId: context.runId,
            invocationId,
            toolName: tool.name,
            error: error instanceof Error ? error.message : "unknown operation error"
          });
          // A caller can opt into retrying an uncertain transport result while
          // retaining the durable invocation for an idempotent command replay.
          if (this.preserveOpenInvocationOnFailure(error)) throw new ToolInvocationFailure();
          await this.finishFailed(invocationId, context, startedAt, error instanceof ToolOperationTimeout ? "tool_timeout" : "tool_execution_failed");
          throw new ToolInvocationFailure();
        }
        const result = canonicalMcpJSON(rawResult);
        const resultBytes = Buffer.byteLength(result);
        if (resultBytes > 64 * 1024) {
          await this.finishFailed(invocationId, context, startedAt, "result_too_large");
          throw new ToolInvocationFailure("result_too_large");
        }
        const reference = actionReference?.(rawResult);
        const finish: McpToolInvocationFinish = {
          invocationId, taskId: context.taskId, runId: context.runId, status: "completed",
          resultSha256: sha256(result), resultBytes, latencyMs: elapsedMilliseconds(startedAt, this.now()),
          ...(reference === undefined ? {} : { actionReference: reference })
        };
        await this.finishCompleted(finish);
        span.setStatus({ code: SpanStatusCode.OK });
        return result;
      } catch (error) {
        if (!(error instanceof ToolInvocationFailure) && !(error instanceof ToolInvocationTerminalUncertainFailure)) {
          // The command may already be committed when its terminal audit fails.
          // Keep the RPC details in the service log so recovery can distinguish
          // a receipt mismatch from a transport failure without logging content.
          console.error("Agent Tool completion audit failed", {
            taskId: context.taskId,
            runId: context.runId,
            invocationId,
            toolName: tool.name,
            grpcCode: grpcErrorCode(error),
            grpcDetails: grpcErrorDetails(error)
          });
          try {
            await this.finishFailed(invocationId, context, startedAt, "tool_execution_failed");
          } catch {
            // The caller receives a stable failure while the span retains the local cause.
          }
        }
        this.failSpan(span, error);
        throw new Error("Tool invocation failed");
      } finally {
        span.end();
      }
    });
  }

  private finishFailed(invocationId: string, context: ExecutionContext, startedAt: number, errorCode: string): Promise<void> {
    return this.audit.finish({
      invocationId, taskId: context.taskId, runId: context.runId, status: "failed", errorCode,
      latencyMs: elapsedMilliseconds(startedAt, this.now())
    });
  }

  private async finishCompleted(finish: McpToolInvocationFinish): Promise<void> {
    try {
      await this.audit.finish(finish);
    } catch (error) {
      if (!isUncertainGrpcFailure(error)) throw error;
      try {
        // Core accepts an identical terminal replay. Retrying once recovers a
        // response lost after Core commits without widening the write scope.
        await this.audit.finish(finish);
      } catch (retryError) {
        if (isUncertainGrpcFailure(retryError)) throw new ToolInvocationTerminalUncertainFailure();
        throw retryError;
      }
    }
  }

  private decorateSpan(span: Span, invocationId: string, tool: { name: string; capabilityId: string }, context: ExecutionContext): void {
    span.setAttribute("dipole.agent.tool.invocation_id", invocationId);
    span.setAttribute("dipole.agent.tool.name", tool.name);
    span.setAttribute("dipole.agent.capability.id", tool.capabilityId);
    span.setAttribute("dipole.agent.task.id", context.taskId);
    span.setAttribute("dipole.agent.run.id", context.runId);
  }

  private failSpan(span: Span, _error: unknown): void {
    span.recordException(new Error("agent.tool.call failed"));
    span.setStatus({ code: SpanStatusCode.ERROR });
  }
}

class ToolInvocationFailure extends Error {}
class ToolInvocationTerminalUncertainFailure extends Error {}
class ToolOperationTimeout extends Error {}

function grpcErrorCode(error: unknown): number | undefined {
  if (typeof error !== "object" || error === null || !("code" in error)) return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === "number" ? code : undefined;
}

function grpcErrorDetails(error: unknown): string | undefined {
  if (typeof error !== "object" || error === null || !("details" in error)) return undefined;
  const details = (error as { details?: unknown }).details;
  return typeof details === "string" && details.trim() ? details : undefined;
}

function isUncertainGrpcFailure(error: unknown): boolean {
  const code = grpcErrorCode(error);
  return code === 4 || code === 14;
}

function operationWithTimeout(operation: (signal: AbortSignal) => Promise<unknown>, timeoutMs: number): Promise<unknown> {
  const controller = new AbortController();
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      controller.abort(new ToolOperationTimeout());
      reject(new ToolOperationTimeout());
    }, timeoutMs);
    Promise.resolve()
      .then(() => operation(controller.signal))
      .then(resolve, reject)
      .finally(() => clearTimeout(timer));
  });
}

function sha256(value: string): string {
  return createHash("sha256").update(value).digest("hex");
}

function elapsedMilliseconds(startedAt: number, finishedAt: number): number {
  return Math.max(0, Math.round(finishedAt - startedAt));
}
