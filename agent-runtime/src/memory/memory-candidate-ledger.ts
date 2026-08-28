import { createHash } from "node:crypto";

import type { Pool, ResultSetHeader, RowDataPacket } from "mysql2/promise";

import { parseMemoryCandidate, type MemoryCandidate } from "./observation-worker.js";

const INSERT_CANDIDATE = `INSERT INTO agent_memory_candidates (
  candidate_uuid, tenant_id, principal_uuid, agent_uuid, resource_type, resource_id,
  candidate_type, source_id, evidence_ids_json, summary, policy_version, candidate_sha256,
  status, observed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`;
const GET_CANDIDATE = `SELECT candidate_sha256 FROM agent_memory_candidates WHERE candidate_uuid = ? LIMIT 1`;

export interface MemoryCandidateLedgerQueryExecutor {
  execute(sql: string, values?: unknown[]): Promise<[unknown, unknown]>;
}

interface CandidateHashRow extends RowDataPacket {
  candidate_sha256: string;
}

export type CandidateAppendResult = { readonly outcome: "inserted" | "duplicate" };

export class MySQLMemoryCandidateLedger {
  constructor(private readonly pool: Pool | MemoryCandidateLedgerQueryExecutor) {}

  private execute<T>(sql: string, values: unknown[] = []): Promise<[T, unknown]> {
    return (this.pool.execute as unknown as (query: string, parameters: unknown[]) => Promise<[T, unknown]>)(sql, values);
  }

  async append(rawCandidate: MemoryCandidate, rawEvidenceIds: readonly string[], rawPolicyVersion: string): Promise<CandidateAppendResult> {
    const candidate = parseMemoryCandidate(rawCandidate);
    const evidenceIds = normalizeEvidenceIds(rawEvidenceIds, candidate.provenance.sourceId);
    const policyVersion = normalizePolicyVersion(rawPolicyVersion);
    const summary = normalizeSummary(candidate.compactContent);
    const hash = this.candidateHash(candidate, evidenceIds, policyVersion);
    try {
      const [result] = await this.execute<ResultSetHeader>(INSERT_CANDIDATE, [
        candidate.memoryId, candidate.tenantId, candidate.principalId, candidate.agentId,
        candidate.resourceType, candidate.resourceId, candidate.provenance.sourceType,
        candidate.provenance.sourceId, JSON.stringify(evidenceIds), summary, policyVersion,
        hash, candidate.observedAt,
      ]);
      if (result.affectedRows === 1) return { outcome: "inserted" };
    } catch (error) {
      if (!isDuplicateKey(error)) throw error;
    }

    const [rows] = await this.execute<CandidateHashRow[]>(GET_CANDIDATE, [candidate.memoryId]);
    const existing = rows[0];
    if (existing === undefined) throw new Error("Agent Memory candidate ledger write is indeterminate");
    if (existing.candidate_sha256 !== hash) throw new Error(`Agent Memory candidate conflict for ${candidate.memoryId}`);
    return { outcome: "duplicate" };
  }

  candidateHash(rawCandidate: MemoryCandidate, evidenceIds: readonly string[], policyVersion: string): string {
    const candidate = parseMemoryCandidate(rawCandidate);
    const canonical = JSON.stringify({
      schemaVersion: candidate.schemaVersion,
      candidateId: candidate.memoryId,
      tenantId: candidate.tenantId,
      principalId: candidate.principalId,
      agentId: candidate.agentId,
      resourceType: candidate.resourceType,
      resourceId: candidate.resourceId,
      candidateType: candidate.provenance.sourceType,
      sourceId: candidate.provenance.sourceId,
      sequence: candidate.provenance.sequence ?? null,
      summary: candidate.compactContent,
      evidenceIds: [...evidenceIds],
      policyVersion,
      observedAt: candidate.observedAt,
    });
    return createHash("sha256").update(canonical, "utf8").digest("hex");
  }
}

function normalizeEvidenceIds(values: readonly string[], sourceId: string): string[] {
  if (!Array.isArray(values) || values.length === 0 || values.length > 128) throw new Error("Agent Memory candidate evidence is invalid");
  const ids = values.map((value) => {
    if (typeof value !== "string" || !/^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,127}$/.test(value.trim())) throw new Error("Agent Memory candidate evidence is invalid");
    return value.trim();
  });
  if (new Set(ids).size !== ids.length || !ids.includes(sourceId)) throw new Error("Agent Memory candidate evidence is invalid");
  return [...ids].sort();
}

function normalizePolicyVersion(value: string): string {
  if (typeof value !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$/.test(value.trim())) throw new Error("Agent Memory candidate policy version is invalid");
  return value.trim();
}

function normalizeSummary(value: string): string {
  const summary = value.trim();
  if (!summary || summary.length > 4096 || /(?:password|passwd|token|secret|authorization|bearer|api[_ -]?key)\s*[:=]/i.test(summary)) {
    throw new Error("Agent Memory candidate summary is invalid");
  }
  return summary;
}

function isDuplicateKey(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && error.code === "ER_DUP_ENTRY";
}
