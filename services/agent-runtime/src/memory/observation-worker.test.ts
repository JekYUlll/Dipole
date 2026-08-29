import { describe, expect, it } from "vitest";

import {
  ObservationWorker,
  ReflectionWorker,
  parseMemoryCandidate,
  type MemoryObservationInput,
} from "./observation-worker.js";

const input = (content: string, eventId = "EV-1"): MemoryObservationInput => ({
  tenantId: "dipole",
  principalId: "U100",
  agentId: "UAI",
  resourceType: "conversation",
  resourceId: "group:G1",
  eventId,
  messageId: "M-1",
  messageSequence: "42",
  senderId: "U200",
  occurredAt: "2026-08-29T00:00:00.000Z",
  content,
});

describe("ObservationWorker", () => {
  it("emits a bounded observational candidate without writing it", () => {
    const worker = new ObservationWorker();
    const candidates = worker.observe(input("决定：周五前完成 API v2，风险是数据库迁移可能延期。"));

    expect(candidates).toHaveLength(1);
    expect(candidates[0]).toMatchObject({
      memoryType: "observational",
      resourceId: "group:G1",
      provenance: { sourceType: "message", sourceId: "M-1", sequence: "42" },
    });
    expect(candidates[0]?.content).toContain("决定");
    expect(candidates[0]?.content).not.toContain("U100");
    expect(() => parseMemoryCandidate(candidates[0])).not.toThrow();
  });

  it("is deterministic and deduplicates repeated event input", () => {
    const worker = new ObservationWorker();
    const first = worker.observe(input("Alice 负责周五前完成 API v2。"));
    const second = worker.observe(input("Alice 负责周五前完成 API v2。"));

    expect(second).toEqual([]);
    expect(first[0]?.memoryId).toMatch(/^OBS-[a-f0-9]{64}$/);
  });

  it("keeps same event IDs independent across scopes", () => {
    const worker = new ObservationWorker();
    const first = worker.observe(input("决定：API v2 周五完成。", "EV-SHARED"));
    const second = worker.observe({ ...input("决定：移动端周五完成。", "EV-SHARED"), tenantId: "tenant-two" });

    expect(first).toHaveLength(1);
    expect(second).toHaveLength(1);
    expect(second[0]?.memoryId).not.toBe(first[0]?.memoryId);
  });

  it("fails closed for credentials and oversized content", () => {
    const worker = new ObservationWorker();
    expect(worker.observe(input("token=secret password=hunter2"))).toEqual([]);
    expect(worker.observe(input("决定：" + "x".repeat(20_000)))).toEqual([]);
  });
});

describe("ReflectionWorker", () => {
  it("requires a stable evidence window and emits a compact reflection", () => {
    const worker = new ReflectionWorker({ minimumObservations: 2 });
    const observations = [
      ...new ObservationWorker().observe(input("决定：API v2 周五完成。", "EV-1")),
      ...new ObservationWorker().observe(input("风险：数据库迁移可能延期。", "EV-2")),
    ];
    const reflection = worker.reflect({
      ...input("ignored", "EV-REFLECT"),
      windowId: "WIN-1",
      observations,
    });

    expect(reflection).toMatchObject({
      memoryType: "observational",
      provenance: { sourceType: "reflection", sourceId: "WIN-1" },
      compactContent: expect.stringContaining("2 observations"),
    });
    expect(reflection?.content).toContain("决定");
    expect(reflection?.content).toContain("风险");
  });

  it("does not reflect an incomplete or repeated window", () => {
    const observations = new ObservationWorker().observe(input("决定：API v2 周五完成。"));
    const worker = new ReflectionWorker({ minimumObservations: 2 });
    expect(worker.reflect({ ...input("ignored", "EV-REFLECT"), windowId: "WIN-1", observations })).toBeUndefined();
    expect(worker.reflect({ ...input("ignored", "EV-REFLECT"), windowId: "WIN-1", observations: [...observations, ...observations] })).toBeUndefined();
  });

  it("keeps the same window ID independent across resources", () => {
    const observationWorker = new ObservationWorker();
    const observations = [
      ...observationWorker.observe(input("决定：API v2 周五完成。", "EV-1")),
      ...observationWorker.observe(input("风险：数据库迁移可能延期。", "EV-2")),
    ];
    const worker = new ReflectionWorker({ minimumObservations: 2 });
    const first = worker.reflect({ ...input("ignored", "EV-REFLECT-1"), windowId: "WIN-SHARED", observations });
    const second = worker.reflect({
      ...input("ignored", "EV-REFLECT-2"),
      windowId: "WIN-SHARED",
      resourceId: "group:G2",
      observations: observations.map((item) => ({ ...item, resourceId: "group:G2" })),
    });

    expect(first).toBeDefined();
    expect(second).toBeDefined();
  });
});
