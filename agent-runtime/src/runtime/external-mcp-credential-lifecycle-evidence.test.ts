import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import {
  createExternalMcpCredentialLifecycleEvidence,
  externalMcpCredentialLifecycleEvidenceSchemaVersion,
  parseExternalMcpCredentialLifecycleEvidence
} from "./external-mcp-credential-lifecycle-evidence.js";

const outcome = {
  initial_credential_verified: true,
  rotated_credential_verified: true,
  old_version_revoked_before_transport: true,
  restart_recovered: true,
  active_version_revoked_before_transport: true,
  transport_open_count: 3,
  transport_close_count: 3,
  inflight_revocation_authority: false
} as const;

describe("external MCP credential lifecycle evidence", () => {
  it("keeps the language-neutral schema aligned with the strict Runtime contract", async () => {
    const path = new URL("../../../contracts/agent-external-mcp/v1/credential-lifecycle-drill-evidence.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string; "x-dipole-version": string; additionalProperties: boolean;
      required: string[]; properties: Record<string, { const?: unknown }>;
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/credential-lifecycle-drill-evidence\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpCredentialLifecycleEvidenceSchemaVersion);
    expect(schema.additionalProperties).toBe(false);
    expect(schema.properties.production_authority?.const).toBe(false);
    expect(schema.properties.inflight_revocation_authority?.const).toBe(false);
  });

  it("creates and validates canonical expiring low-sensitive evidence", () => {
    const now = new Date("2026-08-28T10:00:00.000Z");
    const evidence = createExternalMcpCredentialLifecycleEvidence(outcome, {
      now: () => now, validityMs: 60_000
    });
    expect(parseExternalMcpCredentialLifecycleEvidence(evidence, {
      now: () => new Date("2026-08-28T10:00:30.000Z")
    })).toEqual(evidence);
    expect(JSON.stringify(evidence)).not.toMatch(/token|secret_ref|credential_ref|tenant|path|endpoint/i);
  });

  it.each([
    { ...create(), transport_close_count: 2 },
    { ...create(), inflight_revocation_authority: true },
    { ...create(), production_authority: true },
    { ...create(), extra: true },
    { ...create(), content_sha256: "f".repeat(64) }
  ])("rejects drifted or elevated evidence", value => {
    expect(() => parseExternalMcpCredentialLifecycleEvidence(value, {
      now: () => new Date("2026-08-28T10:00:30.000Z")
    })).toThrow();
  });

  it("rejects future and expired evidence", () => {
    const evidence = create();
    expect(() => parseExternalMcpCredentialLifecycleEvidence(evidence, {
      now: () => new Date("2026-08-28T09:59:59.999Z")
    })).toThrow();
    expect(() => parseExternalMcpCredentialLifecycleEvidence(evidence, {
      now: () => new Date("2026-08-28T10:01:00.000Z")
    })).toThrow();
  });
});

function create() {
  return createExternalMcpCredentialLifecycleEvidence(outcome, {
    now: () => new Date("2026-08-28T10:00:00.000Z"), validityMs: 60_000
  });
}
