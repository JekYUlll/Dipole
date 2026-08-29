import { Resolver } from "node:dns/promises";
import { isIP } from "node:net";

import type { ExternalMcpDnsResolver, ExternalMcpResolvedAddress } from "./external-mcp-network-guard.js";

const hostnamePattern = /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])$/;

export interface NodeExternalMcpDnsClient {
  resolve4(hostname: string, options: { readonly ttl: true }): Promise<readonly { readonly address: string; readonly ttl: number }[]>;
  resolve6(hostname: string, options: { readonly ttl: true }): Promise<readonly { readonly address: string; readonly ttl: number }[]>;
  cancel(): void;
}

export type NodeExternalMcpDnsClientFactory = () => NodeExternalMcpDnsClient;

export class NodeExternalMcpDnsResolver implements ExternalMcpDnsResolver {
  constructor(private readonly createClient: NodeExternalMcpDnsClientFactory = () => new Resolver()) {}

  async resolve(hostname: string, signal: AbortSignal): Promise<readonly ExternalMcpResolvedAddress[]> {
    signal.throwIfAborted();
    if (!hostnamePattern.test(hostname)) throw new Error("External MCP DNS hostname is invalid");
    const client = this.createClient();
    const cancel = (): void => client.cancel();
    signal.addEventListener("abort", cancel, { once: true });
    try {
      const answers = await Promise.allSettled([
        client.resolve4(hostname, { ttl: true }),
        client.resolve6(hostname, { ttl: true })
      ]);
      signal.throwIfAborted();
      const ipv4 = familyAnswers(answers[0], 4);
      const ipv6 = familyAnswers(answers[1], 6);
      return [...ipv4, ...ipv6];
    } finally {
      signal.removeEventListener("abort", cancel);
    }
  }
}

function familyAnswers(
  result: PromiseSettledResult<readonly { readonly address: string; readonly ttl: number }[]>,
  family: 4 | 6
): readonly ExternalMcpResolvedAddress[] {
  if (result.status === "rejected") {
    if (isNoData(result.reason)) return [];
    throw new Error("External MCP DNS lookup failed");
  }
  return result.value.map(answer => {
    if (isIP(answer.address) !== family || !Number.isSafeInteger(answer.ttl) || answer.ttl < 0) {
      throw new Error("External MCP DNS lookup returned invalid evidence");
    }
    return { address: answer.address, family };
  });
}

function isNoData(error: unknown): boolean {
  if (typeof error !== "object" || error === null || !("code" in error)) return false;
  return error.code === "ENODATA" || error.code === "ENOTFOUND";
}
