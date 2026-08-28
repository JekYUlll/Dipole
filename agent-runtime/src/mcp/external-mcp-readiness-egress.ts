import type { Transport } from "@modelcontextprotocol/client";

import type {
  AgentMCPReadinessEvidenceResolution
} from "../capabilities/agent-capability-rpc.js";
import type { ExternalMcpConfig, ExternalMcpProfile } from "./external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "./external-mcp-production-io.js";
import {
  externalMcpReadinessBindingSha256,
  externalMcpReadinessProfileBindingSha256,
  type ExternalMcpReadinessBindingOptions
} from "./external-mcp-readiness-evidence.js";

const sha256Pattern = /^[a-f0-9]{64}$/;

export interface ExternalMcpFreshReadinessResolver {
  resolveFreshMcpReadinessEvidence(
    tenantId: string,
    profileBindingSha256: string,
    runtimeBindingSha256: string
  ): Promise<AgentMCPReadinessEvidenceResolution | undefined>;
}

export interface ExternalMcpReadinessUnderlyingRegistry {
  describe(profileId: string, tenantId: string): ExternalMcpProfile;
  connect(profileId: string, tenantId: string, signal?: AbortSignal): Promise<Transport>;
}

export class ExternalMcpReadinessEgressAuthorizer {
  readonly #config: ExternalMcpConfig;
  readonly #runtimeBindingSha256: string;
  readonly #resolver: ExternalMcpFreshReadinessResolver;
  readonly #now: () => number;

  constructor(
    config: ExternalMcpConfig,
    io: ExternalMcpProductionIoConfig,
    resolver: ExternalMcpFreshReadinessResolver,
    options: ExternalMcpReadinessBindingOptions = {},
    now: () => number = Date.now
  ) {
    this.#config = config;
    this.#runtimeBindingSha256 = externalMcpReadinessBindingSha256(config, io, options);
    this.#resolver = resolver;
    this.#now = now;
  }

  async authorize(profileId: string, tenantId: string, signal?: AbortSignal): Promise<AgentMCPReadinessEvidenceResolution> {
    signal?.throwIfAborted();
    try {
      if (!this.#config.enabled) throw new Error("disabled");
      const profile = this.#config.profiles.find(candidate =>
        candidate.profileId === profileId && candidate.tenantId === tenantId
      );
      if (profile === undefined) throw new Error("unknown Profile binding");
      const profileBindingSha256 = externalMcpReadinessProfileBindingSha256(profile);
      const receipt = await this.#resolver.resolveFreshMcpReadinessEvidence(
        tenantId,
        profileBindingSha256,
        this.#runtimeBindingSha256
      );
      signal?.throwIfAborted();
      assertExactReceipt(receipt, profileBindingSha256, this.#runtimeBindingSha256, this.#now());
      return receipt;
    } catch {
      if (signal?.aborted) signal.throwIfAborted();
      throw new Error("External MCP readiness authorization failed");
    }
  }
}

export class ExternalMcpReadinessGatedTransportRegistry implements ExternalMcpReadinessUnderlyingRegistry {
  readonly #underlying: ExternalMcpReadinessUnderlyingRegistry;
  readonly #authorizer: ExternalMcpReadinessEgressAuthorizer;

  constructor(
    config: ExternalMcpConfig,
    io: ExternalMcpProductionIoConfig,
    underlying: ExternalMcpReadinessUnderlyingRegistry,
    resolver: ExternalMcpFreshReadinessResolver,
    options: ExternalMcpReadinessBindingOptions = {},
    now: () => number = Date.now
  ) {
    this.#underlying = underlying;
    this.#authorizer = new ExternalMcpReadinessEgressAuthorizer(config, io, resolver, options, now);
  }

  describe(profileId: string, tenantId: string): ExternalMcpProfile {
    return this.#underlying.describe(profileId, tenantId);
  }

  async connect(profileId: string, tenantId: string, signal?: AbortSignal): Promise<Transport> {
    const authorization = await this.#authorizer.authorize(profileId, tenantId, signal);
    signal?.throwIfAborted();
    try {
      const actualProfile = this.#underlying.describe(profileId, tenantId);
      if (externalMcpReadinessProfileBindingSha256(actualProfile) !== authorization.profileBindingSha256) {
        throw new Error("underlying Profile drift");
      }
      signal?.throwIfAborted();
    } catch {
      if (signal?.aborted) signal.throwIfAborted();
      throw new Error("External MCP readiness authorization failed");
    }
    return this.#underlying.connect(profileId, tenantId, signal);
  }
}

function assertExactReceipt(
  receipt: AgentMCPReadinessEvidenceResolution | undefined,
  profileBindingSha256: string,
  runtimeBindingSha256: string,
  nowUnixMs: number
): asserts receipt is AgentMCPReadinessEvidenceResolution {
  if (receipt === undefined ||
      receipt.profileBindingSha256 !== profileBindingSha256 ||
      receipt.runtimeBindingSha256 !== runtimeBindingSha256 ||
      !sha256Pattern.test(receipt.evidenceId) ||
      !sha256Pattern.test(receipt.contentSha256)) {
    throw new Error("readiness receipt binding is invalid");
  }
  const collectedAt = new Date(receipt.collectedAt);
  const expiresAt = new Date(receipt.expiresAt);
  if (!Number.isFinite(collectedAt.getTime()) || !Number.isFinite(expiresAt.getTime()) ||
      collectedAt.toISOString() !== receipt.collectedAt || expiresAt.toISOString() !== receipt.expiresAt ||
      collectedAt.getTime() >= expiresAt.getTime() || !Number.isFinite(nowUnixMs) || expiresAt.getTime() <= nowUnixMs) {
    throw new Error("readiness receipt time is invalid");
  }
}
