import type { ExecutionContext } from "../runtime/execution-context.js";
import { PolicyEngine, type CapabilityDescriptor, type ResourceRequest } from "../policy/policy-engine.js";

export interface InputSchema<I> {
  parse(input: unknown): I;
}

export interface AgentCapability<I, O> {
  readonly descriptor: CapabilityDescriptor;
  readonly inputSchema: InputSchema<I>;
  resolveResource(input: I, context: ExecutionContext): ResourceRequest;
  execute(input: I, context: ExecutionContext): Promise<O>;
}

export interface PreparedCapabilityInvocation {
  readonly descriptor: CapabilityDescriptor;
  readonly resource: ResourceRequest;
  readonly input: unknown;
  execute(): Promise<unknown>;
}

export class CapabilityRegistry {
  readonly #capabilities = new Map<string, AgentCapability<unknown, unknown>>();

  constructor(private readonly policy = new PolicyEngine()) {}

  register<I, O>(capability: AgentCapability<I, O>): void {
    const id = capability.descriptor.id.trim();
    if (!id) {
      throw new Error("capability ID is required");
    }
    if (this.#capabilities.has(id)) {
      throw new Error(`capability ${id} is already registered`);
    }
    validateInputSchemaDescriptor(capability.descriptor.inputSchema);
    this.#capabilities.set(id, capability as AgentCapability<unknown, unknown>);
  }

  async execute(id: string, rawInput: unknown, context: ExecutionContext): Promise<unknown> {
    return this.prepare(id, rawInput, context).execute();
  }

  prepare(id: string, rawInput: unknown, context: ExecutionContext): PreparedCapabilityInvocation {
    const capability = this.#capabilities.get(id.trim());
    if (capability === undefined) {
      throw new Error(`capability ${id} is not registered`);
    }
    const input = capability.inputSchema.parse(rawInput);
    const resource = capability.resolveResource(input, context);
    this.policy.authorize(context, capability.descriptor, resource);
    return {
      descriptor: capability.descriptor,
      resource,
      input,
      execute: () => capability.execute(input, context)
    };
  }

  descriptors(): readonly CapabilityDescriptor[] {
    return [...this.#capabilities.values()].map((capability) => capability.descriptor);
  }
}

const inputSchemaKeys = new Set([
  "type", "properties", "required", "additionalProperties", "minimum", "maximum", "minLength", "maxLength", "default", "enum"
]);
const inputSchemaMaxBytes = 4 * 1024;

function validateInputSchemaDescriptor(value: Readonly<Record<string, unknown>> | undefined): void {
  if (value === undefined) return;
  let serialized: string;
  try {
    serialized = JSON.stringify(value);
  } catch {
    throw new Error("capability input schema descriptor is invalid");
  }
  if (Buffer.byteLength(serialized, "utf8") > inputSchemaMaxBytes) {
    throw new Error("capability input schema descriptor is too large");
  }
  visitInputSchemaValue(value, 0, false);
}

function visitInputSchemaValue(value: unknown, depth: number, propertyMap: boolean): void {
  if (depth > 8 || value === null) return;
  if (Array.isArray(value)) {
    for (const item of value) visitInputSchemaValue(item, depth + 1, false);
    return;
  }
  if (typeof value !== "object") return;
  for (const [key, item] of Object.entries(value)) {
    if (!propertyMap && !inputSchemaKeys.has(key)) throw new Error(`capability input schema key ${key} is not allowed`);
    visitInputSchemaValue(item, depth + 1, !propertyMap && key === "properties");
  }
}
