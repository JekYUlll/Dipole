import { describe, expect, it, vi } from "vitest";

import {
  createExternalMcpReadinessPublication,
  type ExternalMcpReadinessPublicationInput
} from "./external-mcp-readiness-publication.js";

describe("external MCP readiness publication", () => {
  it("collects once and publishes one short-lived exact binding", async () => {
    const evidence = readinessEvidence();
    const collect = vi.fn(async () => evidence);
    const publishMcpReadinessEvidence = vi.fn(async () => receipt());
    const publish = createExternalMcpReadinessPublication({ collect, publishMcpReadinessEvidence });
    const input = publicationInput();

    await expect(publish(input)).resolves.toEqual(receipt());

    expect(collect).toHaveBeenCalledOnce();
    expect(collect).toHaveBeenCalledWith({ tenantId: "TENANT-A", profileId: "PROFILE-A" }, expect.any(AbortSignal));
    expect(publishMcpReadinessEvidence).toHaveBeenCalledOnce();
    expect(publishMcpReadinessEvidence).toHaveBeenCalledWith(
      "TENANT-A", evidence, "2026-08-28T14:30:03.000Z", { requestId: "REQ-1", traceId: "TRACE-1" }
    );
  });

  it("fails closed before Core when collection fails or cancellation wins the publication boundary", async () => {
    const publishMcpReadinessEvidence = vi.fn(async () => receipt());
    const failed = createExternalMcpReadinessPublication({
      collect: async () => { throw new Error("token SECRET-VALUE"); }, publishMcpReadinessEvidence
    });
    await expect(failed(publicationInput())).rejects.toThrow("External MCP readiness publication failed");
    expect(publishMcpReadinessEvidence).not.toHaveBeenCalled();

    const controller = new AbortController();
    const cancelled = createExternalMcpReadinessPublication({
      collect: async () => { controller.abort(); return readinessEvidence(); }, publishMcpReadinessEvidence
    });
    await expect(cancelled(publicationInput(), controller.signal)).rejects.toMatchObject({ name: "AbortError" });
    expect(publishMcpReadinessEvidence).not.toHaveBeenCalled();
  });

  it("rejects invalid authority and validity inputs before collection", async () => {
    const collect = vi.fn(async () => readinessEvidence());
    const publish = createExternalMcpReadinessPublication({ collect, publishMcpReadinessEvidence: async () => receipt() });
    for (const input of [
      { ...publicationInput(), tenantId: " TENANT-A" },
      { ...publicationInput(), profileId: "" },
      { ...publicationInput(), requestId: "" },
      { ...publicationInput(), traceId: "T".repeat(129) },
      { ...publicationInput(), validForMs: 3_600_001 }
    ]) {
      await expect(publish(input)).rejects.toThrow("External MCP readiness publication input is invalid");
    }
    expect(collect).not.toHaveBeenCalled();
  });

  it("redacts Publisher failures after the one allowed Core call", async () => {
    const publishMcpReadinessEvidence = vi.fn(async () => { throw new Error("SECRET-VALUE"); });
    const publish = createExternalMcpReadinessPublication({
      collect: async () => readinessEvidence(), publishMcpReadinessEvidence
    });

    await expect(publish(publicationInput())).rejects.toThrow("External MCP readiness publication failed");
    expect(publishMcpReadinessEvidence).toHaveBeenCalledOnce();
  });
});

function publicationInput(): ExternalMcpReadinessPublicationInput {
  return {
    tenantId: "TENANT-A", profileId: "PROFILE-A", validForMs: 30 * 60 * 1_000,
    requestId: "REQ-1", traceId: "TRACE-1"
  };
}

function readinessEvidence() {
  return {
    schemaVersion: "dipole.agent.external-mcp-readiness-evidence.v2" as const,
    bindingSha256: "a".repeat(64), profileBindingSha256: "b".repeat(64),
    startedAt: "2026-08-28T14:00:00.000Z", completedAt: "2026-08-28T14:00:03.000Z",
    preflightCheckedAt: "2026-08-28T14:00:01.000Z", connectivityCheckedAt: "2026-08-28T14:00:02.000Z",
    profileCount: 1, credentialCount: 1, caBundleCount: 1, toolCount: 2
  };
}

function receipt() {
  return {
    evidenceId: "c".repeat(64), profileBindingSha256: "b".repeat(64), runtimeBindingSha256: "a".repeat(64),
    contentSha256: "d".repeat(64), collectedAt: "2026-08-28T14:00:03.000Z",
    expiresAt: "2026-08-28T14:30:03.000Z", created: true
  };
}
