import type { OAuthCallbackRuntimeKeySource } from "./node-oauth-callback-runtime-key-source.js";
import { openOAuthCallbackEnvelope, type OAuthCallbackEnvelopeBinding } from "./oauth-callback-envelope.js";
import { type OAuthCallbackHandoffClaim, type OAuthCallbackHandoffClaimRequest, OAuthCallbackHandoffClaimClient } from "./oauth-callback-handoff-claim-client.js";
import { OAuthCallbackHandoffTerminalClient } from "./oauth-callback-handoff-terminal-client.js";

export interface OAuthCallbackHandoffProcessor {
  process(input: Readonly<{ authorizationCode: string; handoff: OAuthCallbackHandoffClaim }>): Promise<"completed" | "retryable_failure">;
}

export type OAuthCallbackHandoffEnvelopeOpener = (envelope: string, binding: OAuthCallbackEnvelopeBinding, key: Buffer) => string;

/** Default-closed Runtime composition seam; Runtime bootstrap does not construct it. */
export class OAuthCallbackHandoffExecutor {
  constructor(
    private readonly claims: OAuthCallbackHandoffClaimClient,
    private readonly terminal: OAuthCallbackHandoffTerminalClient,
    private readonly keys: OAuthCallbackRuntimeKeySource,
    private readonly processor: OAuthCallbackHandoffProcessor,
    private readonly openEnvelope: OAuthCallbackHandoffEnvelopeOpener = openOAuthCallbackEnvelope
  ) {}

  async execute(input: OAuthCallbackHandoffClaimRequest): Promise<"completed" | "released"> {
    const handoff = await this.claims.claim(input);
    let authorizationCode: string;
    try {
      authorizationCode = await this.keys.use(handoff.runtimeKeyId, (key) => this.openEnvelope(handoff.sealedAuthorizationCode, binding(handoff), key));
    } catch (error) {
      await this.terminal.release(input);
      throw error;
    }
    const result = await this.processor.process({ authorizationCode, handoff });
    if (result === "retryable_failure") {
      await this.terminal.release(input);
      return "released";
    }
    await this.terminal.complete(input);
    return "completed";
  }
}

function binding(handoff: OAuthCallbackHandoffClaim): OAuthCallbackEnvelopeBinding {
  return { handoffId: handoff.handoffId, transactionId: handoff.transactionId, ownerUserId: handoff.ownerUserId, issuer: handoff.issuer,
    redirectUri: handoff.redirectUri, authorizationCodeSHA256: handoff.authorizationCodeSHA256, runtimeKeyId: handoff.runtimeKeyId,
    expiresAt: handoff.expiresAt.toISOString() };
}
