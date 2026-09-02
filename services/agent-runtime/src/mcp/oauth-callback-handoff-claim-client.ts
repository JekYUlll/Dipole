import * as grpc from "@grpc/grpc-js";

import type { ClaimOAuthCallbackHandoffRequest, ClaimOAuthCallbackHandoffResponse } from "../generated/dipole/agent/v1/agent.js";

const callerService = "dipole-agent";
const handoffIDPattern = /^[A-Za-z0-9_-]{16,64}$/;
const leaseOwnerPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;
const runtimeKeyIDPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;

export class OAuthCallbackHandoffClaimInvalidError extends Error {}
export class OAuthCallbackHandoffClaimDeniedError extends Error {}
export class OAuthCallbackHandoffClaimUnavailableError extends Error {}

export interface OAuthCallbackHandoffClaimRPC {
  claimOAuthCallbackHandoff(
    request: ClaimOAuthCallbackHandoffRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (error: grpc.ServiceError | null, response?: ClaimOAuthCallbackHandoffResponse) => void
  ): grpc.ClientUnaryCall;
}

export interface OAuthCallbackHandoffClaimRequest {
  readonly handoffId: string;
  readonly leaseOwner: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface OAuthCallbackHandoffClaim {
  readonly handoffId: string;
  readonly transactionId: string;
  readonly ownerUserId: string;
  readonly issuer: string;
  readonly redirectUri: string;
  readonly authorizationCodeSHA256: string;
  readonly sealedAuthorizationCode: string;
  readonly runtimeKeyId: string;
  readonly expiresAt: Date;
  readonly leaseExpiresAt: Date;
}

/** Claims one opaque callback record over the Runtime-to-Core mTLS transport. */
export class OAuthCallbackHandoffClaimClient {
  constructor(
    private readonly rpc: OAuthCallbackHandoffClaimRPC,
    private readonly sharedSecret: string,
    private readonly timeoutMs = 2_000
  ) {
    if (rpc === undefined || !sharedSecret.trim() || !Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
      throw new OAuthCallbackHandoffClaimInvalidError("OAuth callback handoff claim client is invalid");
    }
  }

  async claim(input: OAuthCallbackHandoffClaimRequest): Promise<OAuthCallbackHandoffClaim> {
    if (!handoffIDPattern.test(input.handoffId) || !leaseOwnerPattern.test(input.leaseOwner)) {
      throw new OAuthCallbackHandoffClaimInvalidError("OAuth callback handoff claim is invalid");
    }
    const metadata = new grpc.Metadata();
    metadata.set("x-dipole-caller-service", callerService);
    metadata.set("x-dipole-service-token", this.sharedSecret);
    if (input.requestId !== undefined) metadata.set("x-request-id", input.requestId);
    if (input.traceId !== undefined) metadata.set("x-trace-id", input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.claimOAuthCallbackHandoff({
        context: { principalUserId: "", deviceId: "", requestId: input.requestId ?? "", traceId: input.traceId ?? "", callerService },
        handoffId: input.handoffId,
        leaseOwner: input.leaseOwner
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null) return reject(mapRPCError(error));
        try {
          resolve(fromProto(input.handoffId, response));
        } catch {
          reject(new OAuthCallbackHandoffClaimUnavailableError("OAuth callback handoff claim returned invalid evidence"));
        }
      });
    });
  }
}

function fromProto(expectedHandoffID: string, response: ClaimOAuthCallbackHandoffResponse | undefined): OAuthCallbackHandoffClaim {
  if (response === undefined || response.handoffId !== expectedHandoffID || !handoffIDPattern.test(response.transactionId) || !handoffIDPattern.test(response.ownerUserId) ||
      !validURL(response.issuer) || !validURL(response.redirectUri) || !/^[a-f0-9]{64}$/u.test(response.authorizationCodeSha256) ||
      !validEnvelope(response.sealedAuthorizationCode) || !runtimeKeyIDPattern.test(response.runtimeKeyId)) {
    throw new Error("invalid response");
  }
  const expiresAt = dateFromUnixMilliseconds(response.expiresAtUnixMs);
  const leaseExpiresAt = dateFromUnixMilliseconds(response.leaseExpiresAtUnixMs);
  if (expiresAt.getTime() <= Date.now() || leaseExpiresAt.getTime() <= Date.now() || leaseExpiresAt > expiresAt) throw new Error("expired response");
  return Object.freeze({ handoffId: response.handoffId, transactionId: response.transactionId, ownerUserId: response.ownerUserId, issuer: response.issuer, redirectUri: response.redirectUri,
    authorizationCodeSHA256: response.authorizationCodeSha256, sealedAuthorizationCode: response.sealedAuthorizationCode,
    runtimeKeyId: response.runtimeKeyId, expiresAt, leaseExpiresAt });
}

function validURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "https:" && parsed.username === "" && parsed.password === "" && parsed.search === "" && parsed.hash === "";
  } catch { return false; }
}

function validEnvelope(value: string): boolean {
  const parts = value.split(".");
  return value.length <= 4096 && parts.length === 5 && parts[0] === "v1" && parts.slice(1).every((part) => /^[A-Za-z0-9_-]{1,2048}$/u.test(part));
}

function dateFromUnixMilliseconds(value: bigint): Date {
  if (value < 0n || value > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error("invalid timestamp");
  const date = new Date(Number(value));
  if (Number.isNaN(date.getTime())) throw new Error("invalid timestamp");
  return date;
}

function mapRPCError(error: grpc.ServiceError): Error {
  switch (error.code) {
    case grpc.status.INVALID_ARGUMENT:
    case grpc.status.FAILED_PRECONDITION:
      return new OAuthCallbackHandoffClaimInvalidError("OAuth callback handoff claim is invalid");
    case grpc.status.UNAUTHENTICATED:
    case grpc.status.PERMISSION_DENIED:
    case grpc.status.NOT_FOUND:
      return new OAuthCallbackHandoffClaimDeniedError("OAuth callback handoff claim is unavailable");
    default:
      return new OAuthCallbackHandoffClaimUnavailableError("OAuth callback handoff claim service is unavailable");
  }
}
