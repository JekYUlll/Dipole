import { createHash } from "node:crypto";
import type { ElicitResult } from "@modelcontextprotocol/client";

import type { AgentTaskDirective } from "../temporal/agent-task-activities.js";
import type { AgentTaskResume } from "../task/agent-task-state.js";
import {
  agentElicitationSchemaVersion,
  validateElicitationForm,
  validateElicitationResponse,
  type AgentElicitationField,
  type AgentElicitationForm
} from "../task/agent-elicitation.js";

const checkpointSchemaVersion = "dipole.mcp.elicitation-checkpoint.v1" as const;
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/;
const fieldIdentifier = /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/;
const sensitiveFieldNames = new Set([
  "password", "passwd", "secret", "secretkey", "token", "apikey", "apitoken", "authkey", "authtoken",
  "accesskey", "accesstoken", "refreshtoken", "sessionid", "sessiontoken", "bearertoken", "authorization",
  "cookie", "credential", "credentials", "privatekey", "clientsecret"
]);
const maximumWaitMs = 24 * 60 * 60 * 1000;

export interface McpElicitationCheckpointV1 {
  readonly schemaVersion: typeof checkpointSchemaVersion;
  readonly requestId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly invocationId: string;
  readonly trust: "untrusted";
  readonly prompt: string;
  readonly form: AgentElicitationForm;
  readonly expiresAtUnixMs: number;
  readonly bindingSha256: string;
}

export type McpElicitationResultInput =
  | { readonly action: "accept"; readonly resume: AgentTaskResume }
  | { readonly action: "decline" | "cancel"; readonly requestId: string };

export class McpDurableElicitationAdapter {
  constructor(private readonly now: () => number = Date.now) {}

  request(input: {
    readonly request: unknown;
    readonly requestId: string;
    readonly serverId: string;
    readonly toolName: string;
    readonly invocationId: string;
    readonly expiresAtUnixMs: number;
  }): { directive: Extract<AgentTaskDirective, { kind: "wait_input" }>; checkpoint: McpElicitationCheckpointV1 } {
    validateIdentity(input.requestId, "request");
    validateIdentity(input.serverId, "Server");
    validateIdentity(input.toolName, "Tool");
    validateIdentity(input.invocationId, "invocation");
    const now = this.now();
    if (!Number.isSafeInteger(input.expiresAtUnixMs) || input.expiresAtUnixMs <= now || input.expiresAtUnixMs > now + maximumWaitMs) {
      throw new Error("MCP Elicitation deadline is invalid");
    }
    const parsed = parseFormRequest(input.request);
    const binding = {
      schemaVersion: checkpointSchemaVersion,
      requestId: input.requestId,
      serverId: input.serverId,
      toolName: input.toolName,
      invocationId: input.invocationId,
      trust: "untrusted" as const,
      prompt: parsed.prompt,
      form: parsed.form,
      expiresAtUnixMs: input.expiresAtUnixMs
    };
    const checkpoint: McpElicitationCheckpointV1 = { ...binding, bindingSha256: sha256(canonicalJSON(binding)) };
    return {
      checkpoint,
      directive: {
        kind: "wait_input", requestId: input.requestId, prompt: parsed.prompt, form: parsed.form,
        source: {
          kind: "mcp", serverId: input.serverId, toolName: input.toolName,
          invocationId: input.invocationId, trust: "untrusted"
        },
        expiresAtUnixMs: input.expiresAtUnixMs, checkpoint
      }
    };
  }

  result(checkpoint: McpElicitationCheckpointV1, input: McpElicitationResultInput): ElicitResult {
    validateCheckpoint(checkpoint);
    if (checkpoint.expiresAtUnixMs <= this.now()) throw new Error("MCP Elicitation checkpoint has expired");
    const requestId = input.action === "accept" ? input.resume.requestId : input.requestId;
    if (requestId !== checkpoint.requestId) throw new Error("MCP Elicitation resume binding does not match the checkpoint");
    if (input.action !== "accept") return { action: input.action };
    if (input.resume.kind !== "input") throw new Error("MCP Elicitation resume binding requires input");
    const value = validateElicitationResponse(checkpoint.form, input.resume.value);
    const content: Record<string, string | boolean | string[]> = {};
    for (const [key, item] of Object.entries(value)) content[key] = Array.isArray(item) ? [...item] : item as string | boolean;
    return { action: "accept", content };
  }
}

function parseFormRequest(raw: unknown): { prompt: string; form: AgentElicitationForm } {
  if (!isRecord(raw) || raw.method !== "elicitation/create") throw new Error("MCP Elicitation request is invalid");
  rejectUnknownKeys(raw, ["method", "params"], "MCP Elicitation request");
  if (!isRecord(raw.params) || (raw.params.mode !== undefined && raw.params.mode !== "form")) {
    throw new Error("MCP Elicitation supports form mode only");
  }
  rejectUnknownKeys(raw.params, ["mode", "message", "requestedSchema"], "MCP Elicitation params");
  if (typeof raw.params.message !== "string" || raw.params.message.trim().length === 0 || raw.params.message.length > 2000) {
    throw new Error("MCP Elicitation message is invalid");
  }
  if (!isRecord(raw.params.requestedSchema)) throw new Error("MCP Elicitation requested schema is invalid");
  const schema = raw.params.requestedSchema;
  rejectUnknownKeys(schema, ["type", "properties", "required"], "MCP Elicitation requested schema");
  if (schema.type !== "object" || !isRecord(schema.properties)) throw new Error("MCP Elicitation requested schema must be an object");
  const entries = Object.entries(schema.properties);
  if (entries.length < 1 || entries.length > 16) throw new Error("MCP Elicitation requested schema must contain 1-16 fields");
  const required = parseRequired(schema.required, entries.map(([id]) => id));
  const fields = entries.map(([id, property]) => parseField(id, property, required.has(id)));
  const form = validateElicitationForm({ schemaVersion: agentElicitationSchemaVersion, fields });
  if (Buffer.byteLength(JSON.stringify(raw), "utf8") > 16 * 1024) throw new Error("MCP Elicitation request exceeds 16 KiB");
  return { prompt: raw.params.message, form };
}

