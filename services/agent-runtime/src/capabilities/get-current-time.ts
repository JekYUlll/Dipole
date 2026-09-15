import { z } from "zod";

import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({ timeZone: z.string().trim().min(1).max(64).optional() }).strict();
type CurrentTimeInput = z.infer<typeof inputSchema>;

export class GetCurrentTimeCapability implements AgentCapability<CurrentTimeInput, unknown> {
  readonly descriptor = {
    id: "time.now", risk: "read" as const, requiredPermission: "time.read",
    inputSchema: { type: "object", properties: { timeZone: { type: "string", minLength: 1, maxLength: 64 } }, additionalProperties: false }
  };
  readonly inputSchema = inputSchema;

  constructor(private readonly now: () => Date = () => new Date()) {}

  resolveResource() { return { resourceType: "time", resourceId: "*", action: "read" }; }

  async execute(input: CurrentTimeInput): Promise<unknown> {
    const timeZone = canonicalTimeZone(input.timeZone);
    const now = this.now();
    if (!Number.isFinite(now.getTime())) throw new Error("Clock returned an invalid time");
    return { iso: now.toISOString(), unixMs: now.getTime(), timeZone };
  }
}

function canonicalTimeZone(value: string | undefined): string {
  try {
    return new Intl.DateTimeFormat("en-US", { timeZone: value ?? "UTC" }).resolvedOptions().timeZone;
  } catch {
    throw new Error("Time zone must be a valid IANA identifier");
  }
}
