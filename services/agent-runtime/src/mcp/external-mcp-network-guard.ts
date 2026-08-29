import type { FetchLike } from "@modelcontextprotocol/client";
import { BlockList, isIP } from "node:net";

import type { ExternalMcpProfile } from "./external-mcp-profile.js";

export type ExternalMcpNetworkErrorCode =
  | "request_cancelled"
  | "request_denied"
  | "dns_timeout"
  | "dns_unavailable"
  | "dns_non_public"
  | "connection_mismatch"
  | "redirect_denied"
  | "transport_unavailable";

export class ExternalMcpNetworkError extends Error {
  readonly code: ExternalMcpNetworkErrorCode;

  constructor(code: ExternalMcpNetworkErrorCode) {
    super({
      request_cancelled: "External MCP network request was cancelled",
      request_denied: "External MCP network request is outside its profile",
      dns_timeout: "External MCP DNS resolution timed out",
      dns_unavailable: "External MCP DNS resolution is unavailable",
      dns_non_public: "External MCP DNS resolution was denied",
      connection_mismatch: "External MCP network connection was denied",
      redirect_denied: "External MCP network redirect was denied",
      transport_unavailable: "External MCP network transport is unavailable"
    }[code]);
    this.name = "ExternalMcpNetworkError";
    this.code = code;
  }
}

export interface ExternalMcpResolvedAddress {
  readonly address: string;
  readonly family: 4 | 6;
}

export interface ExternalMcpDnsResolver {
  resolve(hostname: string, signal: AbortSignal): Promise<readonly ExternalMcpResolvedAddress[]>;
}

export interface ExternalMcpNetworkDispatcher {
  dispatch(input: {
    readonly request: Request;
    readonly addresses: readonly ExternalMcpResolvedAddress[];
    readonly tlsServerName: string;
    readonly caBundleRef: string;
  }, signal: AbortSignal): Promise<{
    readonly response: Response;
    readonly connectedAddress: string;
  }>;
}

export interface ExternalMcpNetworkGuardOptions {
  readonly timeoutMs?: number;
}

export function createExternalMcpNetworkGuardedFetch(
  profile: ExternalMcpProfile,
  resolver: ExternalMcpDnsResolver,
  dispatcher: ExternalMcpNetworkDispatcher,
  options: ExternalMcpNetworkGuardOptions = {}
): FetchLike {
  const timeoutMs = options.timeoutMs ?? 2000;
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
    throw new Error("External MCP DNS timeout must be between 100 and 60000 milliseconds");
  }
  const allowedHosts = new Set(profile.allowedHosts);
  const allowedPorts = new Set(profile.allowedPorts);

  return async (input, init) => {
    let request: Request;
    try {
      const original = new Request(input, init);
      request = new Request(original, { redirect: "manual" });
      assertAllowedRequest(request.url, allowedHosts, allowedPorts, profile.tlsServerName);
    } catch (error) {
      if (error instanceof ExternalMcpNetworkError) throw error;
      throw new ExternalMcpNetworkError("request_denied");
    }
    if (request.signal.aborted) throw new ExternalMcpNetworkError("request_cancelled");

    const addresses = await resolvePublicAddresses(request, resolver, timeoutMs);
    let dispatched: Awaited<ReturnType<ExternalMcpNetworkDispatcher["dispatch"]>>;
    try {
      dispatched = await dispatcher.dispatch({
        request,
        addresses,
        tlsServerName: profile.tlsServerName,
        caBundleRef: profile.caBundleRef
      }, request.signal);
    } catch {
      if (request.signal.aborted) throw new ExternalMcpNetworkError("request_cancelled");
      throw new ExternalMcpNetworkError("transport_unavailable");
    }
    if (!addresses.some(answer => answer.address === dispatched.connectedAddress)) {
      await cancelBody(dispatched.response);
      throw new ExternalMcpNetworkError("connection_mismatch");
    }
    const response = dispatched.response;
    if (
      response.redirected
      || (response.status >= 300 && response.status < 400)
      || (response.url !== "" && response.url !== request.url)
    ) {
      await cancelBody(response);
      throw new ExternalMcpNetworkError("redirect_denied");
    }
    return response;
  };
}

