export const agentElicitationSchemaVersion = "dipole.agent.elicitation.v1" as const;

export type AgentElicitationSource =
  | { kind: "agent" }
  | { kind: "mcp"; serverId: string; toolName: string; invocationId: string; trust: "untrusted" };

type ElicitationFieldBase = { id: string; label: string; required: boolean };
export type AgentElicitationField =
  | (ElicitationFieldBase & { type: "text"; maxLength?: number })
  | (ElicitationFieldBase & { type: "select"; options: string[] })
  | (ElicitationFieldBase & { type: "multiselect"; options: string[]; maxSelections?: number })
  | (ElicitationFieldBase & { type: "boolean" });

export interface AgentElicitationForm {
  schemaVersion: typeof agentElicitationSchemaVersion;
  fields: AgentElicitationField[];
}

export type AgentElicitationValue = Readonly<Record<string, string | boolean | readonly string[]>>;

const identifier = /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/;
const sourceIdentifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/;
const sensitiveFieldNames = new Set([
  "password", "passwd", "secret", "secretkey", "token", "apikey", "apitoken", "authkey", "authtoken",
  "accesskey", "accesstoken", "refreshtoken", "sessionid", "sessiontoken", "bearertoken", "authorization",
  "cookie", "credential", "credentials", "privatekey", "clientsecret", "payment", "creditcard"
]);

export function validateElicitationSource(raw: unknown): AgentElicitationSource {
  if (!isRecord(raw)) throw new Error("Agent Elicitation source is invalid");
  if (raw.kind === "agent") {
    rejectUnknownKeys(raw, ["kind"], "Agent Elicitation source");
    return { kind: "agent" };
  }
  if (raw.kind !== "mcp" || raw.trust !== "untrusted" ||
      typeof raw.serverId !== "string" || !sourceIdentifier.test(raw.serverId) ||
      typeof raw.toolName !== "string" || !sourceIdentifier.test(raw.toolName) ||
      typeof raw.invocationId !== "string" || !sourceIdentifier.test(raw.invocationId)) {
    throw new Error("Agent Elicitation MCP source is invalid");
  }
  rejectUnknownKeys(raw, ["kind", "serverId", "toolName", "invocationId", "trust"], "Agent Elicitation MCP source");
  return {
    kind: "mcp", serverId: raw.serverId, toolName: raw.toolName,
    invocationId: raw.invocationId, trust: "untrusted"
  };
}

export function validateElicitationForm(raw: unknown): AgentElicitationForm {
  if (!isRecord(raw) || raw.schemaVersion !== agentElicitationSchemaVersion || !Array.isArray(raw.fields) || raw.fields.length < 1 || raw.fields.length > 16) {
    throw new Error("Agent Elicitation form must use v1 with 1-16 fields");
  }
  rejectUnknownKeys(raw, ["schemaVersion", "fields"], "Agent Elicitation form");
  const fields = raw.fields.map(validateField);
  if (new Set(fields.map((field) => field.id)).size !== fields.length) {
    throw new Error("Agent Elicitation field IDs must be unique");
  }
  if (byteLength(raw) > 16 * 1024) throw new Error("Agent Elicitation form exceeds 16 KiB");
  return { schemaVersion: agentElicitationSchemaVersion, fields };
}

export function validateElicitationResponse(form: AgentElicitationForm, raw: unknown): AgentElicitationValue {
  const validated = validateElicitationForm(form);
  if (!isRecord(raw) || byteLength(raw) > 16 * 1024) throw new Error("Agent Elicitation response must be an object within 16 KiB");
  const fields = new Map(validated.fields.map((field) => [field.id, field]));
  for (const key of Object.keys(raw)) {
    if (!fields.has(key)) throw new Error(`Agent Elicitation response contains unknown field ${key}`);
  }
  const result: Record<string, string | boolean | readonly string[]> = {};
  for (const field of validated.fields) {
    const value = raw[field.id];
    if (value === undefined) {
      if (field.required) throw new Error(`Agent Elicitation field ${field.id} is required`);
      continue;
    }
    switch (field.type) {
      case "text":
        if (typeof value !== "string" || value.length > (field.maxLength ?? 1000)) throw new Error(`Agent Elicitation field ${field.id} is invalid`);
        result[field.id] = value;
        break;
      case "select":
        if (typeof value !== "string" || !field.options.includes(value)) throw new Error(`Agent Elicitation field ${field.id} has an invalid option`);
        result[field.id] = value;
        break;
      case "multiselect":
        if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || !field.options.includes(item)) ||
            new Set(value).size !== value.length || value.length > (field.maxSelections ?? field.options.length)) {
          throw new Error(`Agent Elicitation field ${field.id} has invalid selections`);
        }
        result[field.id] = [...value] as string[];
        break;
      case "boolean":
        if (typeof value !== "boolean") throw new Error(`Agent Elicitation field ${field.id} must be boolean`);
        result[field.id] = value;
        break;
    }
  }
  return result;
}

