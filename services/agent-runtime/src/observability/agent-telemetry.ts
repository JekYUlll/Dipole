import { SpanStatusCode, trace, type Span, type Tracer } from "@opentelemetry/api";

type LowSensitivityAttribute = string | number | boolean;

export interface AgentSpanContext {
  readonly taskId: string;
  readonly runId?: string;
  readonly attributes?: Readonly<Record<string, LowSensitivityAttribute>>;
}

export class AgentTelemetry {
  constructor(private readonly tracer: Tracer = trace.getTracer("dipole-agent-runtime")) {}

  withSpan<T>(
    name: string,
    context: AgentSpanContext,
    operation: (span: Span) => Promise<T>
  ): Promise<T> {
    return this.tracer.startActiveSpan(name, {}, async span => {
      span.setAttributes({
        "dipole.agent.task.id": required(context.taskId, "Task ID"),
        ...(context.runId === undefined ? {} : { "dipole.agent.run.id": required(context.runId, "Run ID") }),
        ...context.attributes
      });
      try {
        const result = await operation(span);
        span.setStatus({ code: SpanStatusCode.OK });
        return result;
      } catch (error) {
        span.recordException(new Error(`${name} failed`));
        span.setStatus({ code: SpanStatusCode.ERROR });
        throw error;
      } finally {
        span.end();
      }
    });
  }
}

function required(value: string, label: string): string {
  value = value.trim();
  if (value.length < 1 || value.length > 128) throw new Error(`Agent telemetry ${label} must contain 1-128 characters`);
  return value;
}
