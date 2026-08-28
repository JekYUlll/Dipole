import { describe, expect, it, vi } from "vitest";

import {
  NodeExternalMcpDnsResolver,
  type NodeExternalMcpDnsClient
} from "./node-external-mcp-dns-resolver.js";

describe("Node external MCP DNS Resolver", () => {
  it("returns every A and AAAA answer from a fresh Resolver per request", async () => {
    const clients = [
      client([answer("8.8.8.8", 30)], [answer("2606:4700:4700::1111", 60)]),
      client([answer("1.1.1.1", 20)], [])
    ];
    const createClient = vi.fn(() => clients.shift()!);
    const resolver = new NodeExternalMcpDnsResolver(createClient);

    await expect(resolver.resolve("mcp.example.com", new AbortController().signal)).resolves.toEqual([
      { address: "8.8.8.8", family: 4 },
      { address: "2606:4700:4700::1111", family: 6 }
    ]);
    await expect(resolver.resolve("mcp.example.com", new AbortController().signal)).resolves.toEqual([
      { address: "1.1.1.1", family: 4 }
    ]);
    expect(createClient).toHaveBeenCalledTimes(2);
  });

  it("allows one address family to have no records", async () => {
    const missing = Object.assign(new Error("no AAAA record"), { code: "ENODATA" });
    const resolver = new NodeExternalMcpDnsResolver(() => client(
      [answer("8.8.4.4", 10)], Promise.reject(missing)
    ));

    await expect(resolver.resolve("mcp.example.com", new AbortController().signal)).resolves.toEqual([
      { address: "8.8.4.4", family: 4 }
    ]);
  });

  it("fails the complete lookup on transient or malformed family evidence without leaking details", async () => {
    const upstream = Object.assign(new Error("resolver secret for mcp.example.com"), { code: "ESERVFAIL" });
    const failed = new NodeExternalMcpDnsResolver(() => client(
      Promise.reject(upstream), [answer("2606:4700:4700::1111", 10)]
    ));
    const error = await failed.resolve("mcp.example.com", new AbortController().signal).then(
      () => { throw new Error("expected DNS failure"); },
      value => value as Error
    );
    expect(error.message).toBe("External MCP DNS lookup failed");
    expect(error.message).not.toMatch(/example|secret|ESERVFAIL/i);

    const malformed = new NodeExternalMcpDnsResolver(() => client([answer("::1", 10)], []));
    await expect(malformed.resolve("mcp.example.com", new AbortController().signal))
      .rejects.toThrow(/invalid evidence/i);
  });

  it("cancels only the request-local Resolver and preserves the caller reason", async () => {
    const controller = new AbortController();
    let reject4: (reason: unknown) => void = () => undefined;
    let reject6: (reason: unknown) => void = () => undefined;
    const resolve4 = new Promise<never>((_resolve, reject) => { reject4 = reject; });
    const resolve6 = new Promise<never>((_resolve, reject) => { reject6 = reject; });
    const dnsClient = client(resolve4, resolve6);
    dnsClient.cancel = vi.fn<() => void>(() => {
      reject4(Object.assign(new Error("cancelled"), { code: "ECANCELLED" }));
      reject6(Object.assign(new Error("cancelled"), { code: "ECANCELLED" }));
    });
    const resolver = new NodeExternalMcpDnsResolver(() => dnsClient);
    const pending = resolver.resolve("mcp.example.com", controller.signal);
    const reason = new Error("request cancelled by Activity");

    controller.abort(reason);

    await expect(pending).rejects.toBe(reason);
    expect(dnsClient.cancel).toHaveBeenCalledOnce();
  });

  it("rejects invalid hostnames and pre-cancellation before creating a Resolver", async () => {
    const createClient = vi.fn(() => client([], []));
    const resolver = new NodeExternalMcpDnsResolver(createClient);
    await expect(resolver.resolve("127.0.0.1", new AbortController().signal)).rejects.toThrow(/hostname/i);
    await expect(resolver.resolve("MCP.example.com", new AbortController().signal)).rejects.toThrow(/hostname/i);

    const controller = new AbortController();
    controller.abort(new Error("cancelled before DNS"));
    await expect(resolver.resolve("mcp.example.com", controller.signal)).rejects.toThrow(/cancelled before DNS/i);
    expect(createClient).not.toHaveBeenCalled();
  });
});

function answer(address: string, ttl: number) {
  return { address, ttl };
}

function client(
  ipv4: readonly { readonly address: string; readonly ttl: number }[] | Promise<readonly { readonly address: string; readonly ttl: number }[]>,
  ipv6: readonly { readonly address: string; readonly ttl: number }[] | Promise<readonly { readonly address: string; readonly ttl: number }[]>
): NodeExternalMcpDnsClient {
  return {
    resolve4: vi.fn(async () => ipv4),
    resolve6: vi.fn(async () => ipv6),
    cancel: vi.fn<() => void>()
  };
}
