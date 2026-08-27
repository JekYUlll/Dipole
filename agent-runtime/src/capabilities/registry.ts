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
    this.#capabilities.set(id, capability as AgentCapability<unknown, unknown>);
  }

  async execute(id: string, rawInput: unknown, context: ExecutionContext): Promise<unknown> {
    const capability = this.#capabilities.get(id.trim());
    if (capability === undefined) {
      throw new Error(`capability ${id} is not registered`);
    }
    const input = capability.inputSchema.parse(rawInput);
    const resource = capability.resolveResource(input, context);
    this.policy.authorize(context, capability.descriptor, resource);
    return capability.execute(input, context);
  }

  descriptors(): readonly CapabilityDescriptor[] {
    return [...this.#capabilities.values()].map((capability) => capability.descriptor);
  }
}
