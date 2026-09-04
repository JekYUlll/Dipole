import * as grpc from "@grpc/grpc-js";

import type { PersistOAuthTokenLifecycleRequest, PersistOAuthTokenLifecycleResponse } from "../generated/dipole/agent/v1/agent.js";
import type { OAuthCallbackHandoffClaim } from "./oauth-callback-handoff-claim-client.js";
import type { OAuthCallbackRuntimeKeySource } from "./node-oauth-callback-runtime-key-source.js";
import { sealOAuthTokenLifecycleBundle, type SealedOAuthTokenLifecycleBundle } from "./oauth-token-lifecycle-envelope.js";
import type { TokenLifecycleBundle } from "./oauth-callback-token-lifecycle.js";

const callerService = "dipole-agent";
const leaseOwnerPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;

export class OAuthTokenLifecyclePersistenceInvalidError extends Error {}
export class OAuthTokenLifecyclePersistenceDeniedError extends Error {}
export class OAuthTokenLifecyclePersistenceUnavailableError extends Error {}

export interface OAuthTokenLifecyclePersistenceRPC {
  persistOAuthTokenLifecycle(
    request: PersistOAuthTokenLifecycleRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (error: grpc.ServiceError | null, response?: PersistOAuthTokenLifecycleResponse) => void
  ): grpc.ClientUnaryCall;
}

export interface OAuthTokenLifecyclePersistenceInput {
  readonly handoff: OAuthCallbackHandoffClaim;
  readonly leaseOwner: string;
  readonly bundle: TokenLifecycleBundle;
  readonly requestId?: string;
  readonly traceId?: string;
}

/** Runtime-to-Core token lifecycle writer. The RPC receives opaque ciphertext only. */
export class OAuthTokenLifecyclePersistenceClient {
  constructor(
    private readonly rpc: OAuthTokenLifecyclePersistenceRPC,
    private readonly sharedSecret: string,
    private readonly keys: OAuthCallbackRuntimeKeySource,
    private readonly timeoutMs = 2_000,
    private readonly seal = sealOAuthTokenLifecycleBundle
  ) {
    if (rpc === undefined || keys === undefined || !sharedSecret.trim() || !Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
      throw new OAuthTokenLifecyclePersistenceInvalidError("OAuth token lifecycle persistence client is invalid");
    }
  }

  async persistActive(input: OAuthTokenLifecyclePersistenceInput): Promise<void> {
    if (!leaseOwnerPattern.test(input.leaseOwner) || input.handoff.leaseExpiresAt.getTime() <= Date.now() || input.handoff.expiresAt.getTime() <= Date.now()) {
      throw new OAuthTokenLifecyclePersistenceInvalidError("OAuth token lifecycle persistence request is invalid");
    }
    await this.keys.use(input.handoff.runtimeKeyId, async (privateKey) => {
      const sealed = this.seal(input.bundle, { handoffId: input.handoff.handoffId, runtimeKeyId: input.handoff.runtimeKeyId, state: "active" }, privateKey);
      await this.invoke(input, sealed);
    });
  }

  private invoke(input: OAuthTokenLifecyclePersistenceInput, sealed: SealedOAuthTokenLifecycleBundle): Promise<void> {
    const metadata = new grpc.Metadata();
    metadata.set("x-dipole-caller-service", callerService);
    metadata.set("x-dipole-service-token", this.sharedSecret);
    if (input.requestId !== undefined) metadata.set("x-request-id", input.requestId);
    if (input.traceId !== undefined) metadata.set("x-trace-id", input.traceId);
    const request: PersistOAuthTokenLifecycleRequest = {
      context: { principalUserId: "", deviceId: "", requestId: input.requestId ?? "", traceId: input.traceId ?? "", callerService },
      handoffId: input.handoff.handoffId, leaseOwner: input.leaseOwner, state: "active", sealedTokenBundle: sealed.envelope,
      tokenBundleSha256: sealed.sha256, accessTokenExpiresAtUnixMs: BigInt(sealed.expiresAt.getTime()), scope: sealed.scope, revocationReason: ""
    };
    return new Promise((resolve, reject) => {
      this.rpc.persistOAuthTokenLifecycle(request, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null) return reject(mapRPCError(error));
        if (response === undefined || response.handoffId !== input.handoff.handoffId || response.state !== "active") {
          return reject(new OAuthTokenLifecyclePersistenceUnavailableError("OAuth token lifecycle persistence returned invalid evidence"));
        }
        resolve();
      });
    });
  }
}

function mapRPCError(error: grpc.ServiceError): Error {
  switch (error.code) {
    case grpc.status.INVALID_ARGUMENT:
    case grpc.status.FAILED_PRECONDITION:
      return new OAuthTokenLifecyclePersistenceInvalidError("OAuth token lifecycle persistence request is invalid");
    case grpc.status.UNAUTHENTICATED:
    case grpc.status.PERMISSION_DENIED:
    case grpc.status.NOT_FOUND:
      return new OAuthTokenLifecyclePersistenceDeniedError("OAuth token lifecycle persistence is unavailable");
    default:
      return new OAuthTokenLifecyclePersistenceUnavailableError("OAuth token lifecycle persistence service is unavailable");
  }
}
