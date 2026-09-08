import type { EventLedger } from "../events/event-ledger.js";
import type { AgentTaskLifecycleActivities } from "./agent-task-activities.js";

/**
 * Settles the Kafka claim only after Temporal has persisted a terminal Task.
 * A failed or cancelled Workflow releases its claim for a later redelivery.
 */
export function createAgentEventLedgerLifecycleActivities(
  ledger: EventLedger
): Pick<AgentTaskLifecycleActivities, "settleInboundEvent"> {
  return {
    async settleInboundEvent(input): Promise<void> {
      if (input.status === "completed") {
        await ledger.complete(input.claim);
        return;
      }
      await ledger.release(input.claim, input.error ?? input.status);
    }
  };
}
