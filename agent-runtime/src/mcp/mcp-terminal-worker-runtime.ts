import { z } from "zod";

import type { AgentMcpToolCommandTerminalResult } from "../capabilities/agent-capability-rpc.js";
import type { McpElicitationResultInput } from "./mcp-durable-elicitation.js";
import { McpToolRoundTerminalError } from "./mcp-input-required-activity.js";
import type { McpWorkerCommandDispatcher, McpWorkerDispatchResult } from "./mcp-worker-dispatch.js";

const identitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const dispatchIdentitySchema = z.object({
  taskId: identitySchema,
  runId: identitySchema,
  invocationId: identitySchema
}).passthrough();

export interface McpInvocationTerminalClient {
  finishMcpToolInvocationFromRound(input: {
    readonly taskId: string;
    readonly runId: string;
    readonly invocationId: string;
    readonly roundId: string;
  }): Promise<AgentMcpToolCommandTerminalResult>;
}

export class McpTerminalWorkerRuntime {
  constructor(
    private readonly dispatcher: McpWorkerCommandDispatcher,
    private readonly terminals: McpInvocationTerminalClient
  ) {}

  async begin(rawInput: unknown, signal?: AbortSignal): Promise<McpWorkerDispatchResult> {
    const identity = dispatchIdentitySchema.parse(rawInput);
    return this.execute(identity, () => this.dispatcher.begin(rawInput, signal));
  }

  async resume(rawCheckpoint: unknown, input: McpElicitationResultInput, signal?: AbortSignal): Promise<Extract<McpWorkerDispatchResult, { kind: "complete" }>> {
    const identity = dispatchIdentitySchema.parse(rawCheckpoint);
    return this.execute(identity, () => this.dispatcher.resume(rawCheckpoint, input, signal)) as Promise<Extract<McpWorkerDispatchResult, { kind: "complete" }>>;
  }

  private async execute(
    identity: { readonly taskId: string; readonly runId: string; readonly invocationId: string },
    dispatch: () => Promise<McpWorkerDispatchResult>
  ): Promise<McpWorkerDispatchResult> {
    try {
      const result = await dispatch();
      if (result.kind === "complete") {
        await this.finish(identity, result.receipt.roundId, "completed");
      }
      return result;
    } catch (error) {
      if (error instanceof McpToolRoundTerminalError) {
        await this.finish(identity, error.receipt.roundId, "failed");
      }
      throw error;
    }
  }

  private async finish(
    identity: { readonly taskId: string; readonly runId: string; readonly invocationId: string },
    roundId: string,
    expectedStatus: "completed" | "failed"
  ): Promise<void> {
    const terminal = await this.terminals.finishMcpToolInvocationFromRound({ ...identity, roundId });
    if (terminal.invocationId !== identity.invocationId || terminal.status !== expectedStatus) {
      throw new Error("MCP Tool invocation terminal returned conflicting evidence");
    }
  }
}
