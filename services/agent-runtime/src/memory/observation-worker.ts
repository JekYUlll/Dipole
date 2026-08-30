import { createHash } from "node:crypto";

import { z } from "zod";

import { observationalCandidateTypeSchema } from "./memory-type-policy.js";

const identifier = z.string().trim().min(1).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9_.:/-]*$/);
const isoDate = z.iso.datetime();
const contentLimit = 16 * 1024;
const safeCandidateContent = (max: number, label: string) => z.string().trim().min(1).max(max).refine(
  (value) => !/(?:password|passwd|token|secret|authorization|bearer|api[_ -]?key)\s*[:=]/i.test(value),
  { message: `memory candidate ${label} contains a credential pattern` },
);
const candidateSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-candidate.v1"),
  memoryId: z.string().regex(/^(?:OBS|REF)-[a-f0-9]{64}$/),
  tenantId: identifier,
  principalId: identifier,
  agentId: identifier,
  memoryType: observationalCandidateTypeSchema,
  resourceType: identifier,
  resourceId: identifier,
  content: safeCandidateContent(contentLimit, "content"),
  compactContent: safeCandidateContent(4096, "summary"),
  priority: z.number().int().min(0).max(1000),
  provenance: z.object({
    sourceType: z.enum(["message", "reflection"]),
    sourceId: identifier,
    sequence: identifier.optional(),
  }).strict(),
  observedAt: isoDate,
}).strict();

const observationInputSchema = z.object({
  tenantId: identifier,
  principalId: identifier,
  agentId: identifier,
  resourceType: identifier,
  resourceId: identifier,
  eventId: identifier,
  messageId: identifier,
  messageSequence: identifier,
  senderId: identifier,
  occurredAt: isoDate,
  content: z.string().trim().min(1).max(contentLimit),
}).strict();

export type MemoryObservationInput = z.infer<typeof observationInputSchema>;
export type MemoryCandidate = z.infer<typeof candidateSchema>;

export interface ReflectionInput extends MemoryObservationInput {
  readonly windowId: string;
  readonly observations: readonly MemoryCandidate[];
}

export function parseMemoryCandidate(value: unknown): MemoryCandidate {
  return candidateSchema.parse(value);
}

export class ObservationWorker {
  readonly #seenEvents = new Set<string>();

  observe(raw: MemoryObservationInput): MemoryCandidate[] {
    const parsed = observationInputSchema.safeParse(raw);
    if (!parsed.success) return [];
    const input = parsed.data;
    const eventKey = scopeKey(input, input.eventId);
    if (this.#seenEvents.has(eventKey)) return [];

    const content = safeObservationContent(input.content);
    if (content === undefined) {
      this.#seenEvents.add(eventKey);
      return [];
    }

    this.#seenEvents.add(eventKey);
    const canonical = [input.tenantId, input.principalId, input.agentId, input.resourceId, input.eventId, input.messageId, input.messageSequence, content].join("\n");
    const memoryId = `OBS-${sha256(canonical)}`;
    const compactContent = compact(content);
    return [parseMemoryCandidate({
      schemaVersion: "dipole.agent.memory-candidate.v1",
      memoryId,
      tenantId: input.tenantId,
      principalId: input.principalId,
      agentId: input.agentId,
      memoryType: "observational",
      resourceType: input.resourceType,
      resourceId: input.resourceId,
      content,
      compactContent,
      priority: priorityFor(content),
      provenance: { sourceType: "message", sourceId: input.messageId, sequence: input.messageSequence },
      observedAt: input.occurredAt,
    })];
  }
}

export class ReflectionWorker {
  readonly #minimumObservations: number;
  readonly #reflectedWindows = new Set<string>();

  constructor(options: { readonly minimumObservations?: number } = {}) {
    this.#minimumObservations = options.minimumObservations ?? 2;
    if (!Number.isSafeInteger(this.#minimumObservations) || this.#minimumObservations < 2 || this.#minimumObservations > 100) {
      throw new Error("Reflection minimum observations is invalid");
    }
  }

  reflect(raw: ReflectionInput): MemoryCandidate | undefined {
    const parsed = observationInputSchema.safeParse({
      tenantId: raw.tenantId, principalId: raw.principalId, agentId: raw.agentId,
      resourceType: raw.resourceType, resourceId: raw.resourceId, eventId: raw.eventId,
      messageId: raw.messageId, messageSequence: raw.messageSequence, senderId: raw.senderId,
      occurredAt: raw.occurredAt, content: raw.content,
    });
    if (!parsed.success) return undefined;
    const input = parsed.data;
    const windowId = identifier.parse(raw.windowId);
    const windowKey = scopeKey(input, windowId);
    if (this.#reflectedWindows.has(windowKey)) return undefined;
    const observations = raw.observations.map(parseMemoryCandidate);
    const ids = new Set(observations.map((item) => item.memoryId));
    if (observations.length < this.#minimumObservations || ids.size !== observations.length) return undefined;
    if (observations.some((item) => item.tenantId !== input.tenantId || item.principalId !== input.principalId || item.agentId !== input.agentId || item.resourceId !== input.resourceId)) return undefined;

    const lines = observations.map((item) => item.content).join(" ");
    const canonical = [input.tenantId, input.principalId, input.agentId, input.resourceId, windowId, ...[...ids].sort()].join("\n");
    this.#reflectedWindows.add(windowKey);
    return parseMemoryCandidate({
      schemaVersion: "dipole.agent.memory-candidate.v1",
      memoryId: `REF-${sha256(canonical)}`,
      tenantId: input.tenantId,
      principalId: input.principalId,
      agentId: input.agentId,
      memoryType: "observational",
      resourceType: input.resourceType,
      resourceId: input.resourceId,
      content: `Reflection from ${observations.length} observations: ${lines}`.slice(0, contentLimit),
      compactContent: `${observations.length} observations: ${observations.map((item) => item.compactContent).join(" | ")}`.slice(0, 4096),
      priority: Math.min(1000, Math.max(...observations.map((item) => item.priority)) + 10),
      provenance: { sourceType: "reflection", sourceId: windowId },
      observedAt: input.occurredAt,
    });
  }
}

function safeObservationContent(value: string): string | undefined {
  const content = value.trim();
  if (content.length === 0 || content.length > contentLimit || /(?:password|passwd|token|secret|authorization|bearer|api[_ -]?key)\s*[:=]/i.test(content)) return undefined;
  const selected = content.split(/[。！？!?\n]+/u).map((part) => part.trim()).filter((part) => /决定|确定|负责|截止|完成|风险|延期|阻塞|事故/u.test(part));
  if (selected.length === 0) return undefined;
  return selected.join("。 ").slice(0, contentLimit);
}

function compact(content: string): string {
  return content.length <= 512 ? content : `${content.slice(0, 509)}...`;
}

function priorityFor(content: string): number {
  if (/风险|延期|阻塞|事故/u.test(content)) return 90;
  if (/截止|负责|完成/u.test(content)) return 80;
  return 60;
}

function scopeKey(input: Pick<MemoryObservationInput, "tenantId" | "principalId" | "agentId" | "resourceType" | "resourceId">, id: string): string {
  return [input.tenantId, input.principalId, input.agentId, input.resourceType, input.resourceId, id].join("\n");
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}