async function resolvePublicAddresses(
  request: Request,
  resolver: ExternalMcpDnsResolver,
  timeoutMs: number
): Promise<readonly ExternalMcpResolvedAddress[]> {
  const deadlineController = new AbortController();
  const signal = AbortSignal.any([request.signal, deadlineController.signal]);
  const timeoutError = new ExternalMcpNetworkError("dns_timeout");
  const cancellationError = new ExternalMcpNetworkError("request_cancelled");
  let removeCancellationListener = (): void => undefined;
  const cancellation = new Promise<never>((_resolve, reject) => {
    if (request.signal.aborted) {
      reject(cancellationError);
      return;
    }
    const onAbort = (): void => reject(cancellationError);
    request.signal.addEventListener("abort", onAbort, { once: true });
    removeCancellationListener = () => request.signal.removeEventListener("abort", onAbort);
  });
  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => {
      deadlineController.abort(timeoutError);
      reject(timeoutError);
    }, timeoutMs);
    timer.unref?.();
  });

  let addresses: readonly ExternalMcpResolvedAddress[];
  try {
    addresses = await Promise.race([
      Promise.resolve().then(() => resolver.resolve(new URL(request.url).hostname, signal)),
      deadline,
      cancellation
    ]);
  } catch (error) {
    if (error === timeoutError) throw error;
    if (error === cancellationError) throw error;
    if (request.signal.aborted) throw new ExternalMcpNetworkError("request_cancelled");
    throw new ExternalMcpNetworkError("dns_unavailable");
  } finally {
    if (timer !== undefined) clearTimeout(timer);
    removeCancellationListener();
  }
  if (addresses.length === 0 || addresses.length > 32) {
    throw new ExternalMcpNetworkError("dns_non_public");
  }
  const seen = new Set<string>();
  for (const answer of addresses) {
    const actualFamily = isIP(answer.address);
    if (actualFamily !== answer.family || !isPublicAddress(answer.address, answer.family)) {
      throw new ExternalMcpNetworkError("dns_non_public");
    }
    const key = `${answer.family}:${answer.address}`;
    if (seen.has(key)) throw new ExternalMcpNetworkError("dns_non_public");
    seen.add(key);
  }
  return addresses.map(answer => ({ ...answer }));
}

function assertAllowedRequest(
  rawUrl: string,
  allowedHosts: ReadonlySet<string>,
  allowedPorts: ReadonlySet<number>,
  tlsServerName: string
): void {
  const url = new URL(rawUrl);
  const hostname = url.hostname.toLowerCase();
  const port = url.port === "" ? 443 : Number(url.port);
  if (
    url.protocol !== "https:"
    || url.username !== ""
    || url.password !== ""
    || url.search !== ""
    || url.hash !== ""
    || hostname !== tlsServerName
    || !allowedHosts.has(hostname)
    || !allowedPorts.has(port)
  ) {
    throw new ExternalMcpNetworkError("request_denied");
  }
}

const deniedIpv4Addresses = new BlockList();
for (const [network, prefix] of [
  ["0.0.0.0", 8], ["10.0.0.0", 8], ["100.64.0.0", 10], ["127.0.0.0", 8],
  ["169.254.0.0", 16], ["172.16.0.0", 12], ["192.0.0.0", 24], ["192.0.2.0", 24],
  ["192.88.99.0", 24], ["192.168.0.0", 16], ["198.18.0.0", 15], ["198.51.100.0", 24], ["203.0.113.0", 24],
  ["224.0.0.0", 4], ["240.0.0.0", 4]
] as const) {
  deniedIpv4Addresses.addSubnet(network, prefix, "ipv4");
}
const deniedIpv6Addresses = new BlockList();
for (const [network, prefix] of [
  ["::", 128], ["::1", 128], ["::ffff:0:0", 96], ["64:ff9b::", 96],
  ["64:ff9b:1::", 48], ["100::", 64], ["2001::", 23], ["2001:db8::", 32], ["2002::", 16],
  ["3fff::", 20], ["5f00::", 16], ["fc00::", 7], ["fe80::", 10], ["ff00::", 8]
] as const) {
  deniedIpv6Addresses.addSubnet(network, prefix, "ipv6");
}

function isPublicAddress(address: string, family: 4 | 6): boolean {
  return family === 4
    ? !deniedIpv4Addresses.check(address, "ipv4")
    : !deniedIpv6Addresses.check(address, "ipv6");
}

async function cancelBody(response: Response): Promise<void> {
  try {
    await response.body?.cancel();
  } catch {
    // The response is already unusable once the guard rejects it.
  }
}
