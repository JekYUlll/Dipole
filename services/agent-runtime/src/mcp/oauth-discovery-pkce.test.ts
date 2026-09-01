import { describe, expect, it } from "vitest";

import {
  authorizationServerMetadataURL,
  createPkceAuthorizationRequest,
  discoverAuthorizationServerMetadata,
  parseAuthorizationServerMetadata
} from "./oauth-discovery-pkce.js";

const issuer = "https://auth.example.com/tenant-a";

describe("OAuth discovery and PKCE foundation", () => {
  it("derives RFC 8414 metadata before an issuer path and accepts exact S256 metadata", () => {
    expect(authorizationServerMetadataURL(issuer)).toBe(
      "https://auth.example.com/.well-known/oauth-authorization-server/tenant-a"
    );

    expect(parseAuthorizationServerMetadata(issuer, {
      issuer,
      authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["plain", "S256"]
    })).toEqual({
      issuer,
      authorizationEndpoint: "https://auth.example.com/tenant-a/authorize",
      tokenEndpoint: "https://auth.example.com/tenant-a/token"
    });
  });

  it("fails closed for identifier drift, insecure endpoints, or missing S256", () => {
    expect(() => parseAuthorizationServerMetadata(issuer, {
      issuer: "https://auth.example.com/other",
      authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["S256"]
    })).toThrow(/issuer/i);
    expect(() => parseAuthorizationServerMetadata(issuer, {
      issuer,
      authorization_endpoint: "http://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["S256"]
    })).toThrow(/HTTPS/i);
    expect(() => parseAuthorizationServerMetadata(issuer, {
      issuer,
      authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
      token_endpoint: "https://auth.example.com/tenant-a/token",
      code_challenge_methods_supported: ["plain"]
    })).toThrow(/S256/i);
  });

  it("creates a URL-safe S256 PKCE authorization request without persisting token material", () => {
    const request = createPkceAuthorizationRequest({
      metadata: parseAuthorizationServerMetadata(issuer, {
        issuer,
        authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
        token_endpoint: "https://auth.example.com/tenant-a/token",
        code_challenge_methods_supported: ["S256"]
      }),
      clientId: "dipole-agent",
      redirectUri: "https://agent.example.com/oauth/callback",
      scope: "mcp.read",
      randomBytes: size => Buffer.alloc(size, 7)
    });

    expect(request.codeVerifier).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(request.state).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(request.url.toString()).toContain("code_challenge_method=S256");
    expect(request.url.searchParams.get("code_challenge")).toHaveLength(43);
    expect(request.url.searchParams.get("state")).toBe(request.state);
    expect(request.url.searchParams.has("code_verifier")).toBe(false);
  });

  it("rejects unsafe issuer and callback URLs before constructing a request", () => {
    expect(() => authorizationServerMetadataURL("https://auth.example.com/tenant?test=1")).toThrow(/canonical/i);
    expect(() => createPkceAuthorizationRequest({
      metadata: parseAuthorizationServerMetadata(issuer, {
        issuer,
        authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
        token_endpoint: "https://auth.example.com/tenant-a/token",
        code_challenge_methods_supported: ["S256"]
      }),
      clientId: "dipole-agent",
      redirectUri: "http://agent.example.com/oauth/callback",
      scope: "mcp.read"
    })).toThrow(/redirect/i);
  });

  it("discovers exact JSON metadata through an injected non-redirecting fetch", async () => {
    const fetcher = async (url: string | URL | Request, init?: RequestInit) => {
      expect(String(url)).toBe("https://auth.example.com/.well-known/oauth-authorization-server/tenant-a");
      expect(init?.redirect).toBe("manual");
      expect(init?.headers).toEqual({ accept: "application/json" });
      return new Response(JSON.stringify({
        issuer,
        authorization_endpoint: "https://auth.example.com/tenant-a/authorize",
        token_endpoint: "https://auth.example.com/tenant-a/token",
        code_challenge_methods_supported: ["S256"]
      }), { headers: { "content-type": "application/json; charset=utf-8" } });
    };

    await expect(discoverAuthorizationServerMetadata(issuer, { fetcher })).resolves.toMatchObject({
      issuer, authorizationEndpoint: "https://auth.example.com/tenant-a/authorize"
    });
  });

  it("fails closed for redirect, non-JSON, oversized, and failed discovery responses", async () => {
    const cases: Array<[string, typeof fetch, RegExp]> = [
      ["redirect", async () => new Response(null, { status: 302, headers: { location: "https://other.example" } }), /redirect/i],
      ["content type", async () => new Response("{}", { headers: { "content-type": "text/html" } }), /JSON/i],
      ["size", async () => new Response("{}", { headers: { "content-type": "application/json", "content-length": "65537" } }), /large/i],
      ["status", async () => new Response("{}", { status: 503, headers: { "content-type": "application/json" } }), /failed/i],
      ["network", async () => { throw new Error("offline"); }, /failed/i]
    ];
    for (const [, fetcher, expected] of cases) {
      await expect(discoverAuthorizationServerMetadata(issuer, { fetcher })).rejects.toThrow(expected);
    }
  });

  it("rejects unsafe discovery limits before calling fetch", async () => {
    const fetcher = async () => new Response("{}", { headers: { "content-type": "application/json" } });
    await expect(discoverAuthorizationServerMetadata(issuer, { fetcher, timeoutMs: 99 })).rejects.toThrow(/timeout/i);
    await expect(discoverAuthorizationServerMetadata(issuer, { fetcher, maximumBytes: 1023 })).rejects.toThrow(/bytes/i);
  });
});
