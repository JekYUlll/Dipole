import type { OAuthCallbackHandoffControlAPI, OAuthCallbackHandoffNotification } from "../server.js";
import { OAuthCallbackHandoffExecutor } from "./oauth-callback-handoff-executor.js";

const leaseOwnerPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;

/** Builds the optional Runtime control service without adding bootstrap wiring. */
export function createOAuthCallbackHandoffControlService(
  executor: OAuthCallbackHandoffExecutor,
  leaseOwner: string
): OAuthCallbackHandoffControlAPI {
  if (!leaseOwnerPattern.test(leaseOwner)) throw new Error("OAuth callback handoff lease owner is invalid");
  return Object.freeze({
    async notifyHandoff(notification: OAuthCallbackHandoffNotification): Promise<void> {
      await executor.execute({ handoffId: notification.handoffId, leaseOwner,
        ...(notification.requestId === undefined ? {} : { requestId: notification.requestId }),
        ...(notification.traceId === undefined ? {} : { traceId: notification.traceId }) });
    }
  });
}
