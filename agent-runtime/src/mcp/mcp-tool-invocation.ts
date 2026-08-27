import { createHash, randomUUID } from "node:crypto";
import { SpanStatusCode, trace, type Span, type Tracer } from "@opentelemetry/api";

import type { ExecutionContext } from "../runtime/execution-context.js";

export interface McpToolInvocationBegin {
  invocationId: string;
  taskId: string;
  runId: string;
  toolName: string;
  capabilityId: string;
  argumentsSha256: string;
  requestId?: string;
  traceId?: string;
}

export type McpToolInvocationFinish = {
  invocationId: string;
  taskId: string;
  runId: string;
  latencyMs: number;
} & ({ status: "completed"; resultSha256: string; resultBytes: number } | { status: "failed"; errorCode: string });

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
    private readonly timeoutMs: number = 5_000
  ) {
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
      throw new Error("MCP Tool timeout must be between 100 and 60000 milliseconds");
    }
  }

  execute(
    tool: { name: string; capabilityId: string },
    rawArguments: unknown,
    context: ExecutionContext,
    operation: (signal: AbortSignal) => Promise<unknown>
  ): Promise<string> {
    return this.tracer.startActiveSpan("agent.tool.call", {}, async (span) => {
      const invocationId = this.idGenerator();
      const startedAt = this.now();
      this.decorateSpan(span, invocationId, tool, context);
      try {
        await this.audit.begin({
          invocationId, taskId: context.taskId, runId: context.runId, toolName: tool.name,
          capabilityId: tool.capabilityId, argumentsSha256: sha256(canonicalJSON(rawArguments)),
          ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
          ...(context.traceId === undefined ? {} : { traceId: context.traceId })
        });
      } catch (error) {
        this.failSpan(span, error);
        span.end();
        throw new Error("Tool audit unavailable");
      }

      try {
        let rawResult: unknown;
        try {
          rawResult = await operationWithTimeout(operation, this.timeoutMs);
        } catch (error) {
          await this.finishFailed(invocationId, context, startedAt, error instanceof ToolOperationTimeout ? "tool_timeout" : "tool_execution_failed");
          throw new ToolInvocationFailure();
        }
        const result = canonicalJSON(rawResult);
        const resultBytes = Buffer.byteLength(result);
        if (resultBytes > 64 * 1024) {
          await this.finishFailed(invocationId, context, startedAt, "result_too_large");
          throw new ToolInvocationFailure("result_too_large");
        }
        await this.audit.finish({
          invocationId, taskId: context.taskId, runId: context.runId, status: "completed",
          resultSha256: sha256(result), resultBytes, latencyMs: elapsedMilliseconds(startedAt, this.now())
        });
        span.setStatus({ code: SpanStatusCode.OK });
        return result;
      } catch (error) {
        if (!(error instanceof ToolInvocationFailure)) {
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
class ToolOperationTimeout extends Error {}

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

function canonicalJSON(value: unknown): string {
  return JSON.stringify(value) ?? "null";
}

function elapsedMilliseconds(startedAt: number, finishedAt: number): number {
  return Math.max(0, Math.round(finishedAt - startedAt));
}
