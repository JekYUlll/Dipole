import { describe, expect, it } from "vitest";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { createTemporalFaultReceipt, validateTemporalFaultReceipt } from "./temporal-fault-receipt.js";
import { runTemporalFaultReceiptCLI } from "./temporal-fault-receipt-cli.js";

const validObservation = {
  schemaVersion: "dipole.agent.temporal-fault-observation.v1",
  drillId: "worker_replacement_approval_resume",
  observedAt: "2026-08-31T12:00:00.000Z",
  workflow: { taskId: "TASK-1", runId: "RUN-1" },
  transitions: [
    { revision: 1, status: "running" }, { revision: 2, status: "waiting_approval" },
    { revision: 3, status: "running" }, { revision: 4, status: "completed" }
  ],
  faults: { workerReplacements: 1, terminalWriteRetries: 1 },
  effects: { admissions: 1, stepExecutions: 4, approvalRequests: 1, approvalResolutions: 1, terminalWriteAttempts: 2, terminalPersistedWrites: 1, inputSignalsRejected: 0, inputResumptions: 0 }
} as const;

describe("Temporal fault receipt", () => {
  it("binds the durable Worker replacement observation to its state transitions and side-effect counts", () => {
    const receipt = createTemporalFaultReceipt(validObservation);
    expect(receipt).toMatchObject({ outcome: "eligible", failures: [], receiptId: expect.stringMatching(/^TEMPORAL-FAULT-[a-f0-9]{64}$/) });
    expect(validateTemporalFaultReceipt(receipt)).toEqual(receipt);
  });

  it("reports incompatible transition or terminal side-effect evidence without promoting the drill", () => {
    const receipt = createTemporalFaultReceipt({ ...validObservation, effects: { ...validObservation.effects, terminalPersistedWrites: 2 } });
    expect(receipt).toMatchObject({ outcome: "ineligible", failures: ["terminal_write_side_effect_count_mismatch"] });
  });

  it("accepts the exact input resume path after Worker replacement", () => {
    expect(createTemporalFaultReceipt({ ...validObservation, drillId: "worker_replacement_input_resume", transitions: [
      { revision: 1, status: "running" }, { revision: 2, status: "waiting_input" }, { revision: 3, status: "running" }, { revision: 4, status: "completed" }
    ], faults: { workerReplacements: 1, terminalWriteRetries: 0 }, effects: { admissions: 1, stepExecutions: 2, approvalRequests: 0, approvalResolutions: 0, terminalWriteAttempts: 1, terminalPersistedWrites: 1, inputSignalsRejected: 2, inputResumptions: 1 } })).toMatchObject({ outcome: "eligible", failures: [] });
  });

  it("rejects a receipt whose bound observation has been changed", () => {
    const receipt = createTemporalFaultReceipt(validObservation);
    expect(() => validateTemporalFaultReceipt({ ...receipt, outcome: "ineligible" })).toThrow(/binding/i);
  });

  it("reads one observation file and emits the content-bound receipt", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-temporal-fault-"));
    const path = join(directory, "observation.json");
    const output: string[] = [];
    const errors: string[] = [];
    try {
      await writeFile(path, JSON.stringify(validObservation), "utf8");
      await expect(runTemporalFaultReceiptCLI([`--observation=${path}`], { write: value => output.push(value) }, { write: value => errors.push(value) })).resolves.toBe(0);
      expect(JSON.parse(output.join(""))).toMatchObject({ outcome: "eligible", failures: [] });
      expect(errors).toEqual([]);
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  });
});
