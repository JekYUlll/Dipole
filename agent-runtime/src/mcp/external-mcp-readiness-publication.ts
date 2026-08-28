import type {
  AgentMCPReadinessEvidenceReceipt
} from "../capabilities/agent-capability-rpc.js";
import type {
  ExternalMcpReadinessEvidence,
  ExternalMcpReadinessEvidenceCollector
} from "./external-mcp-readiness-evidence.js";

export interface ExternalMcpReadinessPublicationInput {
  readonly tenantId: string;
  readonly profileId: string;
  readonly validForMs: number;
  readonly requestId: string;
  readonly traceId: string;
}

export interface ExternalMcpReadinessPublicationDependencies {
  readonly collect: ExternalMcpReadinessEvidenceCollector;
  readonly publishMcpReadinessEvidence: (
    tenantId: string,
    evidence: ExternalMcpReadinessEvidence,
    expiresAt: string,
    context: { readonly requestId: string; readonly traceId: string }
  ) => Promise<AgentMCPReadinessEvidenceReceipt>;
}

export type ExternalMcpReadinessPublication = (
  input: ExternalMcpReadinessPublicationInput,
  signal?: AbortSignal
) => Promise<AgentMCPReadinessEvidenceReceipt>;

export function createExternalMcpReadinessPublication(
  dependencies: ExternalMcpReadinessPublicationDependencies
): ExternalMcpReadinessPublication {
  return async (input, signal = new AbortController().signal) => {
    validateExternalMcpReadinessPublicationInput(input);
    signal.throwIfAborted();
    try {
      const evidence = await dependencies.collect({ tenantId: input.tenantId, profileId: input.profileId }, signal);
      signal.throwIfAborted();
      const completedAtMs = Date.parse(evidence.completedAt);
      if (!Number.isFinite(completedAtMs) || new Date(completedAtMs).toISOString() !== evidence.completedAt) {
        throw new Error("invalid completion time");
      }
      const expiresAt = new Date(completedAtMs + input.validForMs).toISOString();
      return await dependencies.publishMcpReadinessEvidence(input.tenantId, evidence, expiresAt, {
        requestId: input.requestId,
        traceId: input.traceId
      });
    } catch {
      if (signal.aborted) signal.throwIfAborted();
      throw new Error("External MCP readiness publication failed");
    }
  };
}

export function validateExternalMcpReadinessPublicationInput(input: ExternalMcpReadinessPublicationInput): void {
  if (!validIdentifier(input.tenantId) || !validIdentifier(input.profileId) ||
      !validProvenance(input.requestId) || !validProvenance(input.traceId) ||
      !Number.isSafeInteger(input.validForMs) || input.validForMs < 60_000 || input.validForMs > 3_600_000) {
    throw new Error("External MCP readiness publication input is invalid");
  }
}

function validIdentifier(value: string): boolean {
  return /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(value);
}

function validProvenance(value: string): boolean {
  return value === value.trim() && value.length > 0 && value.length <= 128 && !/[\r\n]/.test(value);
}