function parseField(id: string, raw: unknown, required: boolean): AgentElicitationField {
  if (!fieldIdentifier.test(id) || sensitiveFieldNames.has(normalizeFieldName(id)) || !isRecord(raw)) {
    throw new Error(`MCP Elicitation field ${id} is unsupported or sensitive`);
  }
  const label = raw.title === undefined ? id : raw.title;
  if (typeof label !== "string") throw new Error(`MCP Elicitation field ${id} title is invalid`);
  if (raw.type === "boolean") {
    rejectUnknownKeys(raw, ["type", "title"], `MCP Elicitation field ${id}`);
    return { id, label, type: "boolean", required };
  }
  if (raw.type === "string" && Array.isArray(raw.enum)) {
    rejectUnknownKeys(raw, ["type", "title", "enum"], `MCP Elicitation field ${id}`);
    return { id, label, type: "select", required, options: parseOptions(id, raw.enum) };
  }
  if (raw.type === "string") {
    rejectUnknownKeys(raw, ["type", "title", "maxLength"], `MCP Elicitation field ${id}`);
    const maxLength = raw.maxLength;
    if (maxLength !== undefined && (!Number.isSafeInteger(maxLength) || (maxLength as number) < 1 || (maxLength as number) > 4000)) {
      throw new Error(`MCP Elicitation field ${id} maxLength is invalid`);
    }
    return { id, label, type: "text", required, ...(maxLength === undefined ? {} : { maxLength: maxLength as number }) };
  }
  if (raw.type === "array" && isRecord(raw.items)) {
    rejectUnknownKeys(raw, ["type", "title", "items", "maxItems"], `MCP Elicitation field ${id}`);
    rejectUnknownKeys(raw.items, ["type", "enum"], `MCP Elicitation field ${id} items`);
    if (raw.items.type !== "string" || !Array.isArray(raw.items.enum)) throw new Error(`MCP Elicitation field ${id} is unsupported`);
    const options = parseOptions(id, raw.items.enum);
    const maxSelections = raw.maxItems;
    if (maxSelections !== undefined && (!Number.isSafeInteger(maxSelections) || (maxSelections as number) < 1 || (maxSelections as number) > options.length)) {
      throw new Error(`MCP Elicitation field ${id} maxItems is invalid`);
    }
    return { id, label, type: "multiselect", required, options, ...(maxSelections === undefined ? {} : { maxSelections: maxSelections as number }) };
  }
  throw new Error(`MCP Elicitation field ${id} is unsupported`);
}

function parseRequired(raw: unknown, fieldIds: readonly string[]): Set<string> {
  if (raw === undefined) return new Set();
  if (!Array.isArray(raw) || raw.some(item => typeof item !== "string") || new Set(raw).size !== raw.length || raw.some(item => !fieldIds.includes(item as string))) {
    throw new Error("MCP Elicitation required fields are invalid");
  }
  return new Set(raw as string[]);
}

function parseOptions(id: string, raw: readonly unknown[]): string[] {
  if (raw.length < 1 || raw.length > 32 || raw.some(item => typeof item !== "string" || item.trim().length === 0 || item.length > 128) || new Set(raw).size !== raw.length) {
    throw new Error(`MCP Elicitation field ${id} options are invalid`);
  }
  return [...raw] as string[];
}

function validateCheckpoint(checkpoint: McpElicitationCheckpointV1): void {
  const { bindingSha256, ...binding } = checkpoint;
  validateIdentity(checkpoint.requestId, "request");
  validateIdentity(checkpoint.serverId, "Server");
  validateIdentity(checkpoint.toolName, "Tool");
  validateIdentity(checkpoint.invocationId, "invocation");
  if (checkpoint.schemaVersion !== checkpointSchemaVersion || checkpoint.trust !== "untrusted" || bindingSha256 !== sha256(canonicalJSON(binding))) {
    throw new Error("MCP Elicitation checkpoint integrity validation failed");
  }
  validateElicitationForm(checkpoint.form);
}

function validateIdentity(value: string, label: string): void {
  if (!identifier.test(value)) throw new Error(`MCP Elicitation ${label} identity is invalid`);
}

function normalizeFieldName(value: string): string {
  return value.toLowerCase().replaceAll(/[^a-z0-9]/g, "");
}

function rejectUnknownKeys(value: Record<string, unknown>, allowed: readonly string[], label: string): void {
  const unknown = Object.keys(value).find(key => !allowed.includes(key));
  if (unknown !== undefined) throw new Error(`${label} contains unsupported field ${unknown}`);
}

function canonicalJSON(value: unknown): string {
  if (value === null || typeof value === "string" || typeof value === "boolean" || typeof value === "number") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (!isRecord(value)) throw new Error("MCP Elicitation binding is not JSON serializable");
  return `{${Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
