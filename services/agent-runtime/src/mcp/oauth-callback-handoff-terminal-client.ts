import * as grpc from "@grpc/grpc-js";

import type {
  CompleteOAuthCallbackHandoffRequest,
  CompleteOAuthCallbackHandoffResponse,
  ReleaseOAuthCallbackHandoffRequest,
  ReleaseOAuthCallbackHandoffResponse
} from "../generated/dipole/agent/v1/agent.js";

const callerService = "dipole-agent";
const handoffIDPattern = /^[A-Za-z0-9_-]{16,64}$/;
const leaseOwnerPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;

export class OAuthCallbackHandoffTerminalInvalidError extends Error {}
export class OAuthCallbackHandoffTerminalDeniedError extends Error {}
export class OAuthCallbackHandoffTerminalUnavailableError extends Error {}

export interface OAuthCallbackHandoffTerminalRPC {
  completeOAuthCallbackHandoff(
    request: CompleteOAuthCallbackHandoffRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (error: grpc.ServiceError | null, response?: CompleteOAuthCallbackHandoffResponse) => void
  ): grpc.ClientUnaryCall;
  releaseOAuthCallbackHandoff(
    request: ReleaseOAuthCallbackHandoffRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (error: grpc.ServiceError | null, response?: ReleaseOAuthCallbackHandoffResponse) => void
  ): grpc.ClientUnaryCall;
}

export interface OAuthCallbackHandoffTerminalRequest {
  readonly handoffId: string;
  readonly leaseOwner: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

/** Ends or releases a Runtime-owned callback lease over the Core mTLS channel. */
export class OAuthCallbackHandoffTerminalClient {
  constructor(
    private readonly rpc: OAuthCallbackHandoffTerminalRPC,
    private readonly sharedSecret: string,
    private readonly timeoutMs = 2_000
  ) {
    if (rpc === undefined || !sharedSecret.trim() || !Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
      throw new OAuthCallbackHandoffTerminalInvalidError("OAuth callback handoff terminal client is invalid");
    }
  }

  complete(input: OAuthCallbackHandoffTerminalRequest): Promise<void> {
    return this.invoke(input, (request, metadata, options, callback) =>
      this.rpc.completeOAuthCallbackHandoff(request, metadata, options, callback));
  }

  release(input: OAuthCallbackHandoffTerminalRequest): Promise<void> {
    return this.invoke(input, (request, metadata, options, callback) =>
      this.rpc.releaseOAuthCallbackHandoff(request, metadata, options, callback));
  }

  private invoke(
    input: OAuthCallbackHandoffTerminalRequest,
    call: (
      request: CompleteOAuthCallbackHandoffRequest,
      metadata: grpc.Metadata,
      options: grpc.CallOptions,
      callback: (error: grpc.ServiceError | null, response?: { handoffId: string }) => void
    ) => grpc.ClientUnaryCall
  ): Promise<void> {
    if (!handoffIDPattern.test(input.handoffId) || !leaseOwnerPattern.test(input.leaseOwner)) {
      return Promise.reject(new OAuthCallbackHandoffTerminalInvalidError("OAuth callback handoff terminal request is invalid"));
    }
    const metadata = new grpc.Metadata();
    metadata.set("x-dipole-caller-service", callerService);
    metadata.set("x-dipole-service-token", this.sharedSecret);
    if (input.requestId !== undefined) metadata.set("x-request-id", input.requestId);
    if (input.traceId !== undefined) metadata.set("x-trace-id", input.traceId);
    const request = {
      context: { principalUserId: "", deviceId: "", requestId: input.requestId ?? "", traceId: input.traceId ?? "", callerService },
      handoffId: input.handoffId,
      leaseOwner: input.leaseOwner
    };
    return new Promise((resolve, reject) => {
      call(request, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null) return reject(mapRPCError(error));
        if (response === undefined || response.handoffId !== input.handoffId) {
          return reject(new OAuthCallbackHandoffTerminalUnavailableError("OAuth callback handoff terminal returned invalid evidence"));
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
      return new OAuthCallbackHandoffTerminalInvalidError("OAuth callback handoff terminal request is invalid");
    case grpc.status.UNAUTHENTICATED:
    case grpc.status.PERMISSION_DENIED:
    case grpc.status.NOT_FOUND:
      return new OAuthCallbackHandoffTerminalDeniedError("OAuth callback handoff terminal is unavailable");
    default:
      return new OAuthCallbackHandoffTerminalUnavailableError("OAuth callback handoff terminal service is unavailable");
  }
}