function validateField(raw: unknown): AgentElicitationField {
  if (!isRecord(raw) || typeof raw.id !== "string" || !identifier.test(raw.id) || typeof raw.label !== "string" || raw.label.trim().length < 1 || raw.label.length > 256 || typeof raw.required !== "boolean") {
    throw new Error("Agent Elicitation field identity is invalid");
  }
  if (isSensitiveField(raw.id) || isSensitiveField(raw.label)) {
    throw new Error(`Agent Elicitation field ${raw.id} is sensitive`);
  }
  const base = { id: raw.id, label: raw.label, required: raw.required };
  switch (raw.type) {
    case "text": {
      rejectUnknownKeys(raw, ["id", "label", "type", "required", "maxLength"], `Agent Elicitation field ${raw.id}`);
      const maxLength = raw.maxLength;
      if (maxLength !== undefined && (!Number.isInteger(maxLength) || (maxLength as number) < 1 || (maxLength as number) > 4000)) throw new Error(`Agent Elicitation text field ${raw.id} maxLength is invalid`);
      return { ...base, type: "text", ...(maxLength === undefined ? {} : { maxLength: maxLength as number }) };
    }
    case "select":
      rejectUnknownKeys(raw, ["id", "label", "type", "required", "options"], `Agent Elicitation field ${raw.id}`);
      return { ...base, type: "select", options: validateOptions(raw.id, raw.options) };
    case "multiselect": {
      rejectUnknownKeys(raw, ["id", "label", "type", "required", "options", "maxSelections"], `Agent Elicitation field ${raw.id}`);
      const options = validateOptions(raw.id, raw.options);
      const maxSelections = raw.maxSelections;
      if (maxSelections !== undefined && (!Number.isInteger(maxSelections) || (maxSelections as number) < 1 || (maxSelections as number) > options.length)) throw new Error(`Agent Elicitation multiselect field ${raw.id} maxSelections is invalid`);
      return { ...base, type: "multiselect", options, ...(maxSelections === undefined ? {} : { maxSelections: maxSelections as number }) };
    }
    case "boolean":
      rejectUnknownKeys(raw, ["id", "label", "type", "required"], `Agent Elicitation field ${raw.id}`);
      return { ...base, type: "boolean" };
    default:
      throw new Error(`Agent Elicitation field ${raw.id} type is invalid`);
  }
}

function validateOptions(id: string, raw: unknown): string[] {
  if (!Array.isArray(raw) || raw.length < 1 || raw.length > 32 || raw.some((item) => typeof item !== "string" || item.trim().length < 1 || item.length > 128) || new Set(raw).size !== raw.length) {
    throw new Error(`Agent Elicitation field ${id} options are invalid`);
  }
  return [...raw] as string[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function byteLength(value: unknown): number {
  return new TextEncoder().encode(JSON.stringify(value)).length;
}

function rejectUnknownKeys(value: Record<string, unknown>, allowed: readonly string[], label: string): void {
  const unknown = Object.keys(value).find((key) => !allowed.includes(key));
  if (unknown !== undefined) throw new Error(`${label} contains unknown field ${unknown}`);
}

function isSensitiveField(value: string): boolean {
  return sensitiveFieldNames.has(value.toLowerCase().replace(/[^a-z0-9]/g, ""));
}
