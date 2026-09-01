import type {
  AgentCapabilityRPCClient,
  AgentMemoryPromotionReceiptCommitResult
} from "../capabilities/agent-capability-rpc.js";
import type {
  AgentMemoryPromotionActivities,
  AgentMemoryPromotionReceiptCommitActivityInput
} from "./agent-task-activities.js";

export function createAgentMemoryPromotionCommitActivities(
  client: Pick<AgentCapabilityRPCClient, "commitMemoryPromotionReceipt">
): Pick<AgentMemoryPromotionActivities, "commitPreparedAgentMemoryPromotion"> {
  return {
    async commitPreparedAgentMemoryPromotion(input: AgentMemoryPromotionReceiptCommitActivityInput): Promise<AgentMemoryPromotionReceiptCommitResult> {
      return client.commitMemoryPromotionReceipt(input.receipt, {
        ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
        ...(input.traceId === undefined ? {} : { traceId: input.traceId })
      });
    }
  };
}
