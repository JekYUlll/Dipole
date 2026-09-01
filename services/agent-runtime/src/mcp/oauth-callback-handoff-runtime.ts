import type { OAuthCallbackHandoffControlAPI } from "../server.js";
import { createEncryptedFileOAuthCallbackRuntimeKeySource, type OAuthCallbackRuntimeKeySource } from "./node-oauth-callback-runtime-key-source.js";
import { createOAuthCallbackHandoffControlService } from "./oauth-callback-handoff-control-service.js";
import { OAuthCallbackHandoffClaimClient, type OAuthCallbackHandoffClaimRPC } from "./oauth-callback-handoff-claim-client.js";
import { OAuthCallbackHandoffExecutor, type OAuthCallbackHandoffEnvelopeOpener, type OAuthCallbackHandoffProcessor } from "./oauth-callback-handoff-executor.js";
import { OAuthCallbackHandoffTerminalClient, type OAuthCallbackHandoffTerminalRPC } from "./oauth-callback-handoff-terminal-client.js";
import type { OAuthCallbackRuntimeConfig } from "./oauth-callback-runtime-config.js";

type OAuthCallbackHandoffRPC = OAuthCallbackHandoffClaimRPC & OAuthCallbackHandoffTerminalRPC;

export interface OAuthCallbackHandoffRuntimeDependencies {
  readonly rpc: OAuthCallbackHandoffRPC;
  readonly coreSharedSecret: string;
  readonly processor: OAuthCallbackHandoffProcessor;
  readonly keySource?: OAuthCallbackRuntimeKeySource;
  readonly openEnvelope?: OAuthCallbackHandoffEnvelopeOpener;
}

export interface OAuthCallbackHandoffRuntime {
  readonly controlSecret: string;
  readonly service: OAuthCallbackHandoffControlAPI;
}

/**
 * Combines the callback components only when an explicit processor is injected.
 * Runtime bootstrap intentionally does not create this object until the provider
 * exchange and token lifecycle have their own approved deployment profile.
 */
export function createOAuthCallbackHandoffRuntime(
  config: Extract<OAuthCallbackRuntimeConfig, { enabled: true }>,
  dependencies: OAuthCallbackHandoffRuntimeDependencies
): OAuthCallbackHandoffRuntime {
  const coreSharedSecret = dependencies.coreSharedSecret.trim();
  if (coreSharedSecret.length < 16) throw new Error("OAuth callback Runtime Core credential is invalid");
  const keys = dependencies.keySource ?? createEncryptedFileOAuthCallbackRuntimeKeySource({ keys: config.keyPaths });
  const claims = new OAuthCallbackHandoffClaimClient(dependencies.rpc, coreSharedSecret);
  const terminal = new OAuthCallbackHandoffTerminalClient(dependencies.rpc, coreSharedSecret);
  const executor = new OAuthCallbackHandoffExecutor(claims, terminal, keys, dependencies.processor, dependencies.openEnvelope);
  return Object.freeze({
    controlSecret: config.controlSecret,
    service: createOAuthCallbackHandoffControlService(executor, config.leaseOwner)
  });
}
