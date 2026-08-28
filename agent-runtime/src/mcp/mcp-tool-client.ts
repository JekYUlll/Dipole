import {
  Client,
  isInputRequiredResult,
  specTypeSchemas,
  withInputRequired,
  type CallToolResult,
  type InputRequiredResult,
  type Tool,
  type Transport
} from "@modelcontextprotocol/client";

export interface McpToolEgressPolicy {
  readonly allowedArgumentNames: readonly string[];
  readonly maximumBytes: number;
}

export interface McpToolRoundParams {
  readonly name: string;
  readonly arguments: Readonly<Record<string, unknown>>;
  readonly inputResponses?: Readonly<Record<string, unknown>>;
  readonly requestState?: string;
}

export type McpToolRoundResult = CallToolResult | InputRequiredResult;

export class AllowlistedMcpToolClient {
  readonly #client: Client;
  readonly #allowedTools: ReadonlySet<string>;
  readonly #serverId: string;
  readonly #egressPolicies: ReadonlyMap<string, McpToolEgressPolicy>;
  readonly #requestTimeoutMs: number;
  readonly #manualInputRequiredEnabled: boolean;
  #discovered = new Set<string>();

  constructor(
    serverId: string,
    allowedServerIds: readonly string[],
    allowedTools: readonly string[],
    egressPolicies: Readonly<Record<string, McpToolEgressPolicy>>,
    requestTimeoutMs = 10_000,
    protocolMode: "legacy" | "modern" = "legacy"
  ) {
    const normalizedServerId = serverId.trim();
    if (!allowedServerIds.includes(normalizedServerId)) throw new Error(`MCP Server ${normalizedServerId} is not allowlisted`);
    const normalizedTools = allowedTools.map((name) => name.trim());
    if (normalizedTools.length === 0 || normalizedTools.some((name) => !/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(name)) || new Set(normalizedTools).size !== normalizedTools.length) {
      throw new Error("MCP Tool allowlist is empty, invalid, or duplicated");
    }
    const policyNames = Object.keys(egressPolicies).sort();
    const sortedTools = [...normalizedTools].sort();
    if (policyNames.length !== sortedTools.length || !policyNames.every((name, index) => name === sortedTools[index])) {
      throw new Error("MCP Tool egress policies must exactly match the Tool allowlist");
    }
    this.#egressPolicies = new Map(normalizedTools.map((name) => [name, validateMcpToolEgressPolicy(name, egressPolicies[name])]));
    this.#allowedTools = new Set(normalizedTools);
    this.#serverId = normalizedServerId;
    if (!Number.isSafeInteger(requestTimeoutMs) || requestTimeoutMs < 100 || requestTimeoutMs > 60_000) {
      throw new Error("MCP request timeout must be between 100 and 60000 milliseconds");
    }
    this.#requestTimeoutMs = requestTimeoutMs;
    if (protocolMode !== "legacy" && protocolMode !== "modern") throw new Error("MCP protocol mode is invalid");
    this.#manualInputRequiredEnabled = protocolMode === "modern";
    this.#client = new Client({ name: "dipole-agent", version: "0.1.0" }, protocolMode === "modern" ? {
      capabilities: { elicitation: { form: {} } },
      versionNegotiation: { mode: { pin: "2026-07-28" } },
      inputRequired: { autoFulfill: false, maxRounds: 1 }
    } : undefined);
  }

  async connect(transport: Transport): Promise<readonly Tool[]> {
    const requestOptions = { timeout: this.#requestTimeoutMs, maxTotalTimeout: this.#requestTimeoutMs };
    await this.#client.connect(transport, requestOptions);
    const server = this.#client.getServerVersion();
    if (server?.name !== this.#serverId) {
      await this.#client.close();
      throw new Error(`MCP Server identity mismatch: expected ${this.#serverId}, received ${server?.name ?? "unknown"}`);
    }
    const listed = await this.#client.listTools(undefined, requestOptions);
    if (listed.tools.length > 256) {
      await this.#client.close();
      throw new Error("MCP Server advertised more than 256 Tools");
    }
    this.#discovered = new Set(listed.tools.filter((tool) => this.#allowedTools.has(tool.name)).map((tool) => tool.name));
    return listed.tools.filter((tool) => this.#discovered.has(tool.name));
  }

  async callTool(name: string, arguments_: Readonly<Record<string, unknown>>, signal?: AbortSignal): Promise<CallToolResult> {
    if (!this.#allowedTools.has(name) || !this.#discovered.has(name)) throw new Error(`MCP Tool ${name} is not allowlisted and discovered`);
    const safeArguments = enforceMcpToolEgressPolicy(arguments_, this.#egressPolicies.get(name)!);
    const result = await this.#client.callTool({ name, arguments: safeArguments }, {
      timeout: this.#requestTimeoutMs, maxTotalTimeout: this.#requestTimeoutMs, ...(signal === undefined ? {} : { signal })
    });
    const bytes = new TextEncoder().encode(JSON.stringify(result)).length;
    if (bytes > 128 * 1024) throw new Error(`MCP Tool ${name} response exceeds 128 KiB`);
    if (result.isError === true) throw new Error(`MCP Tool ${name} returned an error`);
    return result;
  }

  async callToolRound(params: McpToolRoundParams, signal?: AbortSignal): Promise<McpToolRoundResult> {
    if (!this.#manualInputRequiredEnabled) throw new Error("MCP manual input_required requires modern protocol mode");
    if (!this.#allowedTools.has(params.name) || !this.#discovered.has(params.name)) {
      throw new Error(`MCP Tool ${params.name} is not allowlisted and discovered`);
    }
    const safeArguments = enforceMcpToolEgressPolicy(params.arguments, this.#egressPolicies.get(params.name)!);
    const continuation = validateRoundContinuation(params.inputResponses, params.requestState);
    // Keep SDK auto-fulfilment disabled: Temporal persists the wait, then a fresh
    // Activity sends the exact continuation parameters produced by the checkpoint.
    const result = await this.#client.request({
      method: "tools/call",
      params: {
        name: params.name,
        arguments: safeArguments,
        ...(continuation.inputResponses === undefined ? {} : { inputResponses: continuation.inputResponses }),
        ...(continuation.requestState === undefined ? {} : { requestState: continuation.requestState })
      }
    }, withInputRequired(specTypeSchemas.CallToolResult), {
      timeout: this.#requestTimeoutMs,
      maxTotalTimeout: this.#requestTimeoutMs,
      allowInputRequired: true,
      ...(signal === undefined ? {} : { signal })
    });
    const bytes = new TextEncoder().encode(JSON.stringify(result)).length;
    if (bytes > 128 * 1024) throw new Error(`MCP Tool ${params.name} response exceeds 128 KiB`);
    if (isInputRequiredResult(result)) {
      if (bytes > 32 * 1024) throw new Error(`MCP Tool ${params.name} input_required response exceeds 32 KiB`);
      return result;
    }
    if (result.isError === true) throw new Error(`MCP Tool ${params.name} returned an error`);
    return result;
  }

  close(): Promise<void> {
    return this.#client.close();
  }
}

const sensitiveArgumentNames = new Set([
  "password", "passwd", "secret", "secretkey", "apikey", "apitoken", "authkey", "authtoken", "token",
  "accesskey", "accesstoken", "refreshtoken", "sessionid", "sessiontoken", "bearertoken",
  "authorization", "cookie", "credential", "credentials", "privatekey", "clientsecret"
]);

export function validateMcpToolEgressPolicy(name: string, policy: McpToolEgressPolicy | undefined): McpToolEgressPolicy {
  if (policy === undefined || !Number.isSafeInteger(policy.maximumBytes) || policy.maximumBytes < 2 || policy.maximumBytes > 64 * 1024) {
    throw new Error(`MCP Tool ${name} egress policy has an invalid byte limit`);
  }
  const allowed = policy.allowedArgumentNames.map(value => value.trim());
  if (allowed.some(value => value.length === 0 || value.length > 128 || sensitiveArgumentNames.has(normalizeFieldName(value))) || new Set(allowed).size !== allowed.length) {
    throw new Error(`MCP Tool ${name} egress policy has invalid argument names`);
  }
  return { allowedArgumentNames: allowed, maximumBytes: policy.maximumBytes };
}

export function enforceMcpToolEgressPolicy(arguments_: Readonly<Record<string, unknown>>, policy: McpToolEgressPolicy): Record<string, unknown> {
  let encoded: string;
  try {
    encoded = JSON.stringify(arguments_);
  } catch {
    throw new Error("MCP Tool egress policy requires JSON arguments");
  }
  if (encoded === undefined || Buffer.byteLength(encoded, "utf8") > policy.maximumBytes) {
    throw new Error("MCP Tool egress policy denied oversized arguments");
  }
  const decoded = JSON.parse(encoded) as unknown;
  if (decoded === null || typeof decoded !== "object" || Array.isArray(decoded)) {
    throw new Error("MCP Tool egress policy requires an argument object");
  }
  const allowed = new Set(policy.allowedArgumentNames);
  if (Object.keys(decoded).some(name => !allowed.has(name))) {
    throw new Error("MCP Tool egress policy denied an undeclared argument");
  }
  rejectSensitiveFields(decoded, 0);
  return decoded as Record<string, unknown>;
}

function validateRoundContinuation(
  inputResponses: Readonly<Record<string, unknown>> | undefined,
  requestState: string | undefined
): { inputResponses?: Record<string, unknown>; requestState?: string } {
  if (requestState !== undefined && inputResponses === undefined) {
    throw new Error("MCP requestState requires inputResponses");
  }
  let safeResponses: Record<string, unknown> | undefined;
  if (inputResponses !== undefined) {
    if (inputResponses === null || typeof inputResponses !== "object" || Array.isArray(inputResponses)) {
      throw new Error("MCP inputResponses must be an object");
    }
    rejectSensitiveFields(inputResponses, 0);
    const encoded = JSON.stringify(inputResponses);
    if (encoded === undefined || Buffer.byteLength(encoded, "utf8") > 16 * 1024) {
      throw new Error("MCP inputResponses exceed 16 KiB");
    }
    safeResponses = JSON.parse(encoded) as Record<string, unknown>;
  }
  if (requestState !== undefined && (typeof requestState !== "string" || Buffer.byteLength(requestState, "utf8") > 8 * 1024)) {
    throw new Error("MCP requestState exceeds 8 KiB");
  }
  return {
    ...(safeResponses === undefined ? {} : { inputResponses: safeResponses }),
    ...(requestState === undefined ? {} : { requestState })
  };
}

function rejectSensitiveFields(value: unknown, depth: number): void {
  if (depth > 16) throw new Error("MCP Tool egress policy denied deeply nested arguments");
  if (Array.isArray(value)) {
    for (const item of value) rejectSensitiveFields(item, depth + 1);
    return;
  }
  if (value === null || typeof value !== "object") return;
  for (const [name, item] of Object.entries(value)) {
    const normalized = normalizeFieldName(name);
    if (sensitiveArgumentNames.has(normalized)) {
      throw new Error("MCP Tool egress policy denied a credential-bearing argument");
    }
    rejectSensitiveFields(item, depth + 1);
  }
}

function normalizeFieldName(value: string): string {
  return value.toLowerCase().replaceAll(/[^a-z0-9]/g, "");
}
