import { request as httpsRequest, type RequestOptions } from "node:https";
import { isIP, type LookupFunction } from "node:net";
import { Readable } from "node:stream";
import type { TLSSocket } from "node:tls";

import type {
  ExternalMcpNetworkDispatcher,
  ExternalMcpResolvedAddress
} from "./external-mcp-network-guard.js";
import {
  validateExternalMcpCaBundle,
  type ExternalMcpCaBundleProvider
} from "./node-external-mcp-ca-bundle-provider.js";

const serverNamePattern = /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])$/;
const caBundleRefPattern = /^CA-[A-Z0-9]{16,64}$/;
const deniedHeaders = new Set(["connection", "proxy-authorization", "proxy-connection", "te", "trailer", "transfer-encoding", "upgrade"]);

export interface NodeExternalMcpHttpsClient {
  request(input: {
    readonly request: Request;
    readonly addresses: readonly ExternalMcpResolvedAddress[];
    readonly tlsServerName: string;
    readonly ca: Uint8Array;
    readonly connectTimeoutMs: number;
  }, signal: AbortSignal): Promise<{ readonly response: Response; readonly connectedAddress: string }>;
}

export interface NodeExternalMcpPinnedTlsDispatcherOptions {
  readonly connectTimeoutMs?: number;
  readonly httpsClient?: NodeExternalMcpHttpsClient;
}

export class NodeExternalMcpPinnedTlsDispatcher implements ExternalMcpNetworkDispatcher {
  readonly #connectTimeoutMs: number;
  readonly #httpsClient: NodeExternalMcpHttpsClient;

  constructor(
    private readonly caBundles: ExternalMcpCaBundleProvider,
    options: NodeExternalMcpPinnedTlsDispatcherOptions = {}
  ) {
    this.#connectTimeoutMs = options.connectTimeoutMs ?? 5_000;
    if (!Number.isSafeInteger(this.#connectTimeoutMs) || this.#connectTimeoutMs < 100 || this.#connectTimeoutMs > 60_000) {
      throw new Error("External MCP TLS connect timeout must be between 100 and 60000 milliseconds");
    }
    this.#httpsClient = options.httpsClient ?? nodeHttpsClient;
  }

  async dispatch(input: {
    readonly request: Request;
    readonly addresses: readonly ExternalMcpResolvedAddress[];
    readonly tlsServerName: string;
    readonly caBundleRef: string;
  }, signal: AbortSignal): Promise<{ readonly response: Response; readonly connectedAddress: string }> {
    signal.throwIfAborted();
    validateInput(input);
    const ca = await this.caBundles.read(input.caBundleRef, signal);
    signal.throwIfAborted();
    validateExternalMcpCaBundle(ca);
    const dispatched = await this.#httpsClient.request({
      request: input.request,
      addresses: input.addresses.map(address => ({ ...address })),
      tlsServerName: input.tlsServerName,
      ca,
      connectTimeoutMs: this.#connectTimeoutMs
    }, signal);
    signal.throwIfAborted();
    const connectedAddress = approvedPeer(dispatched.connectedAddress, input.addresses);
    if (connectedAddress === undefined) {
      await cancelBody(dispatched.response);
      throw new Error("External MCP TLS peer is outside the approved address set");
    }
    return { response: dispatched.response, connectedAddress };
  }
}

const nodeHttpsClient: NodeExternalMcpHttpsClient = {
  request(input, signal) {
    return dispatchNodeHttps(input, signal);
  }
};

function dispatchNodeHttps(
  input: Parameters<NodeExternalMcpHttpsClient["request"]>[0],
  signal: AbortSignal
): Promise<{ readonly response: Response; readonly connectedAddress: string }> {
  const url = new URL(input.request.url);
  const headers = requestHeaders(input.request, url);
  const lookup = pinnedLookup(input.addresses);
  const options: RequestOptions & { autoSelectFamily: boolean; autoSelectFamilyAttemptTimeout: number } = {
    protocol: "https:",
    hostname: input.tlsServerName,
    port: url.port === "" ? 443 : Number(url.port),
    path: `${url.pathname}${url.search}`,
    method: input.request.method,
    headers,
    servername: input.tlsServerName,
    ca: Buffer.from(input.ca),
    rejectUnauthorized: true,
    minVersion: "TLSv1.2",
    agent: false,
    lookup,
    autoSelectFamily: input.addresses.length > 1,
    autoSelectFamilyAttemptTimeout: 250
  };

  return new Promise((resolve, reject) => {
    let settled = false;
    let connectTimer: ReturnType<typeof setTimeout> | undefined;
    const fail = (): void => {
      if (settled) return;
      settled = true;
      cleanup();
      reject(new Error("External MCP TLS request failed"));
    };
    const request = httpsRequest(options, response => {
      if (settled) {
        response.destroy();
        return;
      }
      const remoteAddress = response.socket.remoteAddress;
      const connectedAddress = remoteAddress === undefined ? undefined : approvedPeer(remoteAddress, input.addresses);
      if (connectedAddress === undefined) {
        response.destroy();
        fail();
        return;
      }
      settled = true;
      if (connectTimer !== undefined) clearTimeout(connectTimer);
      const onClose = (): void => cleanup();
      response.once("close", onClose);
      const allowsBody = input.request.method !== "HEAD" && response.statusCode !== 204 && response.statusCode !== 304;
      const responseHeaders = new Headers();
      for (let index = 0; index < response.rawHeaders.length; index += 2) {
        responseHeaders.append(response.rawHeaders[index]!, response.rawHeaders[index + 1]!);
      }
      resolve({
        response: new Response(allowsBody ? Readable.toWeb(response) as ReadableStream<Uint8Array> : undefined, {
          status: response.statusCode ?? 500,
          headers: responseHeaders,
          ...(response.statusMessage === undefined ? {} : { statusText: response.statusMessage })
        }),
        connectedAddress
      });
    });
    const onAbort = (): void => { request.destroy(new Error("request cancelled")); };
    const cleanup = (): void => {
      if (connectTimer !== undefined) clearTimeout(connectTimer);
      signal.removeEventListener("abort", onAbort);
    };
    signal.addEventListener("abort", onAbort, { once: true });
    if (signal.aborted) {
      onAbort();
      return;
    }
    request.once("socket", socket => {
      const tlsSocket = socket as TLSSocket;
      connectTimer = setTimeout(() => request.destroy(new Error("connect timeout")), input.connectTimeoutMs);
      connectTimer.unref?.();
      tlsSocket.once("secureConnect", () => {
        if (connectTimer !== undefined) clearTimeout(connectTimer);
      });
    });
    request.once("error", fail);
    if (input.request.body === null) {
      request.end();
      return;
    }
    const body = Readable.fromWeb(input.request.body as never);
    body.once("error", () => request.destroy(new Error("request body failed")));
    body.pipe(request);
  });
}

function validateInput(input: {
  readonly request: Request;
  readonly addresses: readonly ExternalMcpResolvedAddress[];
  readonly tlsServerName: string;
  readonly caBundleRef: string;
}): void {
  const url = new URL(input.request.url);
  if (url.protocol !== "https:" || url.hostname !== input.tlsServerName || !serverNamePattern.test(input.tlsServerName) ||
      !caBundleRefPattern.test(input.caBundleRef) || input.addresses.length === 0 || input.addresses.length > 32) {
    throw new Error("External MCP TLS dispatch binding is invalid");
  }
  const seen = new Set<string>();
  for (const address of input.addresses) {
    if (isIP(address.address) !== address.family || seen.has(`${address.family}:${canonicalAddress(address.address)}`)) {
      throw new Error("External MCP TLS approved address evidence is invalid");
    }
    seen.add(`${address.family}:${canonicalAddress(address.address)}`);
  }
}

function requestHeaders(request: Request, url: URL): Record<string, string> {
  const headers: Record<string, string> = {};
  for (const [name, value] of request.headers) {
    if (deniedHeaders.has(name.toLowerCase())) throw new Error("External MCP TLS request contains a denied header");
    headers[name] = value;
  }
  headers.host = url.host;
  return headers;
}

function pinnedLookup(addresses: readonly ExternalMcpResolvedAddress[]): LookupFunction {
  const answers = addresses.map(address => ({ address: address.address, family: address.family }));
  return (_hostname, options, callback) => {
    if (options.all === true) {
      callback(null, answers);
      return;
    }
    const first = answers[0]!;
    callback(null, first.address, first.family);
  };
}

function approvedPeer(peer: string, addresses: readonly ExternalMcpResolvedAddress[]): string | undefined {
  const canonicalPeer = canonicalAddress(peer);
  return addresses.find(address => canonicalAddress(address.address) === canonicalPeer)?.address;
}

function canonicalAddress(address: string): string {
  if (isIP(address) === 4) return address;
  const mapped = /^::ffff:(\d{1,3}(?:\.\d{1,3}){3})$/i.exec(address)?.[1];
  if (mapped !== undefined && isIP(mapped) === 4) return mapped;
  if (isIP(address) === 6) return new URL(`https://[${address}]/`).hostname.slice(1, -1);
  return address;
}

async function cancelBody(response: Response): Promise<void> {
  try {
    await response.body?.cancel();
  } catch {
    // The response is permanently rejected after peer verification fails.
  }
}
